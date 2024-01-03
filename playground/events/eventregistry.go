package events

import (
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

// `getID` its an internal function that will to have IDs locally but I highly encourage you to come with your own IDs
func getID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	// The following code will make it look like a UUID, but it's not a fully compliant implementation.
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant is 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

type FnEventID func() (string, error)

// `EventRegistry` store events per kind based on the `KindRegistry`.
// Each kind have a `CircularBuffer` which increase in capacity when we can't process those event fast enough.
type EventRegistry struct {
	*ThreadSafeMap[EventKindID, *CircularBuffer[Event[interface{}]]]
	surrogateID FnEventID
}

type OptionEventRegistry func(s *EventRegistry) error

func OptionEventRegistryGetID(fn FnEventID) OptionEventRegistry {
	return func(s *EventRegistry) error {
		s.surrogateID = fn
		return nil
	}
}

func newEventRegistry(opts ...OptionEventRegistry) (*EventRegistry, error) {
	registry := EventRegistry{
		ThreadSafeMap: NewThreadSafeMap[EventKindID, *CircularBuffer[Event[interface{}]]](),
	}
	for i := 0; i < len(opts); i++ {
		if err := opts[i](&registry); err != nil {
			return nil, err
		}
	}
	return &registry, nil
}

func (er *EventRegistry) checkEvent(e *Event[interface{}]) error {
	if e.Kind == InfoNone {
		return errors.Join(ErrEventGotNoKind, fmt.Errorf("can't enqueue event"))
	}
	if e.ID == NoID {
		return errors.Join(ErrEventGotNoID, fmt.Errorf("can't enqueue event"))
	}
	return nil
}

func (er *EventRegistry) Enqueue(e *Event[interface{}]) error {
	if err := er.checkEvent(e); err != nil {
		return err
	}
	var buffer *CircularBuffer[Event[interface{}]]
	var ok bool
	if !er.Has(e.Kind) {
		er.Set(e.Kind, NewCircularBuffer[Event[interface{}]](10)) // TODO @droman: would be have default sizes?
	}
	if buffer, ok = er.Get(e.Kind); !ok {
		return errors.Join(ErrBufferNotFound, fmt.Errorf("couldn't find buffer for %v", e.Kind))
	}
	if err := buffer.Enqueue(*e); err != nil {
		return errors.Join(ErrEventEnqueued, fmt.Errorf("couldn't enqueue %v", e.Kind))
	}
	return nil
}

func (er *EventRegistry) EnqueueN(es ...*Event[interface{}]) error {
	kinds := []EventKindID{}
	for i := 0; i < len(es); i++ {
		var err error
		if eventerr := er.checkEvent(es[i]); eventerr != nil {
			if err == nil {
				eventerr = err
			} else {
				err = errors.Join(eventerr)
			}
		}
		if err != nil {
			return err
		}
		kinds = append(kinds, es[i].Kind)
	}
	// come back
	return nil
}

func (er *EventRegistry) Register(e *Event[interface{}]) (*Event[interface{}], error) {
	var id string
	var err error
	// getting id from user or default
	if id, err = er.surrogateID(); err != nil {
		return e, err
	}
	e.ID = EventID(id)
	return e, nil
}

// `newUnregisteredEvent` create event from anonymous data which mean the event doesn't have an ID yet
func newUnregisteredEvent(e EventAnonymous) *Event[interface{}] {
	var kind Kind
	name := getNameKind(e)
	if hasKind(name) {
		kind = getKind(name)
	} else {
		kind = newKind(e)
	}
	// event won't have an id
	event := Event[interface{}]{
		Kind:      kind.ID, // we need to know the name
		Data:      e,       // we need to have the data the user is giving
		CreatedAt: EventCreated(time.Now()),
	}
	return &event
}

// `newUnregisteredKindEvent` create event from a known kind which mean the event doesn't have an ID yet
func newUnregisteredKindEvent(kind Kind) *Event[interface{}] {
	// event won't have an id
	event := Event[interface{}]{
		Kind:      kind.ID, // we need to know the name
		CreatedAt: EventCreated(time.Now()),
		Data:      kind.TypeInfo,
	}
	return &event
}
