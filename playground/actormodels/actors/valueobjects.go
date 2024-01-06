package actors

import (
	"time"
)

var (
	NoID EventID = "no-id" // represent the id of a none event
	// NoneID   KindID        = "none-id" // represent the id of a none kind
	InfoNone KindID = "none" // represent the signature of a none kind
)

// `EventClose` represent an event to quit the event system
type EventClose Event[struct{}]

// `EventSize` represent an event to resize a registry of event
type EventSize struct {
	Size int
}

// `KindID` represent the id of a kind, used with a registry of kind
// type KindID string

// `EventID` represent the id of an event which can be customized by user
type EventID string

// `EventCreated` represent the creation date of an event
type EventCreated time.Time

// `KindID` represent the signature of a kind
type KindID string

// `EventAnonymous` represent a given event by the user, it can be known or unknown.
// If it's `Kind` is unknown, then it will be analyzed and registred into the `RegistryKind`
type EventAnonymous interface{}
