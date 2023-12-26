package events

import (
	"fmt"
	"reflect"
)

type registry map[EventKindName]EventKind

var registryKind registry = registry{}

type EventAnonymous interface{}

// `newKind` create a new EventKind without ID
func newKind(e EventAnonymous) EventKind {
	kind := EventKind{
		Kind:   reflect.TypeOf(e).Kind(),
		Info:   getNameKind(e),
		Data:   e,
		IsNone: false,
	}
	return kind
}

// `newKindWithName` create a new EventKind without ID when you know the name
func newKindWithName(e EventAnonymous, name EventKindName) EventKind {
	kind := EventKind{
		Kind:   reflect.TypeOf(e).Kind(),
		Info:   name,
		Data:   e,
		IsNone: false,
	}
	return kind
}

func hasKind(name EventKindName) bool {
	_, ok := registryKind[name]
	return ok
}

func getKind(name EventKindName) EventKind {
	k, ok := registryKind[name]
	if ok {
		return k
	}
	return KindNone
}

// `getNameKind` will generate a unique string based on the pkg path and the name of the type to know the origin of a type
func getNameKind(e EventAnonymous) EventKindName {
	return EventKindName(fmt.Sprintf("%v.%v", reflect.ValueOf(e).Type().PkgPath(), reflect.ValueOf(e).Type().Name()))
}

func storeKind(kind EventKind) {
	registryKind[kind.Info] = kind
}

// `Kind` is helping the mediator to know what is the mapping of your event
func Kind[T any]() EventKind {
	proxy := *new(T)
	name := getNameKind(proxy)
	if !hasKind(name) {
		kind := newKindWithName(proxy, name)
		storeKind(kind)
		return kind
	}
	return getKind(name)
}
