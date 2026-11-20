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
	if maxSize <= 0 {
		panic("the fuck you doing?")
	}
	return Queue[T]{
		Contents: make([]T, 0, maxSize),
		MaxSize:  maxSize,
		mutex:    new(sync.RWMutex),
	}
}

func (q *Queue[T]) Push(value T) bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	added := true
	if len(q.Contents) >= q.MaxSize {
		copy(q.Contents, q.Contents[1:])
		var zero T
		q.Contents[len(q.Contents)-1] = zero
		q.Contents = q.Contents[:len(q.Contents)-1]
		added = false
	}
	q.Contents = append(q.Contents, value)
	return added
}

func (q *Queue[T]) Snapshot() []T {
	q.mutex.RLock()
	out := append(make([]T, 0, q.MaxSize), q.Contents...)
	q.mutex.RUnlock()
	return out

}
func (q *Queue[T]) DeleteAt(index int) {
	q.mutex.Lock()
	q.Contents = slices.Delete(q.Contents, index, index+1)
	q.mutex.Unlock()
}

func (q *Queue[T]) DeleteFunc(f func(v T) bool) {
	q.mutex.Lock()
	q.Contents = slices.DeleteFunc(q.Contents, f)

	q.mutex.Unlock()
}
