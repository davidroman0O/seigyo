package main

import (
	"fmt"
	"runtime"
	"sync/atomic"

	"github.com/davidroman0O/seigyo/playground/stackheap/buffer"
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
	state            int32 // state
	tag              int32
	targetThroughput int64

	queueSize      int
	initialBuffers int

	currentThroughput int64
	taskQueue         *buffer.HierarchicalBuffer[interface{}]
}

// NewScheduler creates a new Scheduler with references to other schedulers for work stealing
func NewScheduler(
	queueSize int,
	initialBuffers int,
	targetThroughput int64,
) *Scheduler {
	return &Scheduler{
		targetThroughput: targetThroughput,
		initialBuffers:   initialBuffers,
		queueSize:        queueSize,
		taskQueue:        buffer.NewHierarchicalBuffer[interface{}](queueSize, initialBuffers),
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
	case LongWork, ShortWork: // i have plan for other types
		if err := s.taskQueue.Push(task); err != nil {
			fmt.Println("is full", err)
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

func (s *Scheduler) PushN(tasks []AnonymousTask) {
	ourTypes := []interface{}{}
	for i := 0; i < len(tasks); i++ {
		switch msg := tasks[i].(type) {
		case LongWork, ShortWork:
			ourTypes = append(ourTypes, tasks[i])
		default:
			fmt.Println("not a managed task", msg)
		}
	}
	if err := s.taskQueue.PushN(ourTypes); err != nil {
		fmt.Println("is full", err)
	}
	if atomic.CompareAndSwapInt32(&s.state, stateIdle, stateRunning) {
		go func() {
			s.process()
			atomic.StoreInt32(&s.state, stateIdle)
		}()
	}
}

func (s *Scheduler) process() {
	var count int64
	var items []interface{}
	// var empty bool
	var err error
	for atomic.LoadInt32(&s.state) != stateStopped {
		if count > s.targetThroughput {
			count = 0
			runtime.Gosched()
		}
		count++
		if items, err = s.taskQueue.PopN(s.queueSize); err != nil {
			if err == buffer.ErrAllBuffersEmpty {
				count = s.targetThroughput + 1
				continue
			}
			fmt.Println("can't popN ", err)
			return
		}
		for _, item := range items {
			if item == nil {
				fmt.Println("item is nil", item)
				continue
			}
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
