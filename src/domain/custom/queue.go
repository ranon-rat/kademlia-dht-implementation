package custom

import (
	"slices"
	"sync"
)

// TODO: Make this thread safe

type Queue[T any] struct {
	Contents []T
	MaxSize  int
	mutex    *sync.RWMutex
}

func NewQueue[T any](maxSize int) Queue[T] {
	return Queue[T]{
		Contents: make([]T, 0, maxSize),
		MaxSize:  maxSize,
	}
}

func (q *Queue[T]) Push(value T) bool {

	newAdded := true
	q.mutex.Lock()
	if len(q.Contents) >= q.MaxSize {
		q.Contents = q.Contents[min(1, len(q.Contents)):]
		newAdded = false
	}
	q.Contents = append(q.Contents, value)
	q.mutex.Unlock()
	return newAdded
}

func (q Queue[T]) Snapshot() []T {
	q.mutex.RLock()
	out := append(make([]T, 0, q.MaxSize), q.Contents...)
	q.mutex.RUnlock()
	return out

}
func (q *Queue[T]) DeleteAt(index int) {
	q.mutex.Lock()
	q.Contents = slices.Delete(q.Contents, index, index)
	q.mutex.Unlock()
}

func (q *Queue[T]) DeleteFunc(f func(v T) bool) {
	q.mutex.Lock()
	q.Contents = slices.DeleteFunc(q.Contents, f)

	q.mutex.Unlock()
}
