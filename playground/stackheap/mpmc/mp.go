package mpmcq

import (
	"errors"
	"runtime"
	"sync/atomic"
	"time"
)

var (
	ErrFlushed = errors.New(`queue: flushed`)

	ErrTimeout = errors.New(`queue: get operator timed out`)
)

func minQuantity(v uint64) uint64 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v |= v >> 32
	v++
	return v
}

type slot struct {
	position uint64
	data     interface{}
}

type slots []*slot

// MPMCQueue =>
type MPMCQueue struct {
	_padding0     [8]uint64
	queue         uint64
	_padding1     [8]uint64
	dequeue       uint64
	_padding2     [8]uint64
	mask, flushed uint64
	_padding3     [8]uint64
	slots         slots
}

// NewMPMCQueue will allocate, initialize, and return a ring buffer
// with the specified size.
func NewMPMCQueue(size uint64) *MPMCQueue {
	mq := &MPMCQueue{}
	mq.new(size)
	return mq
}

func (mq *MPMCQueue) new(size uint64) {
	size = minQuantity(size)
	mq.slots = make(slots, size)
	for i := uint64(0); i < size; i++ {
		mq.slots[i] = &slot{position: i}
	}
	mq.mask = size - 1
}

func (mq *MPMCQueue) Put(item interface{}) error {
	_, err := mq.tryPut(item, false)
	return err
}

func (mq *MPMCQueue) Emplace(item interface{}) (bool, error) {
	return mq.tryPut(item, true)
}

func (mq *MPMCQueue) tryPut(item interface{}, emplace bool) (bool, error) {
	var slot *slot
	pos := atomic.LoadUint64(&mq.queue)
L:
	for {
		if atomic.LoadUint64(&mq.flushed) == 1 {
			return false, ErrFlushed
		}

		slot = mq.slots[pos&mq.mask]
		seq := atomic.LoadUint64(&slot.position)
		switch dif := seq - pos; {
		case dif == 0:
			if atomic.CompareAndSwapUint64(&mq.queue, pos, pos+1) {
				break L
			}
		case dif < 0:
			panic(`operation could not be completed due to negative difference between seq and pos.
			the ring buffer is corrupted.`)
		default:
			pos = atomic.LoadUint64(&mq.queue)
		}

		if emplace {
			return false, nil
		}

		runtime.Gosched() // free up the cpu before the next iteration
	}

	slot.data = item
	atomic.StoreUint64(&slot.position, pos+1)
	return true, nil
}

func (mq *MPMCQueue) Pop() (interface{}, error) {
	return mq.tryPop(0)
}

func (mq *MPMCQueue) tryPop(timeout time.Duration) (interface{}, error) {
	var (
		slot  *slot
		pos   = atomic.LoadUint64(&mq.dequeue)
		start time.Time
	)
	if timeout > 0 {
		start = time.Now()
	}
L:
	for {
		if atomic.LoadUint64(&mq.flushed) == 1 {
			return nil, ErrFlushed
		}

		slot = mq.slots[pos&mq.mask]
		seq := atomic.LoadUint64(&slot.position)
		switch dif := seq - (pos + 1); {
		case dif == 0:
			if atomic.CompareAndSwapUint64(&mq.dequeue, pos, pos+1) {
				break L
			}
		case dif < 0:
			panic(`operation could not be completed due to negative difference between seq and pos.
			the ring buffer is corrupted.`)
		default:
			pos = atomic.LoadUint64(&mq.dequeue)
		}

		if timeout > 0 && time.Since(start) >= timeout {
			return nil, ErrTimeout
		}

		runtime.Gosched() // free up the cpu before the next iteration
	}
	data := slot.data
	slot.data = nil
	atomic.StoreUint64(&slot.position, pos+mq.mask+1)
	return data, nil
}

func (mq *MPMCQueue) PopN(n uint64) ([]interface{}, error) {
	items := make([]interface{}, 0, n)
	for i := uint64(0); i < n; i++ {
		item, err := mq.tryPop(0) // Assuming no timeout needed for PopN
		if err != nil {
			// Handle specific errors, like ErrFlushed, if needed
			return items, err
		}
		if item == nil {
			// This condition can happen if the queue is empty
			break
		}
		items = append(items, item)
	}
	return items, nil
}

func (mq *MPMCQueue) Length() uint64 {
	return atomic.LoadUint64(&mq.queue) - atomic.LoadUint64(&mq.dequeue)
}

func (mq *MPMCQueue) Capacity() uint64 {
	return uint64(len(mq.slots))
}

func (mq *MPMCQueue) Flush() {
	atomic.CompareAndSwapUint64(&mq.flushed, 0, 1)
}
