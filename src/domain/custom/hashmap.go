package custom

import (
	"hash/fnv"
	"slices"
	"sync"
)

// i should make this

type Entry[T any] struct {
	Key     uint64
	UserKey string
	Value   T
}

type Dictionary[T any] struct {
	mutex   *sync.RWMutex
	Buckets [][]Entry[T]
	Size    int
}

func NewDictionary[T any](buckets int) *Dictionary[T] {
	// we have decided that we are going to have a 6 bucket size dictionary
	return &Dictionary[T]{
		Buckets: make([][]Entry[T], buckets),
		Size:    0,
	}
}
func (d *Dictionary[T]) Get(userKey string) (T, bool) {
	hashKey := HashKey(userKey)

	d.mutex.RLock()
	defer d.mutex.RUnlock()

	idx := hashKey % uint64(len(d.Buckets))
	bucket := d.Buckets[idx]
	for _, entry := range bucket {
		if entry.UserKey != userKey {
			continue
		}
		return entry.Value, true
	}
	var out T
	return out, false
}

func (d *Dictionary[T]) Put(userKey string, value T) {
	hashKey := HashKey(userKey)

	d.mutex.Lock()
	defer d.mutex.Lock()

	idx := hashKey % uint64(len(d.Buckets))

	bucket := d.Buckets[idx]
	for i, entry := range bucket {
		if entry.UserKey != userKey {
			continue
		}
		bucket[i].Value = value
		return
	}

	d.Buckets[idx] = append(bucket, Entry[T]{
		UserKey: userKey,
		Key:     hashKey,
		Value:   value,
	})
	d.Size++
}

func (d *Dictionary[T]) Delete(userKey string) {
	hashKey := HashKey(userKey)

	d.mutex.Lock()
	defer d.mutex.Unlock()

	idx := hashKey % uint64(len(d.Buckets))

	d.Buckets[idx] = slices.DeleteFunc(d.Buckets[idx], func(v Entry[T]) bool {
		if v.UserKey != userKey {
			return false
		}
		d.Size--
		return true
	})

}
func HashKey(userKey string) uint64 {
	h := fnv.New64()
	h.Write([]byte(userKey))
	return h.Sum64()
}
