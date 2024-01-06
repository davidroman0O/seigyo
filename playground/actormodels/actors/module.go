package actors

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/davidroman0O/seigyo/playground/actormodels/actors/dag"
)

/// My ideal actor-model is a platform that can host more than one application and have it's own internal systems with middleware capabilities (state management, metrics, etc) with plugins for extended features (grpc, http, tcp, websocket, etc)
/// If that machine can process 30M events without any middleware or plugins, then i will have room to add them
/// Actors won't have their own thread but we will instanciate goroutines when we can and need it to use cpu resource as much as possible
/// Multiple applications should be hosted by this implementation, i don't want to have baremetal hardware that sit down nothing for one app while it could be load balanced for multiple applications

const (
	idle int32 = iota
	alive
	stopping
	stopped
	restarting
)

type Context struct {
	name AddressName
}

// broadcast
func (c *Context) Publish(e EventAnonymous) {

}

type Envelope struct {
	ID    KindID
	Value reflect.Value
}

type Actor struct {
	state     int32
	id        ID
	kind      ActorKind
	context   Context
	inbox     *CircularBuffer[Envelope]
	receivers map[KindID]reflect.Value
	children  []*Actor
}

// should dequeue `n` messages from it's inbox
func (a *Actor) Process() []error {
	msgs, _, err := a.inbox.DequeueN(300)
	if err != nil {
		return []error{err}
	}
	for i := 0; i < len(msgs); i++ {
		a.invokeMsg(msgs[i])
	}
	return nil
}

// temporary function just to test the reflection value invokeMsg
func (a *Actor) invokeMsg(e Envelope) error {
	var ok bool
	var fn reflect.Value
	if fn, ok = a.receivers[e.ID]; !ok {
		return fmt.Errorf("actor is not interested in that kind")
	}
	// Call the function dynamically
	results := fn.Call([]reflect.Value{
		reflect.ValueOf(a.context),
		reflect.ValueOf(e.Value),
	})
	if !results[0].IsNil() {
		return results[0].Interface().(error)
	}
	return nil
}

func (a *Actor) dispatch(e EventAnonymous) error {
	kind := getNameKind(e)
	var ok bool
	if _, ok = a.receivers[kind]; !ok {
		return fmt.Errorf("actor is not interested in that kind of message")
	}
	a.inbox.Enqueue(Envelope{
		ID:    kind,
		Value: reflect.ValueOf(e),
	})
	return nil
}

type ID string

type AddressName string

// type Reducer[T any] func(ctx Context) (T, func(message T, state T) T)

type State[T any] func(ctx Context) T

type Receive[T any] func(ctx Context, message T) error

type IDFactory func() (string, error)

// `receiver` contain the function receiver and the kind of type we must push there
type receiver[T any] struct {
	Kind     Kind
	Receiver Receive[T]
}

func NewReceiver[T any](fn Receive[T]) receiver[T] {
	return receiver[T]{
		Kind:     newKindTyped[T](),
		Receiver: fn,
	}
}

// 1 application == 1 dag of actor-models
type Application struct {
	graph dag.AcyclicGraph
	nodes map[ID]ActorNode
}

func NewApplication() *Application {
	return &Application{
		graph: dag.AcyclicGraph{},
		nodes: map[ID]ActorNode{},
	}
}

func (a *Application) Add(actor *Actor) {
	node := ActorNode{
		actor:  actor,
		ID:     actor.id,
		vertex: a.graph.Add(a),
	}
	a.nodes[node.ID] = node
}

// representation on the dag
type ActorNode struct {
	ID     ID
	actor  *Actor
	vertex dag.Vertex
}

// Core component of the runtime
// i don't like the name
type System struct {
	actors map[ID]Actor
}

func NewSystem() *System {
	return &System{
		actors: map[ID]Actor{},
	}
}

type ActorKind string

// TODO @droman: does actors can have multiple receivers by type of message?!
func (a *System) AddActor(fn ...any) (*Actor, error) {
	ctx := Context{}
	actor := Actor{
		state:     idle,
		context:   ctx,
		receivers: map[KindID]reflect.Value{},
	}
	// I'm testing this approach to have both dependency injection for parameters but also flexibility depending of the type of function or data i want to set
	// so far i like it from the usage perspective since it gives the illusion of dependency injection without reaaaallllyyy doing it the "golang community approved" way
	for _, v := range fn {
		switch fn := v.(type) {
		case ActorKind:
			actor.kind = fn
		case AddressName:
			ctx.name = fn
		default:
			fullName := reflect.TypeOf(v).Name()
			switch reflect.TypeOf(v).Kind() {
			case reflect.Struct:
				if strings.Contains(fullName, "receiver") {
					kind := reflect.ValueOf(v).FieldByName("Kind").Interface().(Kind)
					fn := reflect.ValueOf(v).FieldByName("Receiver")
					fnType := fn.Type()

					if fnType.In(0).Name() != "Context" {
						return nil, fmt.Errorf("first paramter should be ActorContext")
					}

					kindName := getKindID(fnType.In(1))

					if getKindID(fnType.In(1)) != kind.ID {
						fmt.Println(kindName, " == ", kind)
						return nil, fmt.Errorf("first kind")
					}

					actor.receivers[kind.ID] = fn
				}
				break
			case reflect.Func:
				if fullName == "IDFactory" {
					// Get the reflect.Value of the function
					funcValue := reflect.ValueOf(fn)
					// Prepare the arguments
					args := []reflect.Value{}

					results := funcValue.Call(args)

					eventualError := results[1]
					if !eventualError.IsNil() {
						return nil, eventualError.Interface().(error)
					}

					eventualID := results[0].Interface().(string)
					actor.id = ID(eventualID)
				}
				break
			}
		}
	}
	if actor.kind == "" {
		return nil, fmt.Errorf("should have a kind")
	}
	a.actors[actor.id] = actor
	return &actor, nil
}
