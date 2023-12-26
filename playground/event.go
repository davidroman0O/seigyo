package events

import (
	"context"
	"crypto/rand"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/k0kubun/pp/v3"
)

// TODO @droman: i could have phases in which the event is processed and vetted, you could have Publish (queue -> post-process -> sub | internal), Trigger (queue -> post-process -> sub | internal), ImmediatePublish (queue -> sub), ImmediateTrigger (queue -> sub)

// TODO @droman: have dynamic resize to listen in a goroutine if there is activity and have a delta to know when to downsize
type Mediator struct {
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	scheduled   *CircularBuffer[Event[EventKind]]     // events that were queued by user
	transformed *CircularBuffer[Event[interface{}]]   // events that were tranformed by mediator
	queued      *CircularBuffer[[]Event[interface{}]] // events that are ready to be sent
	subscribers []*Subscriber
	getID       func() (string, error)
}

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

var KindNone EventKind = EventKind{
	Kind: reflect.TypeOf(struct{}{}).Kind(),
	Info: InfoNone,
	Data: nil,
}

var EventNone Event[EventKind] = Event[EventKind]{
	ID:   "",
	Kind: KindNone,
	Info: InfoNone,
	Data: nil,
	// what about kind?
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

func OptionMediatorQueuedSize(buffer int) OptionMediator {
	return func(s *Mediator) error {
		s.queued.Resize(buffer)
		return nil
	}
}

func OptionMediatorTransformedSize(buffer int) OptionMediator {
	return func(s *Mediator) error {
		s.transformed.Resize(buffer)
		return nil
	}
}

func OptionMediatorInboxSize(buffer int) OptionMediator {
	return func(s *Mediator) error {
		s.scheduled.Resize(buffer)
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
		scheduled:   NewCircularBuffer[Event[EventKind]](100),
		transformed: NewCircularBuffer[Event[interface{}]](100),
		queued:      NewCircularBuffer[[]Event[interface{}]](100),
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
	go em.runQueued()
	go em.runScheduled()
	go em.runTransformed()
}

func (s *Mediator) AddSubscriber(sub *Subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers = append(s.subscribers, sub)
}

// `tranformKind` allow us to move the data to another container to give more room to flexibility to the subscriber
func (s *Mediator) tranformKind(event Event[EventKind]) Event[interface{}] {
	e := Event[interface{}]{
		ID:        event.ID,
		CreatedAt: event.CreatedAt,
		Info:      event.Info,
		Kind:      event.Kind,
		Data:      event.Data,
	}
	return e
}

func (s *Mediator) runScheduled() {

	for {
		select {
		case <-s.ctx.Done():
			// fmt.Println("mediator context done")
			return
		default:
			if !s.scheduled.IsEmpty() {
				var err error
				var events []Event[EventKind]
				var total int = 10
				if events, total, err = s.scheduled.DequeueN(total); err != nil {
					fmt.Println("Error dequeueing event:", err)
					continue
				}
				// fmt.Println("scheduled dequeue ", total)
				for i := 0; i < len(events); i++ {
					e := s.tranformKind(events[i])
					if err = s.transformed.Enqueue(e); err != nil {
						fmt.Println("Error enqueuing event:", err)
						continue
					}
				}
			}

			// if s.scheduled.IsFractionFull(0.75) {
			// 	s.scheduled.Downsize(ReduceHalf, 0)
			// }

		}
	}
}

func (s *Mediator) runTransformed() {

	for {
		select {
		case <-s.ctx.Done():
			// fmt.Println("mediator context done")
			return
		default:
			if !s.transformed.IsEmpty() {
				var err error
				var events []Event[interface{}]
				var total int = 1000
				if events, total, err = s.transformed.DequeueN(total); err != nil {
					fmt.Println("Error dequeueing event:", err)
					continue
				}
				// fmt.Println("scheduled dequeue ", total)
				// for i := 0; i < len(events); i++ {
				if err = s.queued.Enqueue(events); err != nil {
					fmt.Println("Error enqueuing event:", err)
					continue
				}
				// }
			}

			// if s.scheduled.IsFractionFull(0.75) {
			// 	s.scheduled.Downsize(ReduceHalf, 0)
			// }

		}
	}
}

func (s *Mediator) runQueued() {
	// fmt.Println("mediator started")
	for {
		select {
		case <-s.ctx.Done():
			// fmt.Println("mediator context done")
			return
		default:
			// TODO @droman: we could have multiple queues on different goroutine that all know the different subscribers, it's just they all dispatch all events constantly to increase the i/o
			// TODO @droman: we need to "interrupt" or give room to other goroutines
			if !s.queued.IsEmpty() {
				var err error
				var chunkEvents [][]Event[interface{}]
				var total int = 1000
				if chunkEvents, total, err = s.queued.DequeueN(total); err != nil {
					fmt.Println("Error dequeueing event:", err)
					continue
				}
				askedClose := atomic.Bool{}
				// fmt.Println("deuqueing queue", total)
				// fmt.Println("queued dequeue ", total)
				var wg sync.WaitGroup
				for i := 0; i < len(chunkEvents); i++ {
					wg.Add(1)
					go func(events []Event[interface{}]) {
						pp.Println(events)
						defer wg.Done()
						for idx := 0; idx < len(events); idx++ {
							switch k := events[idx].Kind.(type) {
							case EventKind:
								switch internal := k.Data.(type) {
								case EventSize:
									s.queued.Resize(internal.Size)
									break
								case EventClose:
									askedClose.Store(true)
									break
								default:
									// fmt.Println("distribute")
									s.distribute(events[idx]) // TODO @droman: eventually distribute on another queue
								}
							default:
								fmt.Println("event is not eventkind", k)
								break
							}
						}
					}(chunkEvents[i])
				}

				wg.Wait()

				if askedClose.Load() {
					// fmt.Println("internal event", len(s.subscribers))
					// TODO @droman: trigger close but wait for real closing
					for _, v := range s.subscribers {
						// fmt.Println("close sub channel")
						close(v.Channel)
					}
					s.cancel()
					// fmt.Println("closed sub")
				}

				// for i := 0; i < len(chunkEvents); i++ {
				// 	for ei := 0; ei < len(chunkEvents[i]); ei++ {
				// 		// fmt.Println(i, ei, chunkEvents[i][ei])
				// 		// e := s.tranformKind(events[i]) // TODO @droman: could be in a queue to pre-compute before sending
				// 		// now let see what are we dealing with
				// 		switch k := chunkEvents[i][ei].Kind.(type) {
				// 		case EventKind:
				// 			switch internal := k.Data.(type) {
				// 			case EventSize:
				// 				s.queued.Resize(internal.Size)
				// 				break
				// 			case EventClose:
				// 				// fmt.Println("internal event", len(s.subscribers))
				// 				// TODO @droman: trigger close but wait for real closing
				// 				for _, v := range s.subscribers {
				// 					// fmt.Println("close sub channel")
				// 					close(v.Channel)
				// 				}
				// 				s.cancel()
				// 				// fmt.Println("closed sub")
				// 				break
				// 			default:
				// 				// fmt.Println("distribute")
				// 				s.distribute(chunkEvents[i][ei]) // TODO @droman: eventually distribute on another queue
				// 			}
				// 		default:
				// 			fmt.Println("event is not eventkind", k)
				// 			break
				// 		}
				// 	}
				// }

				// if s.queued.IsFractionFull(0.75) {
				// 	s.queued.Downsize(ReduceHalf, 0)
				// }

				// fmt.Println("mediator queue not empty")
				// event, err := s.queue.Dequeue()
				// if err != nil {
				// 	fmt.Println("Error dequeueing event:", err)
				// 	continue
				// }
				// fmt.Println("mediator dequeued")
				// e := s.tranformKind(event)
				// // now let see what are we dealing with
				// switch k := e.Kind.(type) {
				// case EventKind:
				// 	switch internal := k.Data.(type) {
				// 	case EventSize:
				// 		s.queue.Resize(internal.Size)
				// 		break
				// 	case EventClose:
				// 		// fmt.Println("internal event", len(s.subscribers))
				// 		// TODO @droman: trigger close but wait for real closing
				// 		for _, v := range s.subscribers {
				// 			// fmt.Println("close sub channel")
				// 			close(v.Channel)
				// 		}
				// 		s.cancel()
				// 		// fmt.Println("closed sub")
				// 		break
				// 	default:
				// 		// fmt.Println("distribute")
				// 		s.distribute(e)
				// 	}
				// default:
				// 	fmt.Println("event is not eventkind", k)
				// 	break
				// }
			}
		}
	}
}

func (s *Mediator) distribute(event Event[interface{}]) {
	// TODO @droman: we need to optmize that, i don't want to loop and i want directly access and check
	// fmt.Println("distribute ", event.Info, event.Kind)
	kind := event.Kind.(EventKind)

	for _, sub := range s.subscribers {

		if sub == nil {
			fmt.Printf("subscriber is nil")
		}
		if sub.InterestedIn == nil {
			fmt.Printf("InterestedIn is nil")
		}

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
	// fmt.Println("publish", event.Info)
	if err := s.scheduled.Enqueue(event); err != nil {
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
	var kind EventKind
	name := getNameKind(e)
	if hasKind(name) {
		kind = getKind(name)
	} else {
		kind = newKind(e)
	}
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
	if err = s.scheduled.Enqueue(event); err != nil {
		fmt.Println("Error enqueueing event:", err)
		return err
	}
	return nil
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
