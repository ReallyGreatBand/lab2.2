package datastore

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

type hashIndex map[string]int64

type segment struct {
	filePath string
	file *os.File
	outOffset int64
	index hashIndex
}

const bufSize = 8192

func initSegment(path string) (*segment, error){
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
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
	return seg.file.Close()
}

func (seg *segment) get(key string) (string, error) {
	position, ok := seg.index[key]
	if !ok {
		return "", ErrNotFound
	}

	file, err := os.Open(seg.filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = file.Seek(position, 0)
	if err != nil {
		return "", err
	}

	reader := bufio.NewReader(file)
	value, err := readValue(reader)
	if err != nil {
		return "", nil
	}

	return value, nil
}

func (seg *segment) put(key, value string) error {
	e := entry{
		key:   key,
		value: value,
	}
	n, err := seg.file.Write(e.Encode())
	if err == nil {
		seg.index[key] = seg.outOffset
		seg.outOffset += int64(n)
	}
	return err
}

func (seg *segment) checkHealth() error {
	name := seg.file.Name()
	if !strings.HasPrefix(path.Base(name), segmentPrefix) {
		return fmt.Errorf("segment %s is corrupted", path.Base(name))
	}
	return nil
}

func (seg *segment) recover() error {
	input, err := os.Open(seg.filePath)
	if err != nil {
		return err
	}
	defer input.Close()

	var buf [bufSize]byte
	in := bufio.NewReaderSize(input, bufSize)
	for err == nil {
		var (
			header, data []byte
			n int
		)
		header, err = in.Peek(bufSize)
		if err == io.EOF {
			if len(header) == 0 {
				return err
			}
		} else if err != nil {
			return err
		}
		size := binary.LittleEndian.Uint32(header)

		if size < bufSize {
			data = buf[:size]
		} else {
			data = make([]byte, size)
		}
		n, err = in.Read(data)

		if err == nil {
			if n != int(size) {
				return fmt.Errorf("corrupted file")
			}

			var e entry
			e.Decode(data)
			seg.index[e.key] = seg.outOffset
			seg.outOffset += int64(n)
		}
	}
	return err
}
