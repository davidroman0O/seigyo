package main

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"runtime"
	"time"
)

type Event[EK any] struct {
	Kind EK
	Data interface{}
}

type EventKindName string

type EventKind struct {
	ID   EventID
	Kind reflect.Kind
	Info EventKindName
	None EventNone
}

type EventNone interface{}

type EventSink struct {
	kinds    map[EventKindName]EventKind
	queue    []Event[EventKind]
	sub      chan Event[interface{}]
	closed   bool
	getID    func() (string, error)
	consumer func(subE Event[interface{}])
}

type EventAnonymous interface{}

func getKind(e EventAnonymous) EventKind {
	kind := EventKind{
		Kind: reflect.TypeOf(e).Kind(),
		Info: EventKindName(reflect.ValueOf(e).Type().Name()),
	}
	return kind
}

type eclose struct{}
type EventClose Event[eclose]

type EventID string

var (
	NoID EventID = ""
)

type Option func(s *EventSink) error

func OptionGetID(fn func() (string, error)) Option {
	return func(s *EventSink) error {
		s.getID = fn
		return nil
	}
}

func OptionConsumer(fn func(subE Event[interface{}])) Option {
	return func(s *EventSink) error {
		s.consumer = fn
		return nil
	}
}

func Kind[T any]() EventKind {
	proxy := *new(T)
	kind := EventKind{
		Kind: reflect.TypeOf(proxy).Kind(),
		Info: EventKindName(reflect.ValueOf(proxy).Type().Name()),
		None: proxy,
	}
	return kind
}

func NewEventSink(opts ...Option) (*EventSink, error) {
	sink := EventSink{
		kinds:  map[EventKindName]EventKind{},
		sub:    make(chan Event[interface{}], 5),
		closed: false,
		queue:  []Event[EventKind]{},
		getID: func() (string, error) {
			b := make([]byte, 16)
			_, err := rand.Read(b)
			if err != nil {
				return "", err
			}
			// The following code will make it look like a UUID, but it's not a fully compliant implementation.
			b[6] = (b[6] & 0x0f) | 0x40 // Version 4
			b[8] = (b[8] & 0x3f) | 0x80 // Variant is 10
			return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
		},
	}
	for i := 0; i < len(opts); i++ {
		if err := opts[i](&sink); err != nil {
			return nil, err
		}
	}

	// register internal event to close
	sink.Register(Kind[EventClose]())

	if sink.consumer == nil {
		return nil, fmt.Errorf("consumer function is not configured")
	}
	// Set a finalizer to close the channel when the EventSink is garbage collected
	runtime.SetFinalizer(&sink, func(s *EventSink) {
		fmt.Println("finalizer called")
		s.closed = true
		close(s.sub)
	})

	go func() {
		fmt.Println("start sink", !sink.closed)
		for !sink.closed {
			if len(sink.queue) > 0 {
				fmt.Println("sink open", len(sink.queue))
				// Dequeue the first event and send it to s.sub
				event := sink.queue[0]
				fmt.Println("dequeue ", event)
				sink.sub <- Event[interface{}]{
					Kind: event.Kind,
					Data: event.Data,
				}
				// Remove the first event from the queue
				sink.queue = sink.queue[1:]
			}
		}
		fmt.Println("end sink")
	}()

	return &sink, nil
}

func (s *EventSink) Consume() error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				panic(r)
			}
		}
	}()
	for v := range s.sub {
		fmt.Println("sub", v.Data, v.Kind)
		switch k := v.Kind.(type) {
		case EventKind:
			// internal event
			switch k.None.(type) {
			case EventClose:
				close(s.sub)
				s.closed = true
				fmt.Println("closed sub")
				break
			}
			break
		default:
			s.consumer(v)
		}
	}
	return err
}

func (s *EventSink) Register(kind EventKind) (EventID, error) {
	id, err := s.getID()
	if err != nil {
		return NoID, nil
	}
	kind.ID = EventID(id)
	s.kinds[kind.Info] = kind
	return kind.ID, nil
}

func (s *EventSink) Publish(e EventAnonymous) {
	kind := getKind(e)
	fmt.Println("pushish", kind.Info)
	s.queue = append(s.queue, Event[EventKind]{
		Kind: kind,
		Data: e,
	})
}

func (s *EventSink) Trigger(e EventKind) {
	fmt.Println("trigger", e.Info)
	s.queue = append(s.queue, Event[EventKind]{
		Kind: e,
	})
}

func main() {

	var sink *EventSink
	var err error

	if sink, err = NewEventSink(
		OptionConsumer(func(v Event[interface{}]) {
			fmt.Println("consumer called")
			fmt.Println("event received:", v)
			switch c := v.Data.(type) {
			case Something:
				fmt.Println("value", c.hello)
			}
		}),
	); err != nil {
		panic(err)
	}
	var somethingID EventID
	var elseID EventID

	if somethingID, err = sink.Register(Kind[Something]()); err != nil {
		panic(err)
	}
	if elseID, err = sink.Register(Kind[Else]()); err != nil {
		panic(err)
	}

	fmt.Println(sink, somethingID, elseID)

	sink.Publish(Something{
		hello: "world",
	})

	go func() {
		time.Sleep(time.Second * 2)
		sink.Publish(Something{
			hello: "you can die now",
		})
		sink.Trigger(Kind[EventClose]())
	}()

	if err = sink.Consume(); err != nil {
		panic(err)
	}

	fmt.Println("done")
}

type Something struct {
	hello string
}

type Else struct {
	ok string
}
