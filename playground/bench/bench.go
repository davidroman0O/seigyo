package main

import (
	"context"
	"fmt"

	events "github.com/davidroman0O/seigyo/playground"
)

type Something struct {
	hello string
}

type Else struct {
	ok string
}

func simpleBenchmark() error {

	var mediator *events.Mediator
	var err error

	var subscriber *events.Subscriber

	if subscriber, err = events.NewSubscriber(
		events.OptionSubscriberInterestedIn(
			events.Kind[Something](),
			events.Kind[Else](),
		),
	); err != nil {
		return err
	}

	if mediator, err = events.NewMediator(
		context.Background(),
		events.OptionMediatorSubscriber(subscriber),
		events.OptionMediatorQueuedSize(5),
	); err != nil {
		return err
	}

	mediator.Start()

	cerr := make(chan error)

	go func() {
		for j := 0; j < 4; j++ {
			if err := mediator.Publish(Something{hello: "test"}); err != nil {
				fmt.Println("panic ", err)
				cerr <- err
				return
			}
		}
		if err := mediator.Publish(events.EventSize{
			Size: 7,
		}); err != nil {
			cerr <- err
			return
		}
		if err := mediator.Trigger(events.Kind[events.EventClose]()); err != nil {
			cerr <- err
			return
		}
		cerr <- nil
	}()

	// try to empty the channel
	for _ = range subscriber.Channel {
	}

	select {
	case e := <-cerr:
		if e != nil {
			return e
		}
	}

	return nil
}

func main() {
	if err := simpleBenchmark(); err != nil {
		panic(err)
	}
}
