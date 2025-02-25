package events

import "sync"

type Manager struct {
	lock        sync.RWMutex
	events      []Event
	subscribers []chan Event
}

func NewManager() *Manager {
	return &Manager{
		lock:        sync.RWMutex{},
		events:      make([]Event, 0),
		subscribers: make([]chan Event, 0),
	}
}

func (m *Manager) SendEvent(event Event) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.events = append(m.events, event)

	for _, subscriber := range m.subscribers {
		subscriber <- event
	}
}

func (m *Manager) Subscribe() chan Event {
	// This is locked to avoid races with adding new subscribers and new events at the same time,
	// to avoid a new subscriber missing an event
	m.lock.Lock()
	defer m.lock.Unlock()

	subscriberChan := make(chan Event, 1000000)

	m.subscribers = append(m.subscribers, subscriberChan)

	// spool all existing events to the new subscriber
	for _, event := range m.events {
		subscriberChan <- event
	}

	return subscriberChan
}

func (m *Manager) NewLogWriter() LogWriter {
	return LogWriter{
		eventMgr: m,
	}
}
