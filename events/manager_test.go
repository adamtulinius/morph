package events

import "testing"

func TestSubscribeBeforeAddingEvent(t *testing.T) {
	manager := NewManager()

	data := "hello world"

	subscriber := manager.Subscribe()
	manager.SendEvent(Log{Data: data})

	event1 := <-subscriber

	if event1.(Log).Data != data {
		t.Fatalf("Expected returned event data = %s, got %s", data, event1.(Log).Data)
	}
}

func TestSubscribeAfterAddingEvent(t *testing.T) {
	manager := NewManager()

	data := "hello world"

	manager.SendEvent(Log{Data: data})
	subscriber := manager.Subscribe()

	event1 := <-subscriber

	if event1.(Log).Data != data {
		t.Fatalf("Expected returned event data = %s, got %s", data, event1.(Log).Data)
	}
}
