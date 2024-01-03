
- event type to support streams of data to be picked up by subscriber, confirmation for each batch of data

some pseudo code

```go

type Stream[T any] struct {
 Data []T
}

type EventStream Event[Steam[T any]]

subscriber a -> prepare stream -> system allocate and accumulate -> new event stream
subscriber a -> send batch of data
subscriber b <- fetch batch data
subscriber b -> confirm batch -> data eliminated
subscriber b <- system end of stream 
-> system end event

```