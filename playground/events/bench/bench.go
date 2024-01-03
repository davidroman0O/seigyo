package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	events "github.com/davidroman0O/seigyo/playground"
)

type Something struct {
	hello string
}

type Else struct {
	ok string
}

func simpleBenchmark() error {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var mediator *events.Mediator
	var err error

	var subscriber *events.Subscriber

	if subscriber, err = events.NewSubscriber(
		events.OptionSubscriberInterestedIn(
			events.AsKind[Something](),
			events.AsKind[Else](),
		),
	); err != nil {
		return err
	}

	if mediator, err = events.NewMediator(
		context.Background(),
		events.OptionMediatorSubscriber(subscriber),
		// TODO @droman: add more options
	); err != nil {
		return err
	}

	mediator.Start()

	now := time.Now()
	cerr := make(chan error)

	go func() {
		defer func() {
			close(cerr)
		}()
		for j := 0; j < 1000000; j++ {
			if err := mediator.
				Publish(
					Something{
						hello: "test",
					},
				); err != nil {
				fmt.Println("panic ", err)
				cerr <- err
				return
			}
		}
		// if err := mediator.Publish(events.EventSize{
		// 	Size: 20,
		// }); err != nil {
		// 	cerr <- err
		// 	return
		// }
		fmt.Println("trigger close")
		if err := mediator.Trigger(events.AsKind[events.EventClose]()); err != nil {
			cerr <- err
			return
		}
		fmt.Println("triggered")
		cerr <- nil
	}()

	counter := 0

	// defer func() {
	// 	fmt.Println("count ", counter)
	// }()

	// try to empty the channel
	// for e := range subscriber.Channel {
	// 	fmt.Println(e)
	// }

	for e := range subscriber.Channel {
		switch e.Data.(type) {
		case Something:
			counter++
		}
	}

	select {
	case e := <-cerr:
		if e != nil {
			return e
		}
	}

	fmt.Println("time ", time.Since(now))
	fmt.Println("count ", counter)

	return nil
}

func main() {
	if err := simpleBenchmark(); err != nil {
		panic(err)
	}
}
