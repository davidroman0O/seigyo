package ringbuffer

import (
	"fmt"
	"sync"
)

type CircularBuffer[T any] struct {
	mu          sync.Mutex
	arr         []T
	head        int
	tail        int
	size        int
	capacity    int
	resize      bool
	initialSize int // Adding initialSize to keep track of the initial capacity
	empty       T
}

const (
	ReduceHalf = iota
	ReduceQuarter
	ReduceCustom // for custom size reduction
)

func NewCircularBuffer[T any](capacity int) *CircularBuffer[T] {
	buffer := &CircularBuffer[T]{
		arr:         make([]T, capacity),
		capacity:    capacity,
		resize:      true,
		initialSize: capacity, // Set the initial capacity
		empty:       *new(T),
	}
	buffer.Resize(capacity)
	return buffer
}

func (cb *CircularBuffer[T]) Enqueue(event T) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.size == cb.capacity {
		// fmt.Println("is full enqueue", cb.size == cb.capacity)
		if cb.resize {
			fmt.Println("enqueue resize!", cb.size, "->", cb.capacity*2)
			newBuffer := make([]T, cb.capacity*2)
			for i := 0; i < cb.size; i++ {
				newBuffer[i] = cb.arr[(cb.head+i)%cb.capacity]
			}
			cb.arr = newBuffer
			cb.head = 0
			cb.tail = cb.size
			cb.capacity = cb.capacity * 2
		} else {
			return fmt.Errorf("buffer is full")
		}
	}
	cb.arr[cb.tail] = event
	cb.tail = (cb.tail + 1) % cb.capacity
	cb.size++
	return nil
}

func (cb *CircularBuffer[T]) Dequeue() (T, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.size == 0 {
		return cb.empty, fmt.Errorf("buffer is empty")
	}
	event := cb.arr[cb.head]
	cb.head = (cb.head + 1) % cb.capacity
	cb.size--
	return event, nil
}

func (cb *CircularBuffer[T]) EnqueueN(items []T) int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	numEnqueued := 0
	remainingCapacity := cb.capacity - cb.size

	numToEnqueue := min(len(items), remainingCapacity)
	firstPart := min(numToEnqueue, cb.capacity-cb.tail)

	// Copy the first part
	copy(cb.arr[cb.tail:], items[:firstPart])
	numEnqueued += firstPart

	// If there's more to enqueue and buffer is wrapped
	if numToEnqueue > firstPart {
		secondPart := numToEnqueue - firstPart
		copy(cb.arr, items[firstPart:numToEnqueue])
		numEnqueued += secondPart
	}

	// Update tail and size
	cb.tail = (cb.tail + numEnqueued) % cb.capacity
	cb.size += numEnqueued

	return numEnqueued
}

func (cb *CircularBuffer[T]) Downsize(strategy int, customSize int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	var newCapacity int

	switch strategy {
	case ReduceHalf:
		newCapacity = cb.capacity / 2
	case ReduceQuarter:
		newCapacity = cb.capacity / 4
	case ReduceCustom:
		newCapacity = customSize
	default:
		fmt.Println("Invalid downsizing strategy")
		return
	}

	// Check against the initial size
	if newCapacity < cb.initialSize {
		fmt.Println("Cannot downsize below the initial capacity.")
		return
	}

	if newCapacity < cb.size {
		fmt.Println("New capacity is too small to hold existing data.")
		return
	}

	// Create a new, smaller buffer
	newBuffer := make([]T, newCapacity)

	// Copy existing items to the new buffer
	for i := 0; i < cb.size; i++ {
		newBuffer[i] = cb.arr[(cb.head+i)%cb.capacity]
	}

	// Update buffer properties
	cb.arr = newBuffer
	cb.head = 0
	cb.tail = cb.size
	cb.capacity = newCapacity
}

func (cb *CircularBuffer[T]) DequeueN(n int) ([]T, int, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.size == 0 {
		return nil, 0, fmt.Errorf("buffer is empty")
	}

	// Dequeue only as many items as are available, up to n
	numItems := min(n, cb.size)
	if numItems <= 0 {
		fmt.Println(n, cb.size, numItems)
		return nil, 0, fmt.Errorf("wait for new items")
	}
	items := make([]T, numItems)

	for i := 0; i < numItems; i++ {
		items[i] = cb.arr[cb.head]
		cb.head = (cb.head + 1) % cb.capacity
		cb.size--
	}

	return items, numItems, nil
}

func (cb *CircularBuffer[T]) IsFull() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	// fmt.Println("is full", cb.size == cb.capacity, cb.size, cb.capacity)
	return cb.size == cb.capacity
}

func (cb *CircularBuffer[T]) IsEmpty() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.size == 0
}

func (cb *CircularBuffer[T]) IsHalfFull() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.size <= cb.capacity/2
}

func (cb *CircularBuffer[T]) IsQuarterFull() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.size <= cb.capacity/4
}

func (cb *CircularBuffer[T]) IsFractionFull(fraction float64) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if fraction <= 0 || fraction > 1 {
		fmt.Println("Invalid fraction value. Must be > 0 and <= 1")
		return false
	}
	threshold := int(float64(cb.capacity) * fraction)
	return cb.size <= threshold
}

func (cb *CircularBuffer[T]) Resize(newCapacity int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	// fmt.Println("resize!", cb.capacity, "->", newCapacity)
	newBuffer := make([]T, newCapacity)
	for i := 0; i < cb.size; i++ {
		newBuffer[i] = cb.arr[(cb.head+i)%cb.capacity]
	}
	cb.arr = newBuffer
	cb.head = 0
	cb.tail = cb.size
	cb.capacity = newCapacity
}

// min returns the smaller of x or y.
// TODO @droman: I really need to upgrade my `go` to use the standard one
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
