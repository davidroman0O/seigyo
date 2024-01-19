package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davidroman0O/seigyo/playground/sched/ringbuffer"
	"github.com/enriquebris/goconcurrentqueue"
)

type SchedulerState int32

const (
	StateProcessing SchedulerState = iota
	StateTransferring
)

// Task represents a unit of work
type Task func()

// Scheduler manages task execution
type Scheduler struct {
	state      atomic.Value
	ringBuffer *ringbuffer.CircularBuffer[any] // Preliminary ring buffer
	taskQueue  *goconcurrentqueue.FixedFIFO
	stealFrom  []*Scheduler
}

// NewScheduler creates a new Scheduler with references to other schedulers for work stealing
func NewScheduler(ringBufferSize, queueSize int, stealFrom []*Scheduler) *Scheduler {
	s := &Scheduler{
		ringBuffer: ringbuffer.NewCircularBuffer[any](ringBufferSize),
		taskQueue:  goconcurrentqueue.NewFixedFIFO(queueSize),
		stealFrom:  stealFrom,
	}
	s.state.Store(StateProcessing)
	return s
}

// Schedule adds a task to the ring buffer
func (s *Scheduler) Schedule(task Task) {
	s.ringBuffer.Enqueue(task)
}

func (s *Scheduler) transferTasks() {
	if s.state.Load().(SchedulerState) != StateTransferring {
		return // Exit if not in transferring state
	}
	// for {
	// currentState := s.state.Load().(SchedulerState)
	// if currentState == StateProcessing {
	// 	time.Sleep(10 * time.Millisecond) // Adjust as needed
	// 	continue
	// }
	tasks, _, err := s.ringBuffer.DequeueN(300) // Adjust batch size as needed
	fmt.Println("transfer", err)
	for _, v := range tasks {
		if err := s.taskQueue.Enqueue(v); err != nil {
			fmt.Println("can't push", err)
			s.ringBuffer.Enqueue(v)           // Re-enqueue the task
			s.state.Store(StateProcessing)    // Switch state to processing
			time.Sleep(10 * time.Millisecond) // Adjust as needed
			break
		}
	}
	// }
}

// Global atomic counter
var processedCount int64

func (s *Scheduler) Run() {
	// fmt.Println("run")
	for {
		// Determine the state based on the length of the FixedFIFO queue
		if s.taskQueue.GetLen() >= s.taskQueue.GetCap() { // Check if the FIFO queue is full
			s.state.Store(StateTransferring)
			// fmt.Println("change state StateTransferring")
		} else {
			s.state.Store(StateProcessing)
			// fmt.Println("change state StateProcessing")
		}

		// Handle state-specific logic
		currentState := s.state.Load().(SchedulerState)
		if currentState == StateTransferring {
			// fmt.Println("transfer xxx")
			// Perform the transfer of tasks from the CircularBuffer to the FixedFIFO queue
			s.transferTasks()
		} else if currentState == StateProcessing {

			// fmt.Println(s.taskQueue.GetLen())

			// Process tasks from the FIFO queue
			if s.taskQueue.GetLen() > 0 {
				item, err := s.taskQueue.Dequeue()
				if err == nil {
					task := item.(Task)
					fmt.Println(task)
					task()
					atomic.AddInt64(&processedCount, 1) // Increment the counter
					fmt.Println("Processed a task")
				} else {
					fmt.Println("no dequeue ", err)
				}
			}

			// Work stealing logic (if applicable)
			for _, other := range s.stealFrom {
				if other == nil {
					continue
				}
				if other.taskQueue == nil {
					continue
				}
				if other.taskQueue.GetLen() > 0 {
					item, err := other.taskQueue.Dequeue()
					if err == nil {
						task := item.(Task)
						task()
						atomic.AddInt64(&processedCount, 1) // Increment the counter for stolen tasks
						break
					}
				}
			}
		} else {
			fmt.Println("no state")
		}

		// Yield to allow other goroutines to run
		runtime.Gosched()

		// Short sleep to prevent tight loop
		time.Sleep(1 * time.Millisecond)
	}
}

func reportProcessedCount() {
	ticker := time.NewTicker(1 * time.Second)
	for {
		<-ticker.C
		count := atomic.LoadInt64(&processedCount)
		fmt.Printf("Tasks processed in the last second: %d\n", count)
		atomic.StoreInt64(&processedCount, 0) // Reset the counter
	}
}

var queueSize = 1024 * 4

func main() {
	numCores := runtime.NumCPU()
	runtime.GOMAXPROCS(numCores)

	schedulers := make([]*Scheduler, numCores)
	for i := 0; i < numCores; i++ {
		schedulers[i] = NewScheduler(queueSize, queueSize, schedulers) // Each scheduler can steal from any other
		go schedulers[i].Run()
	}

	go reportProcessedCount() // Start reporting processed tasks

	// Example: Schedule some tasks
	var wg sync.WaitGroup
	for i := 0; i < 10000000; i++ {
		wg.Add(1)
		task := func() {
			// Task logic here
			wg.Done()
		}
		schedulers[i%numCores].Schedule(task)
	}

	wg.Wait() // Wait for all tasks to complete
}
