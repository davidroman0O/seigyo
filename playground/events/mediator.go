package events

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/davidroman0O/multigroup"
)

/// TODO @droman: i have 1M/s but i think that if i track with `atomic` the throughput of the registry processing
/// i could ease the channels by runtime.GoShed (i think it's that) to help the scheduler know it can compute something else
/// I really want to reach 5-10M/s simply because i need room for compute for the next module after `events`
/// Basically, he is giving a callback to be scheduled if there is a thredshold reached, process in batch and then runtime.Goshed

var (
	ErrEventGotNoKind = errors.New("event got no kind")
	ErrEventGotNoID   = errors.New("event got no id")
	ErrEventEnqueued  = errors.New("event couldn't be enqueued")
	ErrBufferNotFound = errors.New("buffer not found")
)

// TODO @droman: i could have phases in which the event is processed and vetted, you could have Publish (queue -> post-process -> sub | internal), Trigger (queue -> post-process -> sub | internal), ImmediatePublish (queue -> sub), ImmediateTrigger (queue -> sub)

// `mediatorStatic` hold a static copies of data used for comparison
type mediatorStatic struct {
	EventNone Event[interface{}]
	KindNone  Kind
}

func newStatic() mediatorStatic {
	return mediatorStatic{
		EventNone: EventNone,
		KindNone:  KindNone,
	}
}

// TODO @droman: have dynamic resize to listen in a goroutine if there is activity and have a delta to know when to downsize
// TODO @droman: different types of queues
type Mediator struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc

	// all broadcasted events will be managed by the `globalRegistry`
	globalRegistry *EventRegistry
	enqueue        *CircularBuffer[Event[interface{}]]

	subscribers []*Subscriber
	getID       func() (string, error)
	static      mediatorStatic // contains all static data computed at init

	// scheduler *BatchScheduler
	throughput int
	procStatus int32
}

type OptionMediator func(s *Mediator) error

func OptionMediatorGetID(fn FnEventID) OptionMediator {
	return func(s *Mediator) error {
		s.getID = fn
		return nil
	}
}

// func OptionMediatorQueuedSize(buffer int) OptionMediator {
// 	return func(s *Mediator) error {
// 		s.queued.Resize(buffer)
// 		return nil
// 	}
// }

// func OptionMediatorTransformedSize(buffer int) OptionMediator {
// 	return func(s *Mediator) error {
// 		s.transformed.Resize(buffer)
// 		return nil
// 	}
// }

// func OptionMediatorInboxSize(buffer int) OptionMediator {
// 	return func(s *Mediator) error {
// 		s.scheduled.Resize(buffer)
// 		return nil
// 	}
// }

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

// `NewMediator` create an event mediator that will manage subscription, context, queues, publishing, triggering and more
func NewMediator(ctx context.Context, opts ...OptionMediator) (*Mediator, error) {
	ctx, cancel := context.WithCancel(ctx)
	sink := Mediator{
		ctx:         ctx,
		cancel:      cancel,
		subscribers: []*Subscriber{},
		getID:       getID,
		enqueue:     NewCircularBuffer[Event[interface{}]](messageBatchSize),
		// scheduler:   NewBatchScheduler(messageBatchSize, defaultThroughput),
		throughput: 10,
	}
	var err error
	for i := 0; i < len(opts); i++ {
		if err = opts[i](&sink); err != nil {
			return nil, err
		}
	}
	if sink.globalRegistry == nil {
		if sink.globalRegistry, err = newEventRegistry(
			OptionEventRegistryGetID(sink.getID),
		); err != nil {
			return nil, err
		}
	}
	return &sink, nil
}

func (em *Mediator) Start() {
	// go em.runGlobalRegistry()

	// dequeue queue
	// go func() {
	// 	for {
	// 		select {
	// 		case <-em.ctx.Done():
	// 			fmt.Println("mediator global registry context done")
	// 			return
	// 		default:

	// 			em.schedule()
	// 		}
	// 	}
	// }()
	// go em.scheduler.Run()
}

func (s *Mediator) AddSubscriber(sub *Subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers = append(s.subscribers, sub)
}

// `tranformKind` allow us to move the data to another container to give more room to flexibility to the subscriber
func (s *Mediator) tranformKind(event Event[interface{}]) Event[interface{}] {
	e := Event[interface{}]{
		ID:        event.ID,
		CreatedAt: event.CreatedAt,
		Kind:      event.Kind,
		Data:      event.Data,
	}
	return e
}

func (s *Mediator) close() {
	atomic.StoreInt32(&s.procStatus, stopped)
	for _, v := range s.subscribers {
		fmt.Println("close channel subscriber", v.Channel)
		close(v.Channel)
	}
	// s.cancel()
	// s.scheduler.Stop()
}

func (s *Mediator) dequeueGlobalRegistry() error {

	var wg sync.WaitGroup

	// fmt.Println("dequeue")
	for _, kindID := range KindRegistry.Keys() {
		wg.Add(1)

		// fmt.Println("dequeue kind", kindID)
		go func(kind EventKindID) {
			defer wg.Done()

			var ok bool
			var err error
			var events []Event[interface{}]
			var buffer *CircularBuffer[Event[interface{}]]

			// fmt.Println("dequeue get kind", kind)
			if buffer, ok = s.globalRegistry.Get(kind); !ok {
				// which is fine because we're are not interested in it
				// fmt.Printf("failed to get buffer %v \n", kindID)
				return
			}

			if buffer.IsEmpty() {
				// fmt.Println("dequeue kind empty", kind)
				return
			}

			// fmt.Println("dequeueing kind", kind)
			if events, _, err = buffer.DequeueN(messageBatchSize); err != nil {
				fmt.Printf("failed to dequeue %v %v: %v \n", messageBatchSize, kind, err.Error())
				return
			}

			// we are broadcasting event to all subscibers
			for idx := 0; idx < len(events); idx++ {

				for _, sub := range s.subscribers {
					if sub == nil {
						fmt.Printf("subscriber is nil")
					}
					if sub.InterestedIn == nil {
						fmt.Printf("InterestedIn is nil")
					}
					if _, ok := sub.InterestedIn[events[idx].Kind]; !ok {
						fmt.Println("subscriber wasn't interested", events[idx].Kind)
						continue
					}
					// fmt.Println("published to subscribeer", event)
					sub.Channel <- events[idx]
				}
			}
		}(kindID)
	}

	wg.Wait()

	return nil
}

func (em *Mediator) runGroups() {
	eventKindSelector := func(p Event[interface{}]) (string, string) { return "Kind", string(p.Kind) }
	var ok bool
	var err error
	var events []Event[interface{}]
	var buffer *CircularBuffer[Event[interface{}]]
	// fmt.Println("dequeue groups")
	if em.enqueue.IsEmpty() {
		return
	}
	if events, _, err = em.enqueue.DequeueN(messageBatchSize); err != nil {
		fmt.Printf("failed to dequeue %v %v: %v \n", messageBatchSize, err.Error())
		return
	}

	externalEvents := []Event[interface{}]{}
	groupedEvents := multigroup.By(events, eventKindSelector)
	// fmt.Println("group of events ", len(groupedEvents))
	for i := 0; i < len(groupedEvents); i++ {
		kind := EventKindID(groupedEvents[i].Keys[0].Value)
		if !em.globalRegistry.Has(EventKindID(kind)) {
			em.globalRegistry.Set(kind, NewCircularBuffer[Event[interface{}]](messageBatchSize))
		}
		if buffer, ok = em.globalRegistry.Get(kind); !ok {
			// which is fine because we're are not interested in it
			fmt.Printf("failed to get buffer %v \n", kind)
			return
		}
		// fmt.Println("group of events count ", len(groupedEvents[i].Items))
		for idx := 0; idx < len(groupedEvents[i].Items); idx++ {

			switch groupedEvents[i].Items[idx].Data.(type) {

			case EventSize:
				// TODO: make a way to capture internal events from the get go into an internal registries
				fmt.Println("event resize")
				break

			case EventClose:
				// TODO: make a way to capture internal events from the get go into an internal registries
				// TODO: DO NOT MANAGE INTERNAL EVENT THAT WAY, IT IS SUPER DUMB, i'm just testing stuff
				fmt.Println("event close")
				em.close()
				continue

			default:
				externalEvents = append(externalEvents, groupedEvents[i].Items[idx])
				continue
			}
		}
		buffer.EnqueueN(externalEvents)
	}
}

func (in *Mediator) schedule() {
	if atomic.CompareAndSwapInt32(&in.procStatus, idle, running) {
		go in.process()
	}
}

func (in *Mediator) process() {
	in.runGlobalRegistry()
	atomic.StoreInt32(&in.procStatus, idle)
}

// Every broadcasted events will be managed by the `globalRegistry`
func (s *Mediator) runGlobalRegistry() {
	i, t := 0, s.throughput
	// fmt.Println("run global registry")
	for atomic.LoadInt32(&s.procStatus) != stopped {
		if i > t {
			i = 0
			runtime.Gosched()
		}
		i++
		// fmt.Println(i, t)
		s.runGroups()
		if err := s.dequeueGlobalRegistry(); err != nil {
			fmt.Println(err)
			return
		}
	}

	// for {
	// 	select {
	// 	case <-s.ctx.Done():
	// 		fmt.Println("mediator global registry context done")
	// 		return
	// 	default:
	// 		// TODO @droman create workers that will manage different kinds for this global registry
	// 		var ok bool
	// 		var err error
	// 		var events []Event[interface{}]

	// 		var buffer *CircularBuffer[Event[interface{}]]
	// 		for _, kindID := range KindRegistry.Keys() {
	// 			if buffer, ok = s.globalRegistry.Get(kindID); !ok {
	// 				// which is fine because we're are not interested in it
	// 				// fmt.Printf("failed to get buffer %v \n", kindID)
	// 				continue
	// 			}
	// 			if buffer.IsEmpty() {
	// 				continue
	// 			}
	// 			if events, _, err = buffer.DequeueN(defaultThroughput); err != nil {
	// 				fmt.Printf("failed to dequeue %v %v: %v \n", defaultThroughput, kindID, err.Error())
	// 				continue
	// 			}
	// 			// we are broadcasting event to all subscibers
	// 			for idx := 0; idx < len(events); idx++ {

	// 				switch events[idx].Data.(type) {

	// 				case EventSize:
	// 					// TODO: make a way to capture internal events from the get go into an internal registries
	// 					fmt.Println("event resize")
	// 					break
	// 				case EventClose:
	// 					// TODO: make a way to capture internal events from the get go into an internal registries
	// 					// TODO: DO NOT MANAGE INTERNAL EVENT THAT WAY, IT IS SUPER DUMB, i'm just testing stuff
	// 					fmt.Println("event close")
	// 					s.close()
	// 					return
	// 				}

	// 				for _, sub := range s.subscribers {
	// 					if sub == nil {
	// 						fmt.Printf("subscriber is nil")
	// 					}
	// 					if sub.InterestedIn == nil {
	// 						fmt.Printf("InterestedIn is nil")
	// 					}
	// 					if _, ok := sub.InterestedIn[events[idx].Kind]; !ok {
	// 						fmt.Println("subscriber wasn't interested", events[idx].Kind)
	// 						continue
	// 					}
	// 					// fmt.Println("published to subscribeer", event)
	// 					sub.Channel <- events[idx]
	// 				}
	// 				// s.distribute(events[idx])
	// 				// switch k := events[idx].Data.(type) {
	// 				// case Kind:
	// 				// 	fmt.Println(k)
	// 				// 	// switch internal := k.(type) {
	// 				// 	// // case EventSize:
	// 				// 	// // 	s.queued.Resize(internal.Size)
	// 				// 	// // 	break
	// 				// 	// // case EventClose:
	// 				// 	// // 	askedClose.Store(true)
	// 				// 	// // 	break
	// 				// 	// default:
	// 				// 	// 	// fmt.Println("distribute")
	// 				// 	// 	s.distribute(events[idx]) // TODO @droman: eventually distribute on another que
	// 				// 	// }
	// 				// default:
	// 				// 	fmt.Println("event is not eventkind", k)
	// 				// 	break
	// 				// }
	// 			}
	// 		}
	// 	}
	// }
}

// func (s *Mediator) runScheduled() {

// 	for {
// 		select {
// 		case <-s.ctx.Done():
// 			// fmt.Println("mediator context done")
// 			return
// 		default:
// 			if !s.scheduled.IsEmpty() {
// 				var err error
// 				var events []Event[interface{}]
// 				var total int = 10
// 				if events, total, err = s.scheduled.DequeueN(total); err != nil {
// 					fmt.Println("Error dequeueing event:", err)
// 					continue
// 				}
// 				// fmt.Println("scheduled dequeue ", total)
// 				for i := 0; i < len(events); i++ {
// 					e := s.tranformKind(events[i])
// 					if err = s.transformed.Enqueue(e); err != nil {
// 						fmt.Println("Error enqueuing event:", err)
// 						continue
// 					}
// 				}
// 			}

// 			// if s.scheduled.IsFractionFull(0.75) {
// 			// 	s.scheduled.Downsize(ReduceHalf, 0)
// 			// }

// 		}
// 	}
// }

// func (s *Mediator) runTransformed() {

// 	for {
// 		select {
// 		case <-s.ctx.Done():
// 			// fmt.Println("mediator context done")
// 			return
// 		default:
// 			if !s.transformed.IsEmpty() {
// 				var err error
// 				var events []Event[interface{}]
// 				var total int = 1000
// 				if events, total, err = s.transformed.DequeueN(total); err != nil {
// 					fmt.Println("Error dequeueing event:", err)
// 					continue
// 				}
// 				// fmt.Println("scheduled dequeue ", total)
// 				// for i := 0; i < len(events); i++ {
// 				if err = s.queued.Enqueue(events); err != nil {
// 					fmt.Println("Error enqueuing event:", err)
// 					continue
// 				}
// 				// }
// 			}

// 			// if s.scheduled.IsFractionFull(0.75) {
// 			// 	s.scheduled.Downsize(ReduceHalf, 0)
// 			// }

// 		}
// 	}
// }

// func (s *Mediator) runQueued() {
// 	// fmt.Println("mediator started")
// 	for {
// 		select {
// 		case <-s.ctx.Done():
// 			// fmt.Println("mediator context done")
// 			return
// 		default:
// 			// TODO @droman: we could have multiple queues on different goroutine that all know the different subscribers, it's just they all dispatch all events constantly to increase the i/o
// 			// TODO @droman: we need to "interrupt" or give room to other goroutines
// 			if !s.queued.IsEmpty() {
// 				// var err error
// 				// var chunkEvents [][]Event[interface{}]
// 				// var total int = 1000
// 				// if chunkEvents, total, err = s.queued.DequeueN(total); err != nil {
// 				// 	fmt.Println("Error dequeueing event:", err)
// 				// 	continue
// 				// }
// 				askedClose := atomic.Bool{}
// 				// fmt.Println("deuqueing queue", total)
// 				// fmt.Println("queued dequeue ", total)
// 				var wg sync.WaitGroup
// 				// for i := 0; i < len(chunkEvents); i++ {
// 				// 	wg.Add(1)
// 				// 	go func(events []Event[interface{}]) {
// 				// 		pp.Println(events)
// 				// 		defer wg.Done()
// 				// 		for idx := 0; idx < len(events); idx++ {
// 				// 			switch k := events[idx].Kind.(type) {
// 				// 			case Kind:
// 				// 				switch internal := k.Data.(type) {
// 				// 				case EventSize:
// 				// 					s.queued.Resize(internal.Size)
// 				// 					break
// 				// 				case EventClose:
// 				// 					askedClose.Store(true)
// 				// 					break
// 				// 				default:
// 				// 					// fmt.Println("distribute")
// 				// 					s.distribute(events[idx]) // TODO @droman: eventually distribute on another queue
// 				// 				}
// 				// 			default:
// 				// 				fmt.Println("event is not eventkind", k)
// 				// 				break
// 				// 			}
// 				// 		}
// 				// 	}(chunkEvents[i])
// 				// }

// 				wg.Wait()

// 				if askedClose.Load() {
// 					// fmt.Println("internal event", len(s.subscribers))
// 					// TODO @droman: trigger close but wait for real closing
// 					for _, v := range s.subscribers {
// 						// fmt.Println("close sub channel")
// 						close(v.Channel)
// 					}
// 					s.cancel()
// 					// fmt.Println("closed sub")
// 				}

// 				// for i := 0; i < len(chunkEvents); i++ {
// 				// 	for ei := 0; ei < len(chunkEvents[i]); ei++ {
// 				// 		// fmt.Println(i, ei, chunkEvents[i][ei])
// 				// 		// e := s.tranformKind(events[i]) // TODO @droman: could be in a queue to pre-compute before sending
// 				// 		// now let see what are we dealing with
// 				// 		switch k := chunkEvents[i][ei].Kind.(type) {
// 				// 		case EventKind:
// 				// 			switch internal := k.Data.(type) {
// 				// 			case EventSize:
// 				// 				s.queued.Resize(internal.Size)
// 				// 				break
// 				// 			case EventClose:
// 				// 				// fmt.Println("internal event", len(s.subscribers))
// 				// 				// TODO @droman: trigger close but wait for real closing
// 				// 				for _, v := range s.subscribers {
// 				// 					// fmt.Println("close sub channel")
// 				// 					close(v.Channel)
// 				// 				}
// 				// 				s.cancel()
// 				// 				// fmt.Println("closed sub")
// 				// 				break
// 				// 			default:
// 				// 				// fmt.Println("distribute")
// 				// 				s.distribute(chunkEvents[i][ei]) // TODO @droman: eventually distribute on another queue
// 				// 			}
// 				// 		default:
// 				// 			fmt.Println("event is not eventkind", k)
// 				// 			break
// 				// 		}
// 				// 	}
// 				// }

// 				// if s.queued.IsFractionFull(0.75) {
// 				// 	s.queued.Downsize(ReduceHalf, 0)
// 				// }

// 				// fmt.Println("mediator queue not empty")
// 				// event, err := s.queue.Dequeue()
// 				// if err != nil {
// 				// 	fmt.Println("Error dequeueing event:", err)
// 				// 	continue
// 				// }
// 				// fmt.Println("mediator dequeued")
// 				// e := s.tranformKind(event)
// 				// // now let see what are we dealing with
// 				// switch k := e.Kind.(type) {
// 				// case EventKind:
// 				// 	switch internal := k.Data.(type) {
// 				// 	case EventSize:
// 				// 		s.queue.Resize(internal.Size)
// 				// 		break
// 				// 	case EventClose:
// 				// 		// fmt.Println("internal event", len(s.subscribers))
// 				// 		// TODO @droman: trigger close but wait for real closing
// 				// 		for _, v := range s.subscribers {
// 				// 			// fmt.Println("close sub channel")
// 				// 			close(v.Channel)
// 				// 		}
// 				// 		s.cancel()
// 				// 		// fmt.Println("closed sub")
// 				// 		break
// 				// 	default:
// 				// 		// fmt.Println("distribute")
// 				// 		s.distribute(e)
// 				// 	}
// 				// default:
// 				// 	fmt.Println("event is not eventkind", k)
// 				// 	break
// 				// }
// 			}
// 		}
// 	}
// }

func (s *Mediator) distribute(event Event[interface{}]) {
	// TODO @droman: we need to optmize that, i don't want to loop and i want directly access and check
	// fmt.Println("distribute ", event.Info, event.Kind)
	// kind := event.Kind.(Kind)

	// for _, sub := range s.subscribers {

	// 	if sub == nil {
	// 		fmt.Printf("subscriber is nil")
	// 	}
	// 	if sub.InterestedIn == nil {
	// 		fmt.Printf("InterestedIn is nil")
	// 	}

	// 	if _, ok := sub.InterestedIn[kind.Info]; !ok {
	// 		// fmt.Println("subscriber wasn't interested", event.Info)
	// 		continue
	// 	}

	// 	// fmt.Println("published to subscribeer", event)
	// 	sub.Channel <- event
	// }
}

// `getEventFromAnonymous` is an helper function to wrap a data into an event
func (s *Mediator) getEventFromAnonymous(e EventAnonymous) (*Event[interface{}], error) {
	// extract kind data from anonymous event
	return s.globalRegistry.Register(newUnregisteredEvent(e))
}

func (s *Mediator) getEventFromKind(kind Kind) (*Event[interface{}], error) {
	return s.globalRegistry.Register(newUnregisteredKindEvent(kind))
}

// `Publish` will process an event to reach all intereted subscribers, it's a broadcast
func (s *Mediator) Publish(e EventAnonymous) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	var event *Event[interface{}]
	if event, err = s.getEventFromAnonymous(e); err != nil {
		return err
	}

	defer s.schedule()
	// s.scheduler.Schedule(s.dequeueGlobalRegistry)
	// it is the struct directly
	// switch e.(type) {
	// case EventSize:
	// 	// TODO: make a way to capture internal events from the get go into an internal registries
	// 	fmt.Println("event resize")
	// 	break
	// case EventClose:
	// 	// TODO: make a way to capture internal events from the get go into an internal registries
	// 	fmt.Println("event close")
	// 	break
	// }
	return s.enqueue.Enqueue(*event)
	// fmt.Println("publish", event.Info)
	// return s.globalRegistry.Enqueue(event)
}

// `Trigger`
func (s *Mediator) Trigger(e Kind) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	var event *Event[interface{}]
	if event, err = s.getEventFromKind(e); err != nil {
		return err
	}

	defer s.schedule()
	// defer s.schedule()
	// s.scheduler.Schedule(s.dequeueGlobalRegistry)
	// we can have the type through the `Kind`
	// switch e.TypeInfo.(type) {
	// case EventSize:
	// 	// TODO: make a way to capture internal events from the get go into an internal registries
	// 	fmt.Println("event resize")
	// 	break
	// case EventClose:
	// 	// TODO: make a way to capture internal events from the get go into an internal registries
	// 	fmt.Println("event close")
	// 	break
	// }
	// return s.globalRegistry.Enqueue(event)
	return s.enqueue.Enqueue(*event)
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

// `ImmediateTriggerTo`
func (s *Mediator) ImmediateTriggerTo(e Kind) error {
	return nil
}

// `ImmediateTrigger`
func (s *Mediator) ImmediateTrigger(e Kind) error {
	return nil
}

// `TriggerTo`
func (s *Mediator) TriggerTo(e Kind) error {
	return nil
}
