package main

import (
	"fmt"
	"runtime"
	"sync/atomic"

	"github.com/davidroman0O/seigyo/playground/stackheap/ringbuffer"
)

const (
	defaultThroughput = 512
	batchSize         = 1024 * 4
)

const (
	stateIdle int32 = iota
	stateRunning
	stateStopped
)

const (
	taggedNone int32 = iota
	taggedDeletion
)

// ShortWork represents a unit of work
type ShortWork func() error

// LongWork is a goroutine that will run longer
type LongWork func() error

// Allowing to know if we have to swtich
type AnonymousTask any

// Scheduler manages task execution
type Scheduler struct {
	state             int32 // state
	tag               int32
	targetThroughput  int64
	currentThroughput int64
	taskQueue         *ringbuffer.CircularBuffer[interface{}]
}

// NewScheduler creates a new Scheduler with references to other schedulers for work stealing
func NewScheduler(
	queueSize int,
	targetThroughput int64,
) *Scheduler {
	return &Scheduler{
		targetThroughput: targetThroughput,
		taskQueue:        ringbuffer.NewCircularBuffer[interface{}](queueSize),
	}
}

func (s *Scheduler) TagDeletion() error {
	if atomic.LoadInt32(&s.tag) != taggedNone {
		return fmt.Errorf("can't tag")
	}
	atomic.StoreInt32(&s.tag, taggedDeletion)
	return nil
}

func (s *Scheduler) Stop() {
	atomic.StoreInt32(&s.state, stateStopped)
}

// Push adds a task to the scheduler
func (s *Scheduler) Push(task AnonymousTask) {
	switch task.(type) {
	case LongWork, ShortWork:
		if err := s.taskQueue.Enqueue(task); err != nil {
			fmt.Println(err)
		}
		if atomic.CompareAndSwapInt32(&s.state, stateIdle, stateRunning) {
			go func() {
				s.process()
				atomic.StoreInt32(&s.state, stateIdle)
			}()
		}
	default:
		fmt.Println("not a managed task")
	}
}

func (s *Scheduler) process() {
	var count int64
	var items []interface{}
	var err error
	for atomic.LoadInt32(&s.state) != stateStopped {
		if count > s.targetThroughput {
			count = 0
			runtime.Gosched()
		}
		count++
		if s.taskQueue.IsEmpty() {
			return // it will put to idle again
		}
		if items, _, err = s.taskQueue.DequeueN(batchSize); err != nil {
			// TODO @droman: test if its even possible
			fmt.Println(err)
			return
		}
		for _, item := range items {
			switch item.(type) {
			case ShortWork:
				task := item.(ShortWork)
				if err := task(); err != nil {
					fmt.Println("unmanaged error", err)
				}
				atomic.AddInt64(&s.currentThroughput, 1) // Increment the counter
				atomic.AddInt64(&metricThrouput, 1)      // Increment the counter
			case LongWork:
				task := item.(LongWork)
				go func() {
					if err := task(); err != nil {
						fmt.Println("unmanaged error", err)
					}
					atomic.AddInt64(&s.currentThroughput, 1) // Increment the counter
					atomic.AddInt64(&metricThrouput, 1)      // Increment the counter
				}()
			}
		}
	}
}
