package events

import "sync"

type SubscriberID string

type Subscriber struct {
	ID           SubscriberID
	InterestedIn map[EventKindID]Kind
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

func OptionSubscriberInterestedIn(es ...Kind) OptionSubscriber {
	return func(s *Subscriber) error {
		for i := 0; i < len(es); i++ {
			s.InterestedIn[es[i].ID] = es[i]
		}
		return nil
	}
}

func NewSubscriber(opts ...OptionSubscriber) (*Subscriber, error) {
	sub := Subscriber{
		InterestedIn: make(map[EventKindID]Kind),
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
func (s *Subscriber) Register(kind Kind) (EventKindID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.InterestedIn[kind.ID] = kind
	return kind.ID, nil
}
