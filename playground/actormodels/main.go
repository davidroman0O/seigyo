package main

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/davidroman0O/seigyo/playground/actormodels/actors"
)

/// I want to be able to both prototype any kind of app and be able to grow into a real one with the same tool
/// I want to be able to express behavior naturally through messages and state mutations, the rest should be controlled by the tool

// `userDefinedID` its an internal function that will to have IDs locally but I highly encourage you to come with your own IDs
func userDefinedID() (string, error) {
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

type StateApp1 struct {
	Count int
}

type Something struct {
	Msg string
}

var kindSomething = actors.ActorKind("something")

func App1ReceiveSomething(ctx actors.Context, message Something) error {
	fmt.Printf("received something %v \n", message.Msg)
	return nil
}

type Else struct {
	Msg string
}

func App1ReceiveElse(ctx actors.Context, message Else) error {
	fmt.Printf("received else %v \n", message.Msg)
	return nil
}

func main() {

	now := time.Now()

	// should be the main api to build the actor-model stuff
	system := actors.NewSystem(
	// should have options
	// middlewares
	// plugins
	// settings
	)

	// we will have to fill the system with actors
	// i know know why i will have to force myself to have a struct that implement an interface when anyway i want to restrict few things to gain more in performances down the road
	actor, err := system.AddActor(
		kindSomething,
		// customized id
		actors.IDFactory(userDefinedID),
		// should create an inbox for Something while pre-interest the actor to Something
		actors.NewReceiver(App1ReceiveSomething),
		actors.NewReceiver(App1ReceiveElse),
	)

	if err != nil {
		panic(err)
	}

	fmt.Println("actor: ", actor)

	// then you can create application by making your tree or graph of actors
	// later, we should be able to reconciliate a dag file into a dag
	app1 := actors.NewApplication()
	app1.Add(actor) // becomes a node in a dag

	app2 := actors.NewApplication()
	app2.Add(actor) // becomes a node in a dag

	// what about
	// then we should add applications to the system so it can analyze it and prepare then start

	fmt.Println(time.Since(now))
}
