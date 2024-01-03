package events

import (
	"fmt"
	"reflect"
)

// `KindRegistry` represent a registry that track all given kind to avoid passing data around
// type KindRegistry *ThreadSafeMap[EventKindName, Kind]

// `registryKind` instance of a `KindRegistry`. There can be only one `KindRegistry` per `go` runtime.
// Those registry are meant to be used as the underlying grid that manage the communicate across different modules
// of an application.
var KindRegistry *ThreadSafeMap[EventKindID, Kind] = NewThreadSafeMap[EventKindID, Kind]()

// `newKind` create a new EventKind without ID
func newKind(e EventAnonymous) Kind {
	kind := Kind{
		Kind:     reflect.TypeOf(e).Kind(),
		ID:       getNameKind(e),
		IsNone:   false,
		TypeInfo: e,
	}
	return kind
}

// `newKindWithName` create a new EventKind without ID when you know the name
func newKindWithName(e EventAnonymous, name EventKindID) Kind {
	kind := Kind{
		Kind:     reflect.TypeOf(e).Kind(),
		ID:       name,
		IsNone:   false,
		TypeInfo: e,
	}
	return kind
}

// `newKindTyped`
func newKindTyped[T any]() Kind {
	e := *new(T)
	kind := Kind{
		Kind:     reflect.TypeOf(e).Kind(),
		ID:       getNameKind(e),
		IsNone:   false,
		TypeInfo: e,
	}
	return kind
}

// `hasKind` verify if the registry knowns a kind name
func hasKind(name EventKindID) bool {
	return KindRegistry.Has(name)
}

func getKind(name EventKindID) Kind {
	k, ok := KindRegistry.Get(name)
	if ok {
		return k
	}
	return KindNone
}

// `getNameKind` will generate a unique string based on the pkg path and the name of the type to know the origin of a type
func getNameKind(e EventAnonymous) EventKindID {
	return EventKindID(fmt.Sprintf("%v.%v", reflect.ValueOf(e).Type().PkgPath(), reflect.ValueOf(e).Type().Name()))
}

func storeKind(kind Kind) {
	KindRegistry.Set(kind.ID, kind)
}

// `AsKind` is helping the mediator to know what is the mapping of your event
func AsKind[T any]() Kind {
	proxy := *new(T)
	name := getNameKind(proxy)
	if !hasKind(name) {
		kind := newKindWithName(proxy, name)
		storeKind(kind)
		return kind
	}
	return getKind(name)
}
