package events

import (
	"fmt"
	"reflect"
)

type ActorContext struct {
	name ActorAddressName
}

type ActorAddressName string

type ActorName func() ActorAddressName
type ActorState[T any] func(ctx ActorContext) T

type ActorReceive[T any] func(ctx ActorContext, message T) error

// type ActorKind[T any] func() Kind

type ActorReceiver[T any] struct {
	kind    Kind
	receive ActorReceive[T]
}

func NewReceiver[T any](fn ActorReceive[T]) ActorReceiver[T] {
	return ActorReceiver[T]{
		kind:    newKindTyped[T](),
		receive: fn,
	}
}

// func ActorReceive[T any](fn ActorReceiver[T]) (reflect.Type, ActorReceiver[T]) {
// 	empty := *new(T)
// 	return reflect.TypeOf(empty), fn
// }

type ActorAny interface{}

func NewActor(fn ...any) ActorAny {
	ctx := ActorContext{}
	for _, v := range fn {
		switch fn := v.(type) {
		case ActorName:
			ctx.name = fn()
		default:
			// name := reflect.TypeOf(v).Name()
			switch reflect.TypeOf(v).Kind() {
			case reflect.Struct:
				fmt.Println("struct", reflect.TypeOf(v).Name())
				fmt.Println(reflect.ValueOf(v).FieldByName("kind"))
				break
			case reflect.Func:
				fmt.Println("func", reflect.TypeOf(v).Name())
				break
			}
		}
	}
	return ctx
}
