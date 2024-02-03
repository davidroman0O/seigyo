package pcbuffer

import (
	"errors"
	"sync/atomic"
)

// BufferSegment represents a segment of the Buffer.
type BufferSegment[T any] struct {
	items   []T
	head    int32
	tail    int32
	padding [8]int32 // Padding to prevent false sharing
	maxSize int32
}

// Buffer is a concurrent segmented ring buffer.
type Buffer[T any] struct {
	segments    []*BufferSegment[T]
	segmentSize int32
}

// NewBufferSegment creates a new BufferSegment with a given size.
func NewBufferSegment[T any](size int) *BufferSegment[T] {
	return &BufferSegment[T]{
		items: make([]T, size),
	}
}

// pushSegment adds an item to the segment. Returns error if the segment is full.
func (s *BufferSegment[T]) pushSegment(item T) error {
	tail := atomic.LoadInt32(&s.tail)
	nextTail := (tail + 1) % s.maxSize

	if nextTail == atomic.LoadInt32(&s.head) { // Segment is full
		return errors.New("segment is full")
	}

	s.items[tail] = item
	atomic.StoreInt32(&s.tail, nextTail)

	return nil
}

// popSegment removes and returns an item from the segment. Returns error if the segment is empty.
func (s *BufferSegment[T]) popSegment() (T, error) {
	head := atomic.LoadInt32(&s.head)
	if head == atomic.LoadInt32(&s.tail) { // Segment is empty
		var zero T
		return zero, errors.New("segment is empty")
	}

	item := s.items[head]
	atomic.StoreInt32(&s.head, (head+1)%s.maxSize)

	return item, nil
}

// NewBuffer creates a new Buffer with given segment size and total size.
func NewBuffer[T any](segmentSize, totalSize int) *Buffer[T] {
	numSegments := (totalSize + segmentSize - 1) / segmentSize
	segments := make([]*BufferSegment[T], numSegments)
	for i := range segments {
		segments[i] = NewBufferSegment[T](segmentSize)
	}
	return &Buffer[T]{
		segments:    segments,
		segmentSize: int32(segmentSize),
	}
}

// Push adds an item to the buffer. Returns error if the buffer is full.
func (b *Buffer[T]) Push(item T) error {
	for _, segment := range b.segments {
		tail := atomic.LoadInt32(&segment.tail)
		nextTail := (tail + 1) % b.segmentSize

		if nextTail != atomic.LoadInt32(&segment.head) { // Segment is not full
			segment.items[tail] = item
			atomic.StoreInt32(&segment.tail, nextTail)
			return nil
		}
	}
	return errors.New("buffer is full")
}

// PopN removes and returns up to n items from the buffer. Returns error if the buffer is empty.
func (b *Buffer[T]) PopN(n int) ([]T, error) {
	var items []T
	for _, segment := range b.segments {
		head := atomic.LoadInt32(&segment.head)

		if head != atomic.LoadInt32(&segment.tail) { // Segment is not empty
			for i := 0; i < n && head != atomic.LoadInt32(&segment.tail); i++ {
				items = append(items, segment.items[head])
				head = (head + 1) % b.segmentSize
				n--
			}
			atomic.StoreInt32(&segment.head, head)
			if n == 0 {
				break
			}
		}
	}
	if len(items) == 0 {
		return nil, errors.New("buffer is empty")
	}
	return items, nil
}
