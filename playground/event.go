package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// TODO @droman: i could have phases in which the event is processed and vetted, you could have Publish (queue -> post-process -> sub | internal), Trigger (queue -> post-process -> sub | internal), ImmediatePublish (queue -> sub), ImmediateTrigger (queue -> sub)

type Event[EK any] struct {
	ID        EventID
	CreatedAt EventCreated
	Info      EventKindName
	Kind      EK
	Data      interface{}
}

type EventKindName string

type EventKind struct {
	ID     EventID
	Kind   reflect.Kind
	Info   EventKindName
	IsNone bool
	Data   interface{}
}

var InfoNone EventKindName = "None"

var EventNone Event[EventKind] = Event[EventKind]{
	ID:   "",
	Info: InfoNone,
	Data: nil,
	// what about kind?
}

type SubscriberID string

type Subscriber struct {
	ID           SubscriberID
	InterestedIn map[EventKindName]EventKind
	Channel      chan Event[interface{}]
	mu           sync.Mutex
}

type OptionSubscriber func(s *Subscriber) error

func OptionSubscriberID(id string) OptionSubscriber {
	return func(s *Subscriber) error {
		s.ID = SubscriberID(id)
		return nil
	}
}

func OptionSubscriberInterestedIn(es ...EventKind) OptionSubscriber {
	return func(s *Subscriber) error {
		for i := 0; i < len(es); i++ {
			s.InterestedIn[es[i].Info] = es[i]
		}
		return nil
	}
}

func NewSubscriber(opts ...OptionSubscriber) (*Subscriber, error) {
	sub := Subscriber{
		InterestedIn: make(map[EventKindName]EventKind),
		Channel:      make(chan Event[interface{}]),
	}
	for i := 0; i < len(opts); i++ {
		if err := opts[i](&sub); err != nil {
			return nil, err
		}
	}
	if sub.ID == "" {
		var id string
		var err error
		if id, err = getID(); err != nil {
			return nil, err
		}
		sub.ID = SubscriberID(id)
	}
	return &sub, nil
}

// `Register` dynamically a new event to listen to
func (s *Subscriber) Register(kind EventKind) (EventID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.InterestedIn[kind.Info] = kind
	return kind.ID, nil
}

// TODO @droman: have dynamic resize to listen in a goroutine if there is activity and have a delta to know when to downsize
type Mediator struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	// kinds  map[EventKindName]EventKind
	queue CircularBuffer
	// sub      chan Event[interface{}]
	subscribers []*Subscriber
	// closed      bool
	getID func() (string, error)
	// consumer func(subE Event[interface{}])
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

type EventSize struct {
	Size int
}

type EventID string
type EventCreated time.Time

var (
	NoID EventID = ""
)

type OptionMediator func(s *Mediator) error

func OptionMediatorGetID(fn func() (string, error)) OptionMediator {
	return func(s *Mediator) error {
		s.getID = fn
		return nil
	}
}

func OptionMediatorSize(buffer int) OptionMediator {
	return func(s *Mediator) error {
		s.queue.Resize(buffer)
		return nil
	}
}

type OptionMediatorSub func(opts ...OptionSubscriber) (*Subscriber, error)

// TODO @droman: this is kinda dumb, how to i get the ID?! I'm not sure if i will keep it
func OptionMediatorNewSubscriber(opts ...OptionSubscriber) OptionMediatorSub {
	return func(opts ...OptionSubscriber) (*Subscriber, error) {
		return NewSubscriber(opts...)
	}
}

// TODO @droman: this is kinda dumb, how to i get the ID?! I'm not sure if i will keep it
func OptionMediatorSubscribers(subs ...OptionMediatorSub) OptionMediator {
	return func(s *Mediator) error {
		var err error
		for i := 0; i < len(subs); i++ {
			var sub *Subscriber
			if sub, err = subs[i](); err != nil {
				return err
			}
			s.subscribers = append(s.subscribers, sub)
		}

		return nil
	}
}

func OptionMediatorSubscriber(subs *Subscriber) OptionMediator {
	return func(s *Mediator) error {

		s.subscribers = append(s.subscribers, subs)
		return nil
	}
}

// `Kind` is helping the mediator to know what is the mapping of your event
func Kind[T any]() EventKind {
	proxy := *new(T)
	kind := EventKind{
		Kind:   reflect.TypeOf(proxy).Kind(),
		Info:   EventKindName(reflect.ValueOf(proxy).Type().Name()),
		IsNone: true,
		Data:   proxy,
	}
	return kind
}

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

// `NewMediator` create an event mediator that will manage subscription, context, queues, publishing, triggering and more
func NewMediator(ctx context.Context, opts ...OptionMediator) (*Mediator, error) {
	ctx, cancel := context.WithCancel(ctx)
	sink := Mediator{
		ctx:         ctx,
		cancel:      cancel,
		subscribers: []*Subscriber{},
		queue:       *NewCircularBuffer(100),
		getID:       getID,
	}
	for i := 0; i < len(opts); i++ {
		if err := opts[i](&sink); err != nil {
			return nil, err
		}
	}
	return &sink, nil
}

func (em *Mediator) Start() {
	go em.dispatch()
}

func (s *Mediator) AddSubscriber(sub *Subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers = append(s.subscribers, sub)
}

func (s *Mediator) dispatch() {
	var now time.Time
	var e Event[interface{}]
	// fmt.Println("mediator started")
	for {
		select {
		case <-s.ctx.Done():
			// fmt.Println("mediator context done")
			return
		default:
			// TODO @droman: we could have multiple queues on different goroutine that all know the different subscribers, it's just they all dispatch all events constantly to increase the i/o

			if !s.queue.IsEmpty() {
				// fmt.Println("mediator queue not empty")
				event, err := s.queue.Dequeue()
				if err != nil {
					fmt.Println("Error dequeueing event:", err)
					continue
				}
				// fmt.Println("mediator dequeued")
				now = time.Now()
				e = Event[interface{}]{
					CreatedAt: EventCreated(now),
					Info:      e.Info,
					Kind:      event.Kind,
					Data:      event.Data,
				}
				switch k := e.Kind.(type) {
				case EventKind:
					switch internal := k.Data.(type) {
					case EventSize:
						s.queue.Resize(internal.Size)
						break
					case EventClose:
						// fmt.Println("internal event", len(s.subscribers))
						// TODO @droman: trigger close but wait for real closing
						for _, v := range s.subscribers {
							fmt.Println("close sub channel")
							close(v.Channel)
						}
						s.cancel()
						fmt.Println("closed sub")
						break
					default:
						// fmt.Println("distribute")
						s.distribute(e)
					}
				default:
					fmt.Println("event is not eventkind", k)
					break
				}
			}
		}
	}
}

func (s *Mediator) distribute(event Event[interface{}]) {
	// TODO @droman: we need to optmize that, i don't want to loop and i want directly access and check
	// fmt.Println("distribute ", event.Info, event.Kind)
	kind := event.Kind.(EventKind)

	for _, sub := range s.subscribers {

		if _, ok := sub.InterestedIn[kind.Info]; !ok {
			// fmt.Println("subscriber wasn't interested", event.Info)
			continue
		}

		// fmt.Println("published to subscribeer", event)
		sub.Channel <- event
	}
}

// `Publish` will process an event to reach an intereted subscribers
func (s *Mediator) PublishTo(subscriberID string, e EventAnonymous) error {
	return nil
}

// `ImmediatePublishTo` will skip any process and direclty send the event, it might hurt performances slightly
func (s *Mediator) ImmediatePublishTo(subscriberID string, e EventAnonymous) error {
	return nil
}

// `ImmediatePublish` will skip any process and direclty broadcast the event, it might hurt performances slightly
func (s *Mediator) ImmediatePublish(e EventAnonymous) error {
	return nil
}

// `Publish` will process an event to reach all intereted subscribers, it's a broadcast
func (s *Mediator) Publish(e EventAnonymous) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	var event Event[EventKind]
	if event, err = s.getEventFromAnonymous(e); err != nil {
		return err
	}
	fmt.Println("publish", event.Info)
	if err := s.queue.Enqueue(event); err != nil {
		fmt.Println("Error enqueueing event:", err)
		return err
	}
	return nil
}

// `ImmediateTriggerTo`
func (s *Mediator) ImmediateTriggerTo(e EventKind) error {
	return nil
}

// `ImmediateTrigger`
func (s *Mediator) ImmediateTrigger(e EventKind) error {
	return nil
}

// `TriggerTo`
func (s *Mediator) TriggerTo(e EventKind) error {
	return nil
}

func (s *Mediator) getEventFromAnonymous(e EventAnonymous) (Event[EventKind], error) {
	// extract kind data from anonymous event
	kind := getKind(e)
	var id string
	var err error
	// getting id from user or default
	if id, err = s.getID(); err != nil {
		return EventNone, err
	}
	event := Event[EventKind]{
		ID:        EventID(id),
		Info:      kind.Info, // we need to know the name
		Kind:      kind,      // we need to know what type of data
		Data:      e,         // we need to have the data the user is giving
		CreatedAt: EventCreated(time.Now()),
	}
	return event, nil
}

func (s *Mediator) getEventFromKind(e EventKind) (Event[EventKind], error) {
	var id string
	var err error
	// getting id from user or default
	if id, err = s.getID(); err != nil {
		return EventNone, err
	}
	event := Event[EventKind]{
		ID:        EventID(id),
		Info:      e.Info,
		Kind:      e,
		CreatedAt: EventCreated(time.Now()),
		Data:      nil,
	}
	return event, err
}

// `Trigger`
func (s *Mediator) Trigger(e EventKind) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	var event Event[EventKind]
	if event, err = s.getEventFromKind(e); err != nil {
		return err
	}
	// fmt.Println("trigger", e.Info)
	if err = s.queue.Enqueue(event); err != nil {
		fmt.Println("Error enqueueing event:", err)
		return err
	}
	return nil
}

func main() {

	var mediator *Mediator
	var err error

	var subscriber *Subscriber

	if subscriber, err = NewSubscriber(
		OptionSubscriberInterestedIn(
			Kind[Something](),
			Kind[Else](),
		),
	); err != nil {
		panic(err)
	}

	if mediator, err = NewMediator(
		context.Background(),
		OptionMediatorSubscriber(subscriber),
		OptionMediatorSize(20),
	// OptionMediatorConsumer(func(v Event[interface{}]) {
	// 	// fmt.Println("consumer called")
	// 	// fmt.Println("event received:", v)
	// 	switch c := v.Data.(type) {
	// 	case Something:
	// 		fmt.Println("value", c.hello)
	// 		// if c.hello == "world" {
	// 		// 	fmt.Printf("Current time: %v\n", time.Now())
	// 		// 	fmt.Printf("Event creation time: %v\n", time.Time(v.CreatedAt))
	// 		// 	elapsed := time.Since(time.Time(v.CreatedAt))
	// 		// 	fmt.Printf("time since event creation: %s\n", elapsed)
	// 		// }
	// 		// if c.hello == "die" {
	// 		// 	fmt.Printf("Current time: %v\n", time.Now())
	// 		// 	fmt.Printf("Event creation time: %v\n", time.Time(v.CreatedAt))
	// 		// 	elapsed := time.Since(time.Time(v.CreatedAt))
	// 		// 	fmt.Printf("time since event creation: %s\n", elapsed)
	// 		// }
	// 		// if c.hello == "two" {
	// 		// 	fmt.Printf("Current time: %v\n", time.Now())
	// 		// 	fmt.Printf("Event creation time: %v\n", time.Time(v.CreatedAt))
	// 		// 	elapsed := time.Since(time.Time(v.CreatedAt))
	// 		// 	fmt.Printf("time since event creation: %s\n", elapsed)
	// 		// }
	// 		// if c.hello == "three" {
	// 		// 	fmt.Printf("Current time: %v\n", time.Now())
	// 		// 	fmt.Printf("Event creation time: %v\n", time.Time(v.CreatedAt))
	// 		// 	elapsed := time.Since(time.Time(v.CreatedAt))
	// 		// 	fmt.Printf("time since event creation: %s\n", elapsed)
	// 		// }
	// 	}
	// }
	// ),
	); err != nil {
		panic(err)
	}

	mediator.Start()

	go func() {
		// time.Sleep(time.Second * 1)
		// TODO @droman: have a thing that allow the main goroutine to work too
		for j := 0; j < 5; j++ {
			if err := mediator.Publish(Something{hello: "test"}); err != nil {
				fmt.Println("panic ", err)
				panic(err)
			}
		}
		mediator.Publish(EventSize{
			Size: 30,
		})
		mediator.Trigger(Kind[EventClose]())
	}()

	for event := range subscriber.Channel {
		fmt.Println("Received event:", event)
	}

	// fmt.Println("done")
}

type Something struct {
	hello string
}

type Else struct {
	ok string
}

type CircularBuffer struct {
	events   []Event[EventKind]
	head     int
	tail     int
	size     int
	capacity int
	resize   bool
}

func NewCircularBuffer(capacity int) *CircularBuffer {
	buffer := &CircularBuffer{
		events:   make([]Event[EventKind], capacity),
		capacity: capacity,
		resize:   true,
	}
	buffer.Resize(capacity)
	return buffer
}

func (cb *CircularBuffer) Enqueue(event Event[EventKind]) error {
	if cb.IsFull() {
		fmt.Println("is full enqueue", cb.IsFull())
		if cb.resize {
			fmt.Println("enqueue resize!", cb.size, "->", cb.capacity*2)
			cb.Resize(cb.capacity * 2)
		} else {
			return fmt.Errorf("buffer is full")
		}
	}
	cb.events[cb.tail] = event
	cb.tail = (cb.tail + 1) % cb.capacity
	cb.size++
	return nil
}

func (cb *CircularBuffer) Dequeue() (Event[EventKind], error) {
	if cb.IsEmpty() {
		return Event[EventKind]{}, fmt.Errorf("buffer is empty")
	}
	event := cb.events[cb.head]
	cb.head = (cb.head + 1) % cb.capacity
	cb.size--
	return event, nil
}

func (cb *CircularBuffer) IsFull() bool {
	fmt.Println("is full", cb.size == cb.capacity, cb.size, cb.capacity)
	return cb.size == cb.capacity
}

func (cb *CircularBuffer) IsEmpty() bool {
	return cb.size == 0
}

func (cb *CircularBuffer) Resize(newCapacity int) {
	fmt.Println("resize!", cb.capacity, "->", newCapacity)
	newBuffer := make([]Event[EventKind], newCapacity)
	for i := 0; i < cb.size; i++ {
		newBuffer[i] = cb.events[(cb.head+i)%cb.capacity]
	}
	cb.events = newBuffer
	cb.head = 0
	cb.tail = cb.size
	cb.capacity = newCapacity
}
