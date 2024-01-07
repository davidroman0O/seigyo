package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/signal"

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

	// should be the main api to build the actor-model stuff
	engine := actors.NewEngine(
	// should have options
	// middlewares
	// plugins
	// settings
	// gossip: for shared apps dags
	)

	// we will have to fill the system with actors
	// i know know why i will have to force myself to have a struct that implement an interface when anyway i want to restrict few things to gain more in performances down the road
	actor, err := engine.AddActor(
		actors.Type("something"),
		// if Remote is used, it should fine a `somethingelse` actor
		// on start, the system will check the definitions of the actor, get what it receive and all
		// actors.Remote("othernode@192.168.0.1:5001"),
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
	app1.Add(actor) // becomes a node in a dag, we should be able to define a whole graph of actors, even actors could dynamically create new actors at init or depending on messages so the dags need to be observed

	app2 := actors.NewApplication()
	app2.Add(actor) // becomes a node in a dag, we should be able to define a whole graph of actors, even actors could dynamically create new actors at init or depending on messages so the dags need to be observed

	// what about
	// then we should add applications to the system so it can analyze it and prepare then start

	// one engine, as much apps as needed
	// the implementation should reconciliate the two dags into one new dag that leverage all the actors as one
	cerr := actors.Start(engine, app1, app2)

	actor.Dispatch(Something{
		Msg: "test",
	})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	// close(cerr)
	select {
	case err := <-cerr:
		fmt.Println("done", err)
	case <-sigs:
		fmt.Println("Received Ctrl+C, stopping...")
		close(cerr)
		// Here you can add any cleanup code or stop your application
		actors.Stop() // trigger end of actors systems
		actors.Wait() // wait for finish
	}

	actors.Stop() // trigger end of actors systems

	actors.Wait() // wait for finish
}
