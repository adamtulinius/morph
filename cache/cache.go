package cache

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"sync"
)

type StepData struct {
	Key   string
	Value string
}

type LockedMap[T any] struct {
	identifier string
	mutex      sync.RWMutex
	data       map[string]T
}

func NewLockedMap[T any](identifier string) *LockedMap[T] {
	return &LockedMap[T]{
		identifier: identifier,
		mutex:      sync.RWMutex{},
		data:       make(map[string]T),
	}
}

func (m *LockedMap[T]) GetOrSet(key string, value T) T {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if data, ok := m.data[key]; ok {
		log.Info().
			Str("event", "store-get-or-set").
			Str("store", m.identifier).
			Str("key", key).
			Any("value", value).
			Msg(fmt.Sprintf("get-or-set on '%s': key '%s' exists", m.identifier, key))

		return data
	} else {
		log.Info().
			Str("event", "store-get-or-set").
			Str("store", m.identifier).
			Str("key", key).
			Any("value", value).
			Msg(fmt.Sprintf("get-or-set on '%s': key '%s' missing -> '%s' = '%v'", m.identifier, key, key, value))

		m.data[key] = value

		return value
	}
}

func (m *LockedMap[T]) Update(key string, value T) {
	log.Info().
		Str("event", "store-write").
		Str("store", m.identifier).
		Str("key", key).
		Any("value", value).
		Msg(fmt.Sprintf("write on '%s': '%s' = '%v'", m.identifier, key, value))

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.data[key] = value
}

func (m *LockedMap[T]) Get(key string) (T, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	data, ok := m.data[key]
	if ok {
		return data, nil
	} else {
		return data, errors.New(fmt.Sprintf("m: miss for '%s'", key))
	}
}

func (m *LockedMap[T]) GetCopy() map[string]T {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	dataCopy := make(map[string]T)

	for key, value := range m.data {
		dataCopy[key] = value
	}

	return dataCopy
}

// run provided fn on the value of a key
func (m *LockedMap[T]) Run(key string, defaultValue T, fn func(T) T) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, ok := m.data[key]; !ok {
		m.data[key] = defaultValue
	}

	m.data[key] = fn(m.data[key])
}
