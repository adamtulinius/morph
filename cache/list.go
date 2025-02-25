package cache

import "sync"

type LockedList[T any] struct {
	mutex sync.RWMutex
	data  []T
}

func NewLockedList[T any]() LockedList[T] {
	return LockedList[T]{
		mutex: sync.RWMutex{},
		data:  make([]T, 0),
	}
}

func (list *LockedList[T]) Get() []T {
	// FIXME: Ensure operations on this doesn't mutate the list
	return list.data
}

func (list *LockedList[T]) Append(elem T) {
	list.mutex.Lock()
	defer list.mutex.Unlock()

	list.data = append(list.data, elem)
}
