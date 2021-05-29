package datastore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testSize int64 = 256

func TestDb_Put(t *testing.T) {
	dir, err := ioutil.TempDir("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := NewDb(dir, testSize, 1)
	if err != nil {
		t.Fatal(err)
	}

	pairs := [][]string {
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}

	outFile, err := os.Open(filepath.Join(dir, segmentPrefix + "0"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("put/get", func(t *testing.T) {
		for _, pair := range pairs {
			err := db.Put(pair[0], pair[1])
			if err != nil {
				t.Errorf("Cannot put %s: %s", pairs[0], err)
			}
			value, err := db.Get(pair[0])
			if err != nil {
				t.Errorf("Cannot get %s: %s", pairs[0], err)
			}
			if value != pair[1] {
				t.Errorf("Bad value returned expected %s, got %s", pair[1], value)
			}
		}
	})

	outInfo, err := outFile.Stat()
	if err != nil {
		t.Fatal(err)
	}
	size1 := outInfo.Size()

	t.Run("file growth", func(t *testing.T) {
		for _, pair := range pairs {
			err := db.Put(pair[0], pair[1])
			if err != nil {
				t.Errorf("Cannot put %s: %s", pairs[0], err)
			}
		}
		outInfo, err := outFile.Stat()
		if err != nil {
			t.Fatal(err)
		}
		if size1 * 2 != outInfo.Size() {
			t.Errorf("Unexpected size (%d vs %d)", size1, outInfo.Size())
		}
	})

	t.Run("db segmentation", func(t *testing.T) {
		key := "long"
		val := strings.Repeat("value", 30)

		err = db.Put(key, val)
		if err != nil {
			t.Errorf("Cannot put key: %s", err)
		}
		_, err = os.Open(filepath.Join(dir, segmentPrefix + "1"))
		if err != nil {
			t.Errorf("Cannot read new segment: %s", err)
		}

		value, err := db.Get(key)
		if err != nil {
			t.Errorf("Cannot read value: %s", err)
		}
		if value != val {
			t.Errorf("Bad value returned expected %s, got %s", val, value)
		}
	})

	t.Run("new db process", func(t *testing.T) {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		db, err = NewDb(dir, testSize, 1)
		if err != nil {
			t.Fatal(err)
		}

		for _, pair := range pairs {
			value, err := db.Get(pair[0])
			if err != nil {
				t.Errorf("Cannot put %s: %s", pairs[0], err)
			}
			if value != pair[1] {
				t.Errorf("Bad value returned expected %s, got %s", pair[1], value)
			}
		}
	})

	t.Run("merge", func(t *testing.T) {
		if err := db.Close(); err != nil {
			fmt.Printf("Closing %v", err)
			t.Fatal(err)
		}
		db, err = NewDb(dir, 32, 1)
		if err != nil {
			t.Fatal(err)
		}

		for _, pair := range pairs {
			err := db.Put(pair[0], pair[1])
			if err != nil {
				t.Errorf("Cannot put %s: %s", pairs[0], err)
			}
			value, err := db.Get(pair[0])
			if err != nil {
				t.Errorf("Cannot get %s: %s", pairs[0], err)
			}
			if value != pair[1] {
				t.Errorf("Bad value returned expected %s, got %s", pair[1], value)
			}
		}

		if _, err = os.Open(filepath.Join(dir, segmentPrefix + "0")); err != nil {
			t.Errorf("Cannot read segment file: %s", err)
		}
		if _, err = os.Open(filepath.Join(dir, segmentPrefix + "1")); err == nil {
			t.Errorf("Segment was not merged!: %s", err)
		}
		if _, err = os.Open(filepath.Join(dir, segmentPrefix + "2")); err != nil {
			t.Errorf("Cannot read segment file: %s", err)
		}
	})

	// may be used for checking 3rd task thanks to counter that atomically checks number of active workers

	t.Run("parallel", func(t *testing.T) {
		parallelDir, err := ioutil.TempDir("", "test-parallel-db")
		if err != nil {
			t.Fatal(err)
		}
		err = db.Close()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(parallelDir)

		db, err := NewDb(parallelDir, 64, 2)
		if err != nil {
			t.Fatalf("Unsuccesful database creation: %s", err)
		}

		pairs := map[string]string {
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
			"key5": "value5",
			"key6": "value6",
			"key7": "value7",
			"key8": "value8",
			"key9": "value9",
		}

		ch := make(chan interface{})

		for k, v := range pairs {
			k := k
			v := v
			go func() {
				for i := 1; i <= 100; i++ {
					err := db.Put(k, v)
					if err != nil {
						t.Errorf("Cannot put %s: %s", k, err)
					}
					value, err := db.Get(k)
					if err != nil {
						t.Errorf("Cannot get %s: %s", k, err)
					}
					if value != v {
						t.Errorf("Bad value returned expected %v, got %s", v, value)
					}
				}
				ch <- 1
			}()
		}

		for range pairs {
			<-ch
		}

		for k, v := range pairs {
			value, err := db.Get(k)
			if err != nil {
				t.Errorf("Cannot get %s: %s", k, err)
			}
			if value != v {
				t.Errorf("Bad value returned expected %v, got %s", v, value)
			}
		}

		err = db.Close()
		if err != nil {
			t.Fatal(err)
		}
	})
}
