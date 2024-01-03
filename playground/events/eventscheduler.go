package events

import (
	"runtime"
	"sync"
	"sync/atomic"
)

const (
	idle int32 = iota
	running
	stopped
)

type Task func()

type BatchScheduler struct {
	taskQueue  chan Task
	procStatus int32
	batchSize  int
	throughput int
	wg         sync.WaitGroup
}

func NewBatchScheduler(batchSize, throughput int) *BatchScheduler {
	return &BatchScheduler{
		taskQueue:  make(chan Task),
		batchSize:  batchSize,
		throughput: throughput,
	}
}

func (bs *BatchScheduler) Schedule(task Task) {
	if atomic.LoadInt32(&bs.procStatus) != stopped {
		bs.taskQueue <- task
	}
}

func (bs *BatchScheduler) Run() {
	for {
		select {
		case task, ok := <-bs.taskQueue:
			if !ok {
				// Task queue closed, check if it's time to stop
				if atomic.LoadInt32(&bs.procStatus) == stopped {
					return
				}
				continue
			}

			atomic.StoreInt32(&bs.procStatus, running)
			bs.executeTask(task)
			atomic.StoreInt32(&bs.procStatus, idle)

		default:
			// If there are no tasks, check if the scheduler should stop
			if atomic.LoadInt32(&bs.procStatus) == stopped {
				return
			}
			runtime.Gosched() // Yield the processor
		}
	}
}

func (bs *BatchScheduler) executeTask(task Task) {
	var i int
	tasks := []Task{task}

	// Fetch additional tasks to fill the batch
	for len(tasks) < bs.batchSize {
		select {
		case additionalTask, ok := <-bs.taskQueue:
			if !ok {
				break // Task queue closed
			}
			tasks = append(tasks, additionalTask)
		default:
			break // No more tasks available
		}
	}

	// Execute the tasks in the batch
	for _, t := range tasks {
		t()
		i++
		if i >= bs.throughput {
			i = 0
			runtime.Gosched()
		}
	}
}

func (bs *BatchScheduler) Stop() {
	atomic.StoreInt32(&bs.procStatus, stopped)
	close(bs.taskQueue)
	bs.wg.Wait() // Wait for all tasks to complete
}

// /////////////////////////////////////////////////////////////////////////

const defaultThroughput = 300
const messageBatchSize = 1024 * 4

///////////////////////////////////////////////////////////////////////////

// type Scheduler struct {
// 	tasks        chan func()
// 	stop         chan struct{}
// 	wg           sync.WaitGroup
// 	procCount    int32
// 	rateLimiter  <-chan time.Time
// 	maxQueueSize int
// }

// func NewScheduler(concurrencyLevel int, rateLimit time.Duration, maxQueueSize int) *Scheduler {
// 	s := &Scheduler{
// 		tasks:        make(chan func(), maxQueueSize),
// 		stop:         make(chan struct{}),
// 		rateLimiter:  time.Tick(rateLimit),
// 		maxQueueSize: maxQueueSize,
// 	}
// 	s.wg.Add(concurrencyLevel)
// 	for i := 0; i < concurrencyLevel; i++ {
// 		go s.worker()
// 	}
// 	return s
// }

// func (s *Scheduler) Schedule(task func()) bool {
// 	if len(s.tasks) >= s.maxQueueSize {
// 		return false // Queue is full, task cannot be scheduled now
// 	}
// 	s.tasks <- task
// 	return true
// }

// func (s *Scheduler) worker() {
// 	defer s.wg.Done()
// 	for {
// 		select {
// 		case task := <-s.tasks:
// 			<-s.rateLimiter // Wait for the next tick before processing the task
// 			atomic.AddInt32(&s.procCount, 1)
// 			task()
// 			atomic.AddInt32(&s.procCount, -1)
// 		case <-s.stop:
// 			return
// 		}
// 	}
// }

// // Stop waits for all tasks to complete and stops the scheduler.
// func (s *Scheduler) Stop() {
// 	close(s.stop)
// 	s.wg.Wait()
// 	close(s.tasks)
// }

// // CurrentRunningTasks returns the number of tasks currently being processed.
// func (s *Scheduler) CurrentRunningTasks() int {
// 	return int(atomic.LoadInt32(&s.procCount))
// }
