package main

import (
	"context"
	"testing"
	"time"
)

func doNothing(event Event[interface{}]) {

}
func BenchmarkEventProcessing(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var err error
		var subscriber *Subscriber

		if subscriber, err = NewSubscriber(
			OptionSubscriberInterestedIn(
				Kind[Something](),
				Kind[Else](),
			),
		); err != nil {
			b.Fatal(err)
		}

		mediator, err := NewMediator(
			context.Background(),
			OptionMediatorSubscriber(subscriber),
			OptionMediatorSize(200),
		)
		if err != nil {
			b.Fatal(err)
		}

		// Start timer
		start := time.Now()

		mediator.Start()

		go func() {
			for j := 0; j < 200; j++ {
				mediator.Publish(Something{hello: "test"})
			}
			mediator.Trigger(Kind[EventClose]())
		}()

		for event := range subscriber.Channel {
			doNothing(event)
		}

		// Stop timer and log total duration
		duration := time.Since(start)
		b.Log("Total time to process 1M events:", duration)
	}
}
