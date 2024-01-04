package main

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/davidroman0O/seigyo/playground/eventsactormodel/events"
)

type Something struct {
	Msg string
}

type ActorSomethingState struct {
	Count int
}

func main() {

	now := time.Now()
	// create a new actor
	// also new actor is a new vertice
	a, err := events.NewActor(
		// name directly
		events.ActorAddressName("something"),
		// customized id
		events.ActorIDFactory(getID),
		// internal state
		events.ActorState[ActorSomethingState](
			func(ctx events.ActorContext) ActorSomethingState {
				return ActorSomethingState{
					Count: 0,
				}
			}),
		// should create an inbox for Something while pre-interest the actor to Something
		events.NewReceiver[Something](
			func(ctx events.ActorContext, message Something) error {
				fmt.Printf("received something %v \n", message.Msg)
				return nil
			},
		),
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("actor: ", a)

	// a.Call(Something{
	// 	Msg: "hello world",
	// })

	fmt.Println(time.Since(now))

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
