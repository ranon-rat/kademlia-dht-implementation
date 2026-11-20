package safe

import (
	"maps"
	"sync"
)

// this exists for constant read and writting, I would be using the sync.Map
// but it had a few limitations that made me consider this as a better option
//
// here i have to be sure that this is sync secure so for that i have to do the next
type Map[K comparable, V any] struct {
	mu  sync.RWMutex
	Map map[K]V
}

func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		Map: make(map[K]V),
	}
}

func (sm *Map[K, V]) Set(key K, value V) {
	sm.mu.Lock()
	sm.Map[key] = value
	sm.mu.Unlock()
}
func (sm *Map[K, V]) Delete(key K) {
	sm.mu.Lock()
	delete(sm.Map, key)
	sm.mu.Unlock()
}

func (sm *Map[K, V]) Get(key K) (V, bool) {
	sm.mu.RLock()
	value, exists := sm.Map[key]
	sm.mu.RUnlock()
	return value, exists
}

func (sm *Map[K, V]) Len() int {
	sm.mu.RLock()
	size := len(sm.Map)
	sm.mu.RUnlock()
	return size
}

func (sm *Map[K, V]) Snapshot() map[K]V {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make(map[K]V, len(sm.Map))
	maps.Copy(result, sm.Map)
	return result
}

// if the function passed to the range returns true the range will stop.
func (sm *Map[K, V]) Range(function func(key K, value V) bool) {
	smap := sm.Snapshot()

	for key, value := range smap {
		if function(key, value) {
			break
		}

	}

}

func (sm *Map[K, V]) Keys() []K {
	smap := sm.Snapshot()
	keys := make([]K, 0, len(smap))
	for key := range smap {
		keys = append(keys, key)
	}
	return keys
}

func (sm *Map[K, V]) Values() []V {
	smap := sm.Snapshot()
	values := make([]V, 0, len(smap))
	for _, value := range smap {
		values = append(values, value)
	}
	return values
}
