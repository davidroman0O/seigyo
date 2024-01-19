package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davidroman0O/seigyo/playground/sched/ringbuffer"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

///////////////////////////////////

const (
	defaultThroughput = 512
	batchSize         = 1024 * 4
)

const (
	stateIdle int32 = iota
	stateRunning
	stateStopped
)

// Global atomic counter
var metricProcessedCount int64

// ShortWork represents a unit of work
type ShortWork func() error

// LongWork is a goroutine that will run longer
type LongWork func() error

// Allowing to know if we have to swtich
type AnonymousTask any

// Scheduler manages task execution
type Scheduler struct {
	id                int
	state             int32
	targetThroughput  int64
	currentThroughput int64
	taskQueue         *ringbuffer.CircularBuffer[interface{}]
}

// NewScheduler creates a new Scheduler with references to other schedulers for work stealing
func NewScheduler(
	id int,
	queueSize int,
	targetThroughput int64,
) *Scheduler {
	return &Scheduler{
		id:               id,
		targetThroughput: targetThroughput,
		taskQueue:        ringbuffer.NewCircularBuffer[interface{}](queueSize),
	}
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
				atomic.AddInt64(&s.currentThroughput, 1)  // Increment the counter
				atomic.AddInt64(&metricProcessedCount, 1) // Increment the counter
			case LongWork:
				task := item.(LongWork)
				go func() {
					if err := task(); err != nil {
						fmt.Println("unmanaged error", err)
					}
					atomic.AddInt64(&s.currentThroughput, 1)  // Increment the counter
					atomic.AddInt64(&metricProcessedCount, 1) // Increment the counter
				}()
			}
		}
	}
}

func Task(fn func() error) ShortWork {
	return fn
}

func Process(fn func() error) LongWork {
	return fn
}

func reportProcessedCount() {
	ticker := time.NewTicker(1 * time.Second)
	for {
		<-ticker.C
		count := atomic.LoadInt64(&metricProcessedCount)
		fmt.Printf("Tasks processed in the last second: %d\n", count)
		atomic.StoreInt64(&metricProcessedCount, 0) // Reset the counter
	}
}

type WorkScheduler struct {
	Schedulers    []*Scheduler
	coreMultipler int
	state         int32

	dispatcher int32 // atomic modulo dispatcher to different schedulers
}

func (w *WorkScheduler) Len() int {
	return len(w.Schedulers)
}

func (w *WorkScheduler) Push(task AnonymousTask) {
	total := len(w.Schedulers)
	if atomic.LoadInt32(&w.dispatcher) < int32(total) {
		atomic.AddInt32(&w.dispatcher, 1)
	} else {
		atomic.StoreInt32(&w.dispatcher, 1)
	}
	w.Schedulers[int(atomic.LoadInt32(&w.dispatcher))%total].Push(task)
}

func (w *WorkScheduler) scale() {
	for {
		ticker := time.NewTicker(time.Microsecond * 150)
		select {
		case <-ticker.C:
			// stop everything when asked
			if atomic.LoadInt32(&w.state) == stateStopped {
				for _, s := range w.Schedulers {
					s.Stop()
				}
				ticker.Stop()
				return
			}
			// Perform your desired actions here
		}
	}
}

func NewWorkScheduler() *WorkScheduler {
	runtime.GOMAXPROCS(runtime.NumCPU())
	ws := WorkScheduler{
		Schedulers:    make([]*Scheduler, 3),
		coreMultipler: runtime.NumCPU() * 1,
		dispatcher:    1,
	}
	go ws.scale()
	// we start with only one
	ws.Schedulers[0] = NewScheduler(0, batchSize*ws.coreMultipler, defaultThroughput)
	ws.Schedulers[1] = NewScheduler(1, batchSize*ws.coreMultipler, defaultThroughput)
	ws.Schedulers[2] = NewScheduler(2, batchSize*ws.coreMultipler, defaultThroughput)
	return &ws
}

// getSystemResourceUsage returns CPU usage for all cores and memory usage.
func getSystemResourceUsage() ([]float64, float64, error) {

	// Get CPU usage for each core
	cpuPercents, err := cpu.Percent(0, true)
	if err != nil {
		return nil, 0, err
	}

	// Get Memory usage
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, 0, err
	}

	memUsage := memStats.UsedPercent

	return cpuPercents, memUsage, nil
}

func checkCPUOverload(cpuPercents []float64) bool {
	const cpuOverloadThreshold = 90.0

	for _, usage := range cpuPercents {
		if usage > cpuOverloadThreshold {
			return true
		}
	}
	return false
}

type SchedulerManager struct {
	schedulers        []*Scheduler
	resourceThreshold float64
	maxSchedulers     int
	mu                sync.Mutex
	dispatcher        int32 // atomic modulo dispatcher to different schedulers
}

func NewSchedulerManager(maxSchedulers int, resourceThreshold float64) *SchedulerManager {
	sm := &SchedulerManager{
		schedulers:        make([]*Scheduler, 1, maxSchedulers),
		resourceThreshold: resourceThreshold,
		maxSchedulers:     maxSchedulers,
	}
	sm.schedulers[0] = NewScheduler(0, batchSize*batchSize*(runtime.NumCPU()*16), defaultThroughput)
	return sm
}

func (sm *SchedulerManager) MonitorResources() {
	for {
		ticker := time.NewTicker(time.Millisecond * 50)
		select {
		case <-ticker.C:
			cpuPercents, memUsage, err := getSystemResourceUsage()
			if err != nil {
				// Handle the error appropriately
				continue
			}
			isCPUOverloaded := checkCPUOverload(cpuPercents)
			isMemoryOverloaded := memUsage > sm.resourceThreshold

			sm.mu.Lock()
			if isCPUOverloaded || isMemoryOverloaded {
				if len(sm.schedulers) > 1 {
					// Logic to scale down schedulers
					schedulerToRemove := sm.schedulers[len(sm.schedulers)-1]
					schedulerToRemove.Stop()
					sm.schedulers = sm.schedulers[:len(sm.schedulers)-1]
				}
			} else if len(sm.schedulers) < sm.maxSchedulers {
				// Logic to scale up schedulers
				newScheduler := NewScheduler(len(sm.schedulers)+1, batchSize*(runtime.NumCPU()*16), defaultThroughput)
				sm.schedulers = append(sm.schedulers, newScheduler)
			}
			sm.mu.Unlock()
		}
	}
}

func (w *SchedulerManager) Len() int {
	return len(w.schedulers)
}

func (w *SchedulerManager) Push(task AnonymousTask) {
	total := len(w.schedulers)
	if atomic.LoadInt32(&w.dispatcher) < int32(total) {
		atomic.AddInt32(&w.dispatcher, 1)
	} else {
		atomic.StoreInt32(&w.dispatcher, 1)
	}
	w.schedulers[int(atomic.LoadInt32(&w.dispatcher))%total].Push(task)
}

func main() {
	// currently workingn on a resources and throughput based scaler
	{
		// workScheduler := NewWorkScheduler()

		workScheduler := NewSchedulerManager(200, 80)

		go workScheduler.MonitorResources()

		go reportProcessedCount() // Start reporting processed tasks

		// senders has to scale as much as the schedulers
		var wwg sync.WaitGroup
		for idx := 0; idx < workScheduler.Len(); idx++ {
			wwg.Add(1)
			go func() {
				defer func() {
					fmt.Println("done")
					wwg.Done()
				}()
				for i := 0; i < 30000000; i++ {
					wwg.Add(1)
					workScheduler.Push(
						ShortWork(func() error {
							// Task logic here
							defer func() {
								wwg.Done()
							}()
							return nil
						}),
					)
				}
				fmt.Println(getSystemResourceUsage())
			}()
		}

		wwg.Wait() // Wait for all tasks to complete
		time.Sleep(time.Second * 1)
		fmt.Printf("processed %v\n", metricProcessedCount)
	}

	return
	// this part can reach 30M msg processed per second on my machine
	runtime.GOMAXPROCS(runtime.NumCPU())
	coreMultipler := runtime.NumCPU() * 16

	fmt.Println(runtime.NumCPU(), coreMultipler)

	schedulers := make([]*Scheduler, coreMultipler)
	for i := 0; i < coreMultipler; i++ {
		schedulers[i] = NewScheduler(i, batchSize*coreMultipler, defaultThroughput) // Each scheduler can steal from any other
	}

	go reportProcessedCount() // Start reporting processed tasks

	// senders has to scale as much as the schedulers
	var wg sync.WaitGroup
	for idx := 0; idx < coreMultipler; idx++ {
		go func() {
			for i := 0; i < 1000000; i++ {
				wg.Add(1)
				schedulers[i%coreMultipler].Push(
					ShortWork(func() error {
						// Task logic here
						defer wg.Done()
						return nil
					}),
				)
			}
		}()
	}

	wg.Wait() // Wait for all tasks to complete
	time.Sleep(time.Second * 1)
}
