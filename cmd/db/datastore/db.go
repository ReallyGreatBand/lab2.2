package datastore

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const DefaultSegment = 10485760
const segmentPrefix = "segment"
var ErrNotFound = fmt.Errorf("record does not exist")


type writeRecord struct {
	key string
	value string
	result chan error
	close bool
}

type Db struct {
	mux *sync.RWMutex
	dirPath string
	segments []*segment
	segSize int64
	writeQueue chan writeRecord
	getChan chan int
	getCounter safeCounter
	isClosed bool
}

func NewDb(dir string, segmentSize int64, numWorkers int) (*Db, error) {

	db := &Db{
		mux: &sync.RWMutex{},
		dirPath: dir,
		segments: nil,
		segSize: segmentSize,
		writeQueue: make(chan writeRecord),
		getChan: make(chan  int, numWorkers),
		getCounter: safeCounter{
			mux: &sync.Mutex{},
			counter: 0,
		},
	}
	err := db.recover()
	go db.writeWorker()

	if err != nil && err != io.EOF {
		return nil, err
	}
	return db, nil
}

func (db *Db) writeWorker() {
	for record := range db.writeQueue {
		if record.close {
			return
		}
		db.mux.Lock()
		err := db.tail().put(record.key, record.value)
		if err != nil {
			db.mux.Unlock()
			record.result <- err
			continue
		}

		if db.tail().outOffset >= db.segSize {
			err := db.createSegment()
			if err != nil {
				db.mux.Unlock()
				record.result <- err
				continue
			}
		}

		db.mux.Unlock()
		record.result <- nil
	}
}



func (db *Db) recover() error {
	contents, err := ioutil.ReadDir(db.dirPath)
	if err != nil {
		return err
	}
	var segments []*segment
	for _, file := range contents {
		if !file.IsDir() && strings.HasPrefix(file.Name(), segmentPrefix) {
			segment, err := initSegment(filepath.Join(db.dirPath, file.Name()))
			if err != nil {
				return err
			}

			segments = append(segments, segment)
		}
	}

	if len(segments) == 0 {
		path := filepath.Join(db.dirPath, segmentPrefix + "0")
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if err != nil {
			return err
		}

		segments = append(segments, &segment{
			filePath: path,
			file: file,
			outOffset: 0,
			index: make(hashIndex),
		})
	}

	db.segments = segments

	return err
}

func (db *Db) Close() error {
	if db.isClosed {
		return fmt.Errorf("database is already closed")
	}
	db.writeQueue <- writeRecord{close: true}
	for _, seg := range db.segments {
		err := seg.close()
		if err != nil {
			return err
		}
	}
	db.isClosed = true
	return nil
}



func (db *Db) Put(key, value string) error {
	rec := writeRecord{
		key: key,
		value: value,
		result: make(chan error),
	}
	db.writeQueue <- rec

	return <- rec.result
}

func (db *Db) tail() *segment {
	return db.segments[len(db.segments) - 1]
}

func (db *Db) createSegment() error {
	tail :=  db.tail()

	err := tail.checkHealth()
	if err != nil {
		return err
	}
	name := tail.file.Name()
	count, err := strconv.Atoi(name[len(name) - 1:])
	path := filepath.Join(db.dirPath, fmt.Sprintf("%s%d", segmentPrefix, count + 1))

	seg, err := initSegment(path)
	if err != nil {
		return err
	}
	db.segments = append(db.segments, seg)

	//mergeChan := merge{close: false}

	if len(db.segments) > 2 {
		err := db.merge()
		if err != nil {
			return err
		}
		/*go func() {
			db.mergeQueue <- mergeChan
		}()*/
	}

	return nil
}

func (db *Db) merge() error {
	previos := db.segments
	mergees := db.segments[0:len(db.segments) - 1] //segment-merged segment3 // segemnt0 segment1 -> segment0 segment2 -> segment0 segment3
	newPath := filepath.Join(db.dirPath, segmentPrefix + "-merged")

	file, err := os.OpenFile(newPath, os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}

	mergedSeg := &segment{
		filePath:  newPath,
		file:      file,
		outOffset: 0,
		index:     make(hashIndex),
	}

	keys := make(map[string]int)

	for i := len(mergees) - 1; i >= 0; i-- {
		mergee := mergees[i]
		for key := range mergee.index {
			if _, exists := keys[key]; exists {
				continue
			}

			value, err := mergee.get(key)
			if err != nil {
				_ = mergedSeg.close()
				_ = os.Remove(newPath)
				return err
			}

			err = mergedSeg.put(key, value)
			if err != nil {
				_ = mergedSeg.close()
				_ = os.Remove(newPath)
				return err
			}
			
			keys[key] = 1
		}
	}


	newSeg := []*segment{mergedSeg}
	db.segments = append(newSeg, db.segments[len(mergees):]...)

	err = os.Rename(newPath, mergees[0].filePath)
	if err != nil {
		db.segments = previos
		os.Remove(newPath)
		return err
	}
	f, err := os.OpenFile(mergees[0].filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		db.segments = previos
		os.Remove(newPath)
		return err
	}
	mergedSeg.file = f
	mergedSeg.filePath = mergees[0].filePath

	for _, segment := range mergees {
		_ = segment.close()
		if segment != mergees[0] {
			_ = os.Remove(segment.filePath)
		}
	}
	return nil
}