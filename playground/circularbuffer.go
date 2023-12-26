package events

import "fmt"

type CircularBuffer[T any] struct {
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
	if cb.IsFull() {
		// fmt.Println("is full enqueue", cb.IsFull())
		if cb.resize {
			fmt.Println("enqueue resize!", cb.size, "->", cb.capacity*2)
			cb.Resize(cb.capacity * 2)
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
	if cb.IsEmpty() {
		return cb.empty, fmt.Errorf("buffer is empty")
	}
	event := cb.arr[cb.head]
	cb.head = (cb.head + 1) % cb.capacity
	cb.size--
	return event, nil
}

func (cb *CircularBuffer[T]) EnqueueN(items []T) int {
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

// // DequeueN dequeues up to n items from the buffer.
// // It returns a slice of dequeued items and the actual number of items dequeued.
// func (cb *CircularBuffer[T]) DequeueN(n int) ([]T, int, error) {
// 	if cb.IsEmpty() {
// 		return nil, 0, fmt.Errorf("buffer is empty")
// 	}

// 	numToDequeue := min(n, cb.size)
// 	items := make([]T, numToDequeue)

// 	firstPart := min(numToDequeue, cb.capacity-cb.head)

// 	// Copy the first part
// 	copy(items, cb.events[cb.head:cb.head+firstPart])

// 	// If wrapped around and there's more to dequeue
// 	if numToDequeue > firstPart {
// 		secondPart := numToDequeue - firstPart
// 		copy(items[firstPart:], cb.events[:secondPart])
// 	}

// 	// Update head and size
// 	cb.head = (cb.head + numToDequeue) % cb.capacity
// 	cb.size -= numToDequeue

//		return items, numToDequeue, nil
//	}

func (cb *CircularBuffer[T]) Downsize(strategy int, customSize int) {
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
	if cb.IsEmpty() {
		return nil, 0, fmt.Errorf("buffer is empty")
	}

	// Dequeue only as many items as are available, up to n
	numItems := min(n, cb.size)
	items := make([]T, numItems)

	for i := 0; i < numItems; i++ {
		items[i] = cb.arr[cb.head]
		cb.head = (cb.head + 1) % cb.capacity
		cb.size--
	}

	return items, numItems, nil
}

func (cb *CircularBuffer[T]) IsFull() bool {
	// fmt.Println("is full", cb.size == cb.capacity, cb.size, cb.capacity)
	return cb.size == cb.capacity
}

func (cb *CircularBuffer[T]) IsEmpty() bool {
	return cb.size == 0
}

func (cb *CircularBuffer[T]) IsHalfFull() bool {
	return cb.size <= cb.capacity/2
}

func (cb *CircularBuffer[T]) IsQuarterFull() bool {
	return cb.size <= cb.capacity/4
}

func (cb *CircularBuffer[T]) IsFractionFull(fraction float64) bool {
	if fraction <= 0 || fraction > 1 {
		fmt.Println("Invalid fraction value. Must be > 0 and <= 1")
		return false
	}
	threshold := int(float64(cb.capacity) * fraction)
	return cb.size <= threshold
}

func (cb *CircularBuffer[T]) Resize(newCapacity int) {
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
