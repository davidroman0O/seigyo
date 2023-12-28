package events

import "testing"

func TestSimpleEventRegistry(t *testing.T) {
	var registry *EventRegistry

	var err error
	if registry, err = newEventRegistry(); err != nil {
		t.Error(err)
	}

	if err := registry.Enqueue(&EventNone); err == nil {
		t.Error("shoul have an issue storing a none event")
	}
}
