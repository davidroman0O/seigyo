package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/davidroman0O/seigyo"
	"github.com/k0kubun/pp/v3"
)

/// With the different test i did on actor-model

/// TODO: ok we have tell pattern now but what about ask pattern?

type ActorContext struct {
	context.Context
}

func (c *ActorContext) Behave(be Behavior) {

}

type behaviorReceive struct {
	Kind seigyo.Kind
	Call interface{}
}

func Receive[T any](receive func(ctx ActorContext, msg seigyo.Msg[T])) Behavior {
	return behaviorReceive{
		Kind: seigyo.AsKind[T](),
		Call: receive,
	}
}

type behaviorTell struct {
	Kind seigyo.Kind
	Msg  seigyo.MsgAnonymous
}

// TODO: make TellWithName or something
func Tell[T any](msg seigyo.MsgAnonymous) Behavior {
	return behaviorTell{
		Kind: seigyo.AsKind[T](),
		Msg:  msg,
	}
}

type behaviorForward struct {
	Kind seigyo.Kind
	Msg  seigyo.MsgAnonymous
}

func Forward[T any](msg seigyo.MsgAnonymous) Behavior {
	return behaviorForward{
		Kind: seigyo.AsKind[T](),
		Msg:  msg,
	}
}

type behaviorAmI struct {
	name string
}

func AmI(name string) Behavior {
	return behaviorAmI{
		name: name,
	}
}

type behaviorArg[T any] struct {
	data T
}

func Arg[T any](data T) Behavior {
	return behaviorArg[T]{
		data: data,
	}
}

type Behavior any

// function func(ctx ActorContext, msg TypeMessage) error
type ActorReceiver any

type ActorDef[T any] struct {
	KindID    seigyo.KindID
	Kind      seigyo.Kind
	Receivers map[seigyo.KindID]ActorReceiver
}

// TODO: maybe an actor should register itself
func Actor[T any](bebe ...Behavior) ActorDef[T] {
	kind := seigyo.AsKind[T]()
	actor := ActorDef[T]{
		KindID:    kind.ID,
		Kind:      kind,
		Receivers: map[seigyo.KindID]ActorReceiver{},
	}
	//	just reflect to get the behavior back
	defs := []Behavior{}
	defs = append(defs, bebe...)

	return actor
}

type ActorDoSomeConfig struct{}
type ActorDoSome struct{}
type ActorExit struct{}
type MsgDo struct{}
type MsgSome struct{}

type actUnit struct {
	unit int
}

func Unit(unit int) Act {
	return actUnit{
		unit: unit,
	}
}

type actPort struct {
	port int
}

func Port(port int) Act {
	return actPort{
		port: port,
	}
}

type Act any

func Spawn[T any](actor ActorDef[T], acts ...Act) {

}

func main() {

	// actor should be defined, and then actors should be spawn with settings
	// it should be simple to define actors, behaviors and config
	// we should be able to have a basic api of actors in few lines

	Spawn(
		Actor[ActorDoSome](
			// assign info
			AmI("actorA"),
			// assign args as props
			Arg[ActorDoSomeConfig](ActorDoSomeConfig{}),
			// behavior receive
			// Receive[MsgDo]( // interested topic
			// 	func(ctx ActorContext, msg seigyo.Msg[MsgDo]) {
			// 		fmt.Println(msg.Data)
			// 		// push new msg to another actor
			// 		// todo: add more parameters for specific opts
			// 		ctx.Behave(Tell[ActorDoSome](MsgSome{}))
			// 	}),
			// Receive[MsgSome]( // interested topic
			// 	func(ctx ActorContext, msg seigyo.Msg[MsgSome]) {
			// 		fmt.Println(msg.Data)
			// 		//	ask to forward that behavior
			// 		ctx.Behave(Forward[ActorExit](msg))
			// 	}),
		),
		Unit(5),
		Port(3001),
	)

	Spawn(
		Actor[ActorExit](
			AmI("actorB"),
			Receive[MsgSome](func(ctx ActorContext, msg seigyo.Msg[MsgSome]) {
				fmt.Println("received")
			}),
		),
		Unit(1),
		Port(3002),
	)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	// fmt.Println(memStats)
	pp.Println(memStats)

}
