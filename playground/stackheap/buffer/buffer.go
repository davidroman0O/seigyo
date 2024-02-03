package buffer

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type Buffer[T any] struct {
	items   []T
	head    int
	tail    int
	full    bool
	maxSize int
	mu      sync.Mutex
}

func (b *Buffer[T]) PopN(n int) ([]T, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.full && b.head == b.tail {
		return nil, errors.New("buffer is empty") // No items to pop
	}

	var items []T
	for i := 0; i < n && (b.full || b.head != b.tail); i++ {
		item := b.items[b.head]
		b.head = (b.head + 1) % b.maxSize
		b.full = false
		items = append(items, item)
	}

	return items, nil
}

func NewBuffer[T any](size int) *Buffer[T] {
	return &Buffer[T]{
		items:   make([]T, size),
		maxSize: size,
	}
}

func (b *Buffer[T]) IsFull() bool {
	return b.full
}

func (b *Buffer[T]) PushN(items []T) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	count := 0
	for _, item := range items {
		if b.full {
			break
		}
		b.items[b.tail] = item
		b.tail = (b.tail + 1) % b.maxSize
		count++
		if b.tail == b.head {
			b.full = true
		}
	}

	if count == 0 {
		return 0, errors.New("buffer is full")
	}

	return count, nil
}

func (b *Buffer[T]) Push(item T) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.full {
		return errors.New("buffer is full")
	}

	b.items[b.tail] = item
	b.tail = (b.tail + 1) % b.maxSize
	if b.tail == b.head {
		b.full = true
	}

	return nil
}

func (b *Buffer[T]) Pop() (T, error) {
	var zero T

	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.full && b.head == b.tail {
		return zero, errors.New("buffer is empty")
	}

	item := b.items[b.head]
	b.head = (b.head + 1) % b.maxSize
	b.full = false

	return item, nil
}

// Note: I have excellent performances with a hierarchial buffer
type HierarchicalBuffer[T any] struct {
	buffers   []*Buffer[T]
	maxSize   int
	readIndex int
	sync.Mutex
}

func NewHierarchicalBuffer[T any](bufferSize, initialBuffers int) *HierarchicalBuffer[T] {
	hBuffer := &HierarchicalBuffer[T]{
		buffers: make([]*Buffer[T], initialBuffers),
		maxSize: bufferSize,
	}
	for i := 0; i < initialBuffers; i++ {
		hBuffer.buffers[i] = NewBuffer[T](bufferSize)
	}
	return hBuffer
}

func (hb *HierarchicalBuffer[T]) Push(item T) error {
	hb.Lock()
	defer hb.Unlock()

	for _, buffer := range hb.buffers {
		if !buffer.IsFull() {
			return buffer.Push(item)
		}
	}

	// All buffers are full, create a new one
	newBuffer := NewBuffer[T](hb.maxSize)
	hb.buffers = append(hb.buffers, newBuffer)
	return newBuffer.Push(item)
}

func (hb *HierarchicalBuffer[T]) Pop() (T, error) {
	var zero T

	hb.Lock()
	defer hb.Unlock()

	start := hb.readIndex
	for {
		buffer := hb.buffers[hb.readIndex]
		item, err := buffer.Pop()
		if err == nil {
			return item, nil
		}

		// Move to next buffer
		hb.readIndex = (hb.readIndex + 1) % len(hb.buffers)

		// If we have checked all buffers, return error
		if hb.readIndex == start {
			return zero, errors.New("all buffers are empty")
		}
	}
}

func (hb *HierarchicalBuffer[T]) HasMessages() bool {
	hb.Lock()
	defer hb.Unlock()

	for _, buffer := range hb.buffers {
		if buffer.head != buffer.tail || buffer.full {
			return true
		}
	}
	return false
}

func (hb *HierarchicalBuffer[T]) FullEmptyBufferCount() (fullCount, emptyCount int) {
	hb.Lock()
	defer hb.Unlock()

	for _, buffer := range hb.buffers {
		if buffer.full {
			fullCount++
		} else if buffer.head == buffer.tail {
			emptyCount++
		}
	}
	return
}

func (hb *HierarchicalBuffer[T]) BufferCount() int {
	hb.Lock()
	defer hb.Unlock()

	return len(hb.buffers)
}

var ErrAllBuffersEmpty = errors.New("all buffers are empty")

func (hb *HierarchicalBuffer[T]) PopN(n int) ([]T, error) {
	hb.Lock()
	defer hb.Unlock()

	var items []T
	totalNeeded := n
	start := hb.readIndex
	didCycle := false

	for totalNeeded > 0 {
		buffer := hb.buffers[hb.readIndex]
		popped, err := buffer.PopN(totalNeeded)
		if err == nil {
			items = append(items, popped...)
			totalNeeded -= len(popped)
		}

		hb.readIndex = (hb.readIndex + 1) % len(hb.buffers)

		if hb.readIndex == start {
			if didCycle {
				break // Break if we've already cycled through all buffers once
			}
			didCycle = true
		}
	}

	if len(items) == 0 {
		return nil, ErrAllBuffersEmpty
	}

	return items, nil
}

func (hb *HierarchicalBuffer[T]) PushN(items []T) error {
	hb.Lock()
	defer hb.Unlock()

	for len(items) > 0 {
		for _, buffer := range hb.buffers {
			if !buffer.IsFull() {
				pushed, err := buffer.PushN(items)
				if err != nil {
					return err
				}
				items = items[pushed:]
				if len(items) == 0 {
					return nil
				}
				break
			}
		}

		// All buffers are full, create a new one
		newBuffer := NewBuffer[T](hb.maxSize)
		hb.buffers = append(hb.buffers, newBuffer)
	}

	return nil
}

// Producer and Consumer data structure to simplify the storage and process of messages
type ProducerConsumer[T any] struct {
	context.Context
	buffer   *HierarchicalBuffer[T]
	producer chan []T
	consumer chan []T
	total    int32
}

func NewProducerConsumer[T any](size, buffers, pop int) *ProducerConsumer[T] {
	e := &ProducerConsumer[T]{
		Context:  context.Background(),
		buffer:   NewHierarchicalBuffer[T](size, buffers),
		producer: make(chan []T, size),
		consumer: make(chan []T, size),
	}
	// from what i've read and understood, that 1us of delay to accumulate data and then goshed make it super fast cause the other goroutine can have time to push
	go func() {
		tickerPush := time.NewTicker(1 * time.Nanosecond)
		tickerPop := time.NewTicker(1 * time.Nanosecond)
		// note: i could just use accumulator and not edge, simply because i could accumulate data to be processed in batches
		// big batches > (items or small arrays)
		// also when the buffer is not big, it could reduce the ticker or give more control to the dev
		// tickerPop.Reset()
		defer tickerPop.Stop()
		for {
			select {
			case <-tickerPush.C:
				data := <-e.producer
				atomic.AddInt32(&e.total, int32(len(data)))
				if err := e.buffer.PushN(data); err != nil {
					fmt.Println(err)
				}
				runtime.Gosched()
			case <-tickerPop.C:
				if atomic.LoadInt32(&e.total) == 0 {
					runtime.Gosched()
					continue
				}
				items, err := e.buffer.PopN(pop)
				if err != nil {
					fmt.Println(err)
					continue
				}
				atomic.StoreInt32(&e.total, atomic.LoadInt32(&e.total)-int32(len(items)))
				e.consumer <- items
				runtime.Gosched()
			case <-e.Context.Done():
				fmt.Println("done")
				return // Exit the function when context is cancelled
			}
		}
	}()
	return e
}

// func (b *Buffer[T]) PopN(n int) ([]T, error) {
// 	b.mu.Lock()
// 	defer b.mu.Unlock()

// 	var items []T
// 	for i := 0; i < n; i++ {
// 		if !b.full && b.head == b.tail {
// 			// Stop if the buffer is empty
// 			break
// 		}
// 		item := b.items[b.head]
// 		b.items[b.head] = *new(T) // Zero the item (optional, for garbage collection)
// 		b.head = (b.head + 1) % b.maxSize
// 		b.full = false
// 		items = append(items, item)
// 	}

// 	if len(items) == 0 {
// 		return nil, errors.New("buffer is empty")
// 	}

//		return items, nil
//	}
// func (b *Buffer[T]) PopN(n int) ([]T, error) {
// 	b.mu.Lock()
// 	defer b.mu.Unlock()

// 	if !b.full && b.head == b.tail {
// 		return nil, errors.New("buffer is empty") // No items to pop
// 	}

// 	var items []T
// 	for i := 0; i < n && (b.full || b.head != b.tail); i++ {
// 		item := b.items[b.head]
// 		b.head = (b.head + 1) % b.maxSize
// 		b.full = false
// 		items = append(items, item)
// 	}

// 	return items, nil
// }

// func (hb *HierarchicalBuffer[T]) PopN(n int) ([]T, error) {
// 	var items []T

// 	hb.Lock()
// 	defer hb.Unlock()

// 	start := hb.readIndex
// 	for n > 0 {
// 		buffer := hb.buffers[hb.readIndex]
// 		popped, err := buffer.PopN(n)
// 		if err == nil {
// 			items = append(items, popped...)
// 			n -= len(popped)
// 		}

// 		// Move to next buffer
// 		hb.readIndex = (hb.readIndex + 1) % len(hb.buffers)

// 		// If we have checked all buffers and found no items, break
// 		if hb.readIndex == start && len(items) == 0 {
// 			return nil, errors.New("all buffers are empty")
// 		}
// 	}

//		return items, nil
//	}
// func (hb *HierarchicalBuffer[T]) PopN(n int) ([]T, error) {
// 	hb.Lock()
// 	defer hb.Unlock()

// 	var items []T
// 	start := hb.readIndex
// 	for n > 0 {
// 		buffer := hb.buffers[hb.readIndex]
// 		popped, err := buffer.PopN(n)
// 		if err == nil && len(popped) > 0 {
// 			items = append(items, popped...)
// 			n -= len(popped)
// 		}

// 		hb.readIndex = (hb.readIndex + 1) % len(hb.buffers)

// 		if hb.readIndex == start && len(items) == 0 {
// 			return nil, errors.New("all buffers are empty")
// 		}
// 	}

// 	return items, nil
// }

// func (hb *HierarchicalBuffer[T]) PopN(n int) ([]T, error) {
// 	var items []T

// 	hb.Lock()
// 	defer hb.Unlock()

// 	for n > 0 && hb.readIndex < len(hb.buffers) {
// 		popped, err := hb.buffers[hb.readIndex].PopN(n)
// 		if err != nil {
// 			hb.readIndex++
// 			continue
// 		}
// 		items = append(items, popped...)
// 		n -= len(popped)
// 		if n <= 0 {
// 			break
// 		}
// 	}

// 	if len(items) == 0 {
// 		return nil, errors.New("no items popped")
// 	}

// 	return items, nil
// }

// func main() {
// 	// Example usage
// 	hBuffer := NewHierarchicalBuffer[int](5, 1) // Buffers of size 5, initially 1 buffer

// 	for i := 0; i < 10; i++ {
// 		hBuffer.Push(i)
// 	}

// 	for i := 0; i < 10; i++ {
// 		item, err := hBuffer.Pop()
// 		if err != nil {
// 			panic(err)
// 		}
// 		println(item)
// 	}
// }
