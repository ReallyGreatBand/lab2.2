package datastore

import (
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

	db, err := NewDb(dir, testSize)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

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
		db, err = NewDb(dir, testSize)
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
			t.Fatal(err)
		}
		db, err = NewDb(dir, 32)
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

		if _, err = os.Open(filepath.Join(dir, segmentPrefix + "-merged")); err != nil {
			t.Errorf("Cannot read segment file: %s", err)
		}
		if _, err = os.Open(filepath.Join(dir, segmentPrefix + "1")); err == nil {
			t.Errorf("Segment was not merged!: %s", err)
		}
		if _, err = os.Open(filepath.Join(dir, segmentPrefix + "2")); err != nil {
			t.Errorf("Cannot read segment file: %s", err)
		}
	})
}
