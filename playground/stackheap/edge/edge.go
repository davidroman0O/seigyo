package edge

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/davidroman0O/seigyo/playground/stackheap/buffer"
)

/// I'm not sure how i'm going to use those two but we will see
/// So far, `go` love the Accumulator, 100m+ msgs per seconds

// Used to connect processes
type Edge[T any] struct {
	// context.Context
	buffer *buffer.HierarchicalBuffer[T]
	in     chan []T
	out    chan []T
}

func NewEdge[T any]() *Edge[T] {
	e := &Edge[T]{
		// Context: context.Background(),
		buffer: buffer.NewHierarchicalBuffer[T](1024, 4),
		in:     make(chan []T),
		out:    make(chan []T),
	}
	go func() {
		for {
			e.buffer.PushN(<-e.in)
			runtime.Gosched()
			item, err := e.buffer.PopN(1024)
			if err != nil {
				fmt.Println(err)
				continue
			}
			e.out <- item
		}
	}()
	return e
}

// Used to sink messages from
type Accumulator[T any] struct {
	context.Context
	buffer *buffer.HierarchicalBuffer[T]
	in     chan []T
	out    chan []T
	total  int32
}

func NewAccumulator[T any]() *Accumulator[T] {
	e := &Accumulator[T]{
		Context: context.Background(),
		buffer:  buffer.NewHierarchicalBuffer[T](1024, 4),
		in:      make(chan []T),
		out:     make(chan []T),
	}
	// from what i've read and understood, that 1us of delay to accumulate data and then goshed make it super fast cause the other goroutine can have time to push
	go func() {
		tickerPush := time.NewTicker(1 * time.Microsecond)
		tickerPop := time.NewTicker(1 * time.Microsecond)
		// note: i could just use accumulator and not edge, simply because i could accumulate data to be processed in batches
		// big batches > (items or small arrays)
		// also when the buffer is not big, it could reduce the ticker or give more control to the dev
		// tickerPop.Reset()
		defer tickerPop.Stop()
		for {
			select {
			case <-tickerPush.C:
				data := <-e.in
				atomic.AddInt32(&e.total, int32(len(data)))
				e.buffer.PushN(data)
				runtime.Gosched()
			case <-tickerPop.C:
				if atomic.LoadInt32(&e.total) == 0 {
					runtime.Gosched()
					continue
				}
				items, err := e.buffer.PopN(1024 * 2)
				if err != nil {
					fmt.Println(err)
					continue
				}
				atomic.StoreInt32(&e.total, atomic.LoadInt32(&e.total)-int32(len(items)))
				e.out <- items
				runtime.Gosched()
			case <-e.Context.Done():
				return // Exit the function when context is cancelled
			}
		}
	}()
	return e
}
