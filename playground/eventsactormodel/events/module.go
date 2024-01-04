package events

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	idle int32 = iota
	alive
	stopping
	stopped
	restarting
)

type ActorContext struct {
	name ActorAddressName
}

// broadcast
func (c *ActorContext) Publish(e EventAnonymous) {

}

type ActorEntity struct {
	state int32

	id        ActorID
	context   ActorContext
	inbox     *CircularBuffer[reflect.Value]
	receivers map[KindID]reflect.Value
	children  []*ActorEntity
}

func (a *ActorEntity) Call(e EventAnonymous) error {
	kindID := getNameKind(e)
	var ok bool
	var fn interface{}
	if fn, ok = a.receivers[kindID]; !ok {
		return fmt.Errorf("actor is not interested in that kind")
	}
	// Get the reflect.Value of the function
	funcValue := reflect.ValueOf(fn)
	// Call the function dynamically
	results := funcValue.Call([]reflect.Value{
		reflect.ValueOf(a.context),
		reflect.ValueOf(e),
	})
	if !results[0].IsNil() {
		return results[0].Interface().(error)
	}
	return nil
}

type ActorID string

type ActorAddressName string

type ActorState[T any] func(ctx ActorContext) T

type ActorReceive[T any] func(ctx ActorContext, message T) error

type ActorIDFactory func() (string, error)

// `ActorReceiver` contain the function receiver and the kind of type we must push there
type ActorReceiver[T any] struct {
	Kind     Kind
	Receiver ActorReceive[T]
}

func NewReceiver[T any](fn ActorReceive[T]) ActorReceiver[T] {
	return ActorReceiver[T]{
		Kind:     newKindTyped[T](),
		Receiver: fn,
	}
}

func NewActor(fn ...any) (*ActorEntity, error) {
	ctx := ActorContext{}
	entity := ActorEntity{
		state:     idle,
		context:   ctx,
		receivers: map[KindID]reflect.Value{},
	}
	for _, v := range fn {
		switch fn := v.(type) {
		case ActorAddressName:
			ctx.name = fn
		default:
			fullName := reflect.TypeOf(v).Name()
			switch reflect.TypeOf(v).Kind() {
			case reflect.Struct:
				if strings.Contains(fullName, "ActorReceiver") {
					kind := reflect.ValueOf(v).FieldByName("Kind").Interface().(Kind)
					fn := reflect.ValueOf(v).FieldByName("Receiver")
					fnType := fn.Type()

					if fnType.In(0).Name() != "ActorContext" {
						return nil, fmt.Errorf("first paramter should be ActorContext")
					}

					kindName := getKindID(fnType.In(1))

					if getKindID(fnType.In(1)) != kind.ID {
						fmt.Println(kindName, " == ", kind)
						return nil, fmt.Errorf("first kind")
					}

					entity.receivers[kind.ID] = fn
				}
				break
			case reflect.Func:
				if fullName == "ActorIDFactory" {
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
					entity.id = ActorID(eventualID)
				}
				break
			}
		}
	}
	return &entity, nil
}

// we're next time
// now we need to do the fun thing
type ActorNamespace struct {
}

// represent a goroutine that will take in charge multiple actors
type ActorwWorker struct {
}
