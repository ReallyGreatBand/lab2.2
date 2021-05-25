package datastore

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type hashIndex map[string]int64

type segment struct {
	filePath string
	file *os.File
	outOffset int64
	index hashIndex
}

func initSegment(path string) (*segment, error){
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	seg := &segment{
		filePath: path,
		file: file,
		outOffset: 0,
		index: make(hashIndex),
	}

	err = seg.recover()
	if err != nil && err != io.EOF {
		return nil, err
	}

	return seg, nil
}


func (seg *segment) close() error {
	
	return nil
}

func (seg *segment) get(key string) (string, error) {
	return "", nil
}

func (seg *segment) put(key, value string) error {
	return nil
}

func (seg *segment) checkHealth() error {
	name := seg.file.Name()
	if !strings.HasPrefix(name, segmentPrefix) {
		return fmt.Errorf("segment %s is corrupted", name)
	}
	return nil
}

func (seg *segment) recover() error {
	return nil
}
