package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func doNothing(event Event[interface{}]) {

}

// go test -benchmem -run=^$ -bench ^BenchmarkEventProcessing$ .\playground\
func BenchmarkEventProcessing(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var err error
		var subscriber *Subscriber
		fmt.Println("Starting")

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
			OptionMediatorSize(1000001),
		)
		if err != nil {
			b.Fatal(err)
		}

		// Start timer
		start := time.Now()

		mediator.Start()

		// Create an error channel
		errChan := make(chan error, 1)

		go func() {
			fmt.Println("Starting to publish events")
			defer close(errChan) // Close the channel to indicate no errors
			var ierr error
			time.Sleep(time.Millisecond * 20)
			for j := 0; j < 1000000; j++ {
				if ierr = mediator.Publish(Something{hello: "test"}); ierr != nil {
					errChan <- ierr // Send the error to the main goroutine
					return
				}
			}
			fmt.Println("Finished publishing events")

			if ierr = mediator.Trigger(Kind[EventClose]()); ierr != nil {
				errChan <- ierr
				return
			}
		}()
		// Check for errors from the goroutine
		if ierr, ok := <-errChan; ok {
			b.Fatal(ierr) // If there's an error, report it
		}

		fmt.Println("Subscriber listening")
		for event := range subscriber.Channel {
			doNothing(event)
		}

		fmt.Println("ending")
		// Stop timer and log total duration
		duration := time.Since(start)
		b.Log("Total time to process 1M events:", duration)
	}
}
