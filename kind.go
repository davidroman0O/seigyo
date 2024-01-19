package seigyo

import (
	"fmt"
	"reflect"
	"time"
)

var (
	InfoNone KindID = "none" // represent the signature of a none kind
	// `KindNone` represent a kind that is none
	KindNone Kind = Kind{
		Kind: reflect.TypeOf(struct{}{}).Kind(),
		ID:   InfoNone,
	}
)

// `MsgClose` represent an Msg to quit the Msg system
type MsgClose Msg[struct{}]

// `MsgSize` represent an Msg to resize a registry of Msg
type MsgSize struct {
	Size int
}

// `MsgID` represent the id of an event which can be customized by user
type MsgID string

// `MsgCreated` represent the creation date of an event
type MsgCreated time.Time

// `Msg` represent the main data managed by a mediator and event registries
// which can hold a data that need to reach a `Subscriber`. Every `Event` will be
// dispatched through different means.
type Msg[T any] struct {
	ID        MsgID
	CreatedAt MsgCreated
	Kind      KindID
	Data      T
}

// `Kind` represent the signature of a given type by the user to track
// in which registry it belong to.
type Kind struct {
	ID       KindID
	Kind     reflect.Kind
	IsNone   bool
	TypeInfo interface{}
}

// `KindID` represent the signature of a kind
type KindID string

// `MsgAnonymous` represent a given event by the user, it can be known or unknown.
// If it's `Kind` is unknown, then it will be analyzed and registred into the `RegistryKind`
type MsgAnonymous interface{}

// `KindRegistry` represent a registry that track all given kind to avoid passing data around
// type KindRegistry *ThreadSafeMap[EventKindName, Kind]

// `registryKind` instance of a `KindRegistry`. There can be only one `KindRegistry` per `go` runtime.
// Those registry are meant to be used as the underlying grid that manage the communicate across different modules
// of an application.
var KindRegistry *ThreadSafeMap[KindID, Kind] = NewThreadSafeMap[KindID, Kind]()

// `newKind` create a new EventKind without ID
func newKind(e MsgAnonymous) Kind {
	kind := Kind{
		Kind:     reflect.TypeOf(e).Kind(),
		ID:       getNameKind(e),
		IsNone:   false,
		TypeInfo: e,
	}
	return kind
}

// `newKindWithName` create a new EventKind without ID when you know the name
func newKindWithName(e MsgAnonymous, name KindID) Kind {
	kind := Kind{
		Kind:     reflect.TypeOf(e).Kind(),
		ID:       name,
		IsNone:   false,
		TypeInfo: e,
	}
	return kind
}

func getKindID(fnType reflect.Type) KindID {
	return KindID(fmt.Sprintf("%v.%v", fnType.PkgPath(), fnType.Name()))
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
func hasKind(name KindID) bool {
	return KindRegistry.Has(name)
}

func getKind(name KindID) Kind {
	k, ok := KindRegistry.Get(name)
	if ok {
		return k
	}
	return KindNone
}

// `getNameKind` will generate a unique string based on the pkg path and the name of the type to know the origin of a type
func getNameKind(e MsgAnonymous) KindID {
	return KindID(fmt.Sprintf("%v.%v", reflect.ValueOf(e).Type().PkgPath(), reflect.ValueOf(e).Type().Name()))
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
