package events

import "reflect"

// `KindNone` represent a kind that is none
var KindNone Kind = Kind{
	Kind: reflect.TypeOf(struct{}{}).Kind(),
	ID:   InfoNone,
}

// `EventNone` represent an event that is none
var EventNone Event[interface{}] = Event[interface{}]{
	ID:   NoID,
	Kind: InfoNone,
	Data: nil,
}

// `Kind` represent the signature of a given type by the user to track
// in which registry it belong to.
type Kind struct {
	ID       EventKindID
	Kind     reflect.Kind
	IsNone   bool
	TypeInfo interface{}
}

// `Event` represent the main data managed by a mediator and event registries
// which can hold a data that need to reach a `Subscriber`. Every `Event` will be
// dispatched through different means.
type Event[T any] struct {
	ID        EventID
	CreatedAt EventCreated
	Kind      EventKindID
	Data      T
}
