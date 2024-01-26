package circular

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// Config defines configuration options for the buffer.
type Config struct {
	AutoResize bool
}

// Buffer is our circular buffer
type Buffer struct {
	read, write uint64
	lastWrite   uint64
	maskVal     uint64
	data        []unsafe.Pointer
	autoResize  bool
}

// NewBuffer allocates a new buffer. The size needs to be a power of two.
func NewBuffer(size uint64, config *Config) *Buffer {
	if size&(size-1) != 0 {
		panic("size must be a power of two")
	}
	return &Buffer{
		read:       1,
		write:      1,
		data:       make([]unsafe.Pointer, size),
		maskVal:    size - 1,
		autoResize: config != nil && config.AutoResize,
	}
}

// resize doubles the size of the buffer. Must be called atomically.
func (b *Buffer) resize() {
	newSize := (b.maskVal + 1) * 2
	newData := make([]unsafe.Pointer, newSize)
	for i := uint64(0); i <= b.maskVal; i++ {
		newData[i] = b.data[b.mask(b.read+i)]
	}
	b.data = newData
	b.read = 0
	b.write = b.maskVal + 1
	b.maskVal = newSize - 1
	b.lastWrite = b.maskVal
}

// Size returns the current number of elements in the buffer.
func (b *Buffer) Size() uint64 {
	return atomic.LoadUint64(&b.write) - atomic.LoadUint64(&b.read)
}

// Empty returns true if the buffer is empty.
func (b *Buffer) Empty() bool {
	return b.Size() == 0
}

// Full returns true if the buffer is full.
func (b *Buffer) Full() bool {
	return b.Size() > b.maskVal
}

// mask is a helper to wrap values around the buffer's size.
func (b *Buffer) mask(val uint64) uint64 {
	return val & b.maskVal
}

// Push places an item onto the ring buffer.
func (b *Buffer) Push(object unsafe.Pointer) bool {
	if b.Full() {
		if b.autoResize {
			fmt.Println("resize")
			b.resize()
		} else {
			fmt.Println("it's fulls")
			return true // Buffer is full
		}
	}

	index := atomic.AddUint64(&b.write, 1) - 1
	atomic.StorePointer(&b.data[b.mask(index)], object)

	for !atomic.CompareAndSwapUint64(&b.lastWrite, index-1, index) {
		runtime.Gosched()
	}

	return false
}

// Pop returns the next item on the ring buffer.
func (b *Buffer) Pop() (unsafe.Pointer, bool) {
	if b.Empty() {
		return nil, false // Buffer is empty
	}

	index := atomic.AddUint64(&b.read, 1) - 1
	for index >= atomic.LoadUint64(&b.write) {
		runtime.Gosched()
	}

	return atomic.LoadPointer(&b.data[b.mask(index)]), true
}

// PopN returns up to N items from the ring buffer.
func (b *Buffer) PopN(n uint64) ([]unsafe.Pointer, bool) {
	actualN := min(n, b.Size()) // Get the lesser of n or the buffer size
	if actualN == 0 {
		return nil, true // Buffer is empty
	}

	items := make([]unsafe.Pointer, actualN)
	for i := uint64(0); i < actualN; i++ {
		item, ok := b.Pop()
		if !ok {
			break // Should not happen since we've checked the size, but just in case
		}
		items[i] = item
	}

	return items, false
}

// min returns the smaller of two uint64 values.
func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// PushN places N items onto the ring buffer.
func (b *Buffer) PushN(objects []unsafe.Pointer) bool {
	if b.Size()+uint64(len(objects)) > b.maskVal {
		return false // Not enough space to push all items
	}

	for _, object := range objects {
		if !b.Push(object) {
			return false
		}
	}
	return true
}
