package datastore

import (
	"fmt"
	"log"
	"sync"
)
// atomic counter for checking number of active workers
type safeCounter struct {
	mux *sync.Mutex
	counter int
}
//atomic add
func (sc *safeCounter) add(maxWorkers int) {
	sc.mux.Lock()
	sc.counter += 1
	if sc.counter > maxWorkers {
		log.Printf("%v", sc.counter)
	}
	sc.mux.Unlock()
}

//atomic minus
func (sc *safeCounter) minus() {
	sc.mux.Lock()
	sc.counter -= 1
	sc.mux.Unlock()
}

func (db *Db) Get(key string) (string, error) {
	db.getChan <- 1
	db.getCounter.add(len(db.getChan))
	db.mux.RLock()
	defer func() {
		db.mux.RUnlock()
		db.getCounter.minus()
		<- db.getChan
	}()

	for _, seg := range db.segments {
		value, err := seg.get(key)
		if err == nil {
			return value, err
		}
	}

	return "", fmt.Errorf("did't find any value at the key: %s", key)
}


