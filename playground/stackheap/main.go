package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

///////////////////////////////////

// type Unit struct {
// 	value int64
// }

// func (a *Unit) Increment() {
// 	atomic.AddInt64(&a.value, 1)
// }

// func (a *Unit) Set(value int64) {
// 	atomic.StoreInt64(&a.value, value)
// }

// func (a *Unit) Get() int64 {
// 	return atomic.LoadInt64(&a.value)
// }

// type DynamicMetric struct {
// 	mutex   sync.RWMutex
// 	metrics []Unit
// }

// func NewDynamicMetric(initialSize int) *DynamicMetric {
// 	metrics := make([]Unit, initialSize)
// 	for i := range metrics {
// 		metrics[i] = Unit{}
// 	}
// 	return &DynamicMetric{
// 		metrics: metrics,
// 	}
// }

// func (d *DynamicMetric) AddMetric() int {
// 	d.mutex.Lock()
// 	defer d.mutex.Unlock()
// 	d.metrics = append(d.metrics, Unit{})
// 	return len(d.metrics) - 1
// }

// func (d *DynamicMetric) RemoveMetric(index int) {
// 	d.mutex.Lock()
// 	defer d.mutex.Unlock()
// 	d.metrics = append(d.metrics[:index], d.metrics[index+1:]...)
// }

// func (d *DynamicMetric) GetMetric(index int) *Unit {
// 	d.mutex.RLock()
// 	defer d.mutex.RUnlock()
// 	if index < 0 || index >= len(d.metrics) {
// 		return nil
// 	}
// 	return &d.metrics[index]
// }

// func (d *DynamicMetric) SetMetric(index int, value int64) bool {
// 	d.mutex.RLock()
// 	defer d.mutex.RUnlock()
// 	if index < 0 || index >= len(d.metrics) {
// 		return false
// 	}
// 	d.metrics[index].Set(value)
// 	return true
// }

// type Metrics struct {
// 	Throughput           *DynamicMetric
// 	SchedulersThroughput *DynamicMetric
// 	SchedulersQueueSize  *DynamicMetric
// 	CPUs                 *DynamicMetric
// 	RAM                  *DynamicMetric
// }

// func newMetrics(numSchedulers, initialCPUs int) *Metrics {
// 	return &Metrics{
// 		Throughput:           NewDynamicMetric(1),
// 		RAM:                  NewDynamicMetric(1),
// 		SchedulersThroughput: NewDynamicMetric(numSchedulers),
// 		SchedulersQueueSize:  NewDynamicMetric(numSchedulers),
// 		CPUs:                 NewDynamicMetric(initialCPUs),
// 	}
// }

// var metrics *Metrics
// var metricsTicker *time.Ticker
// var metricsContext context.Context

// func init() {
// 	metrics = newMetrics(1, runtime.NumCPU())
// 	metricsContext = context.Background()
// 	go trackMetrics()

// }

// func trackMetrics() {
// 	metricsTicker = time.NewTicker(time.Millisecond * 50)
// 	for {
// 		select {
// 		case <-metricsTicker.C:
// 			cpus, _, err := getSystemResourceUsage()
// 			if err != nil {
// 				continue
// 			}
// 			for idx, cpu := range cpus {
// 				metrics.CPUs.SetMetric(idx, int64(cpu))
// 			}
// 		case <-metricsContext.Done():
// 			metricsTicker.Stop()
// 			fmt.Println("bye bye")
// 			return
// 		}
// 	}
// }

// func stop() {
// 	metricsContext.Done()
// }

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// currently workingn on a resources and throughput based scaler
	{
		// workScheduler := NewSchedulerManager(200, 80)

		// go workScheduler.MonitorResources()

		// go reportProcessedCount() // Start reporting processed tasks

		// // senders has to scale as much as the schedulers
		// var wwg sync.WaitGroup
		// for idx := 0; idx < workScheduler.Len(); idx++ {
		// 	wwg.Add(1)
		// 	go func() {
		// 		defer func() {
		// 			fmt.Println("done")
		// 			wwg.Done()
		// 		}()
		// 		for i := 0; i < 30000000; i++ {
		// 			wwg.Add(1)
		// 			workScheduler.Push(
		// 				ShortWork(func() error {
		// 					// Task logic here
		// 					defer func() {
		// 						wwg.Done()
		// 					}()
		// 					return nil
		// 				}),
		// 			)
		// 		}
		// 		fmt.Println(getSystemResourceUsage())
		// 	}()
		// }

		// wwg.Wait() // Wait for all tasks to complete
		// time.Sleep(time.Second * 1)
		// fmt.Printf("processed %v\n", metricProcessedCount)
		// return
	}

	fmt.Println("start")

	// why do it have drop compared to `simpleScheduler`?
	go func() {
		// this is simulating senders
		coreMultipler := runtime.NumCPU() * 200
		var wg sync.WaitGroup
		for idx := 0; idx < coreMultipler; idx++ {
			go func() {
				for i := 0; i < 1000000; i++ {
					wg.Add(1)

					Dispatch(
						ShortWork(func() error {
							defer wg.Done()
							return nil
						}),
					)

				}
			}()
		}
		wg.Wait()
	}()

	Run(
		WithDefaultThroughput(1024),
	)

	fmt.Println("stop")
}

// this part can reach 30M msg processed per second on my machine
func simpleSchedulers() {
	coreMultipler := runtime.NumCPU() * 16

	fmt.Println(runtime.NumCPU(), coreMultipler)

	schedulers := make([]*Scheduler, coreMultipler)

	for idx := 0; idx < coreMultipler; idx++ {
		schedulers[idx] = NewScheduler(batchSize*coreMultipler, defaultThroughput) // Each scheduler can steal from any other
	}

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

// type WorkScheduler struct {
// 	Schedulers    []*Scheduler
// 	coreMultipler int
// 	state         int32
// 	dispatcher    int32 // atomic modulo dispatcher to different schedulers
// }

// func (w *WorkScheduler) Len() int {
// 	return len(w.Schedulers)
// }

// func (w *WorkScheduler) Push(task AnonymousTask) {
// 	total := len(w.Schedulers)
// 	if atomic.LoadInt32(&w.dispatcher) < int32(total) {
// 		atomic.AddInt32(&w.dispatcher, 1)
// 	} else {
// 		atomic.StoreInt32(&w.dispatcher, 1)
// 	}
// 	w.Schedulers[int(atomic.LoadInt32(&w.dispatcher))%total].Push(task)
// }

// func (w *WorkScheduler) scale() {
// 	for {
// 		ticker := time.NewTicker(time.Microsecond * 150)
// 		select {
// 		case <-ticker.C:
// 			// stop everything when asked
// 			if atomic.LoadInt32(&w.state) == stateStopped {
// 				for _, s := range w.Schedulers {
// 					s.Stop()
// 				}
// 				ticker.Stop()
// 				return
// 			}
// 			// Perform your desired actions here
// 		}
// 	}
// }

// func NewWorkScheduler() *WorkScheduler {
// 	runtime.GOMAXPROCS(runtime.NumCPU())
// 	ws := WorkScheduler{
// 		Schedulers:    make([]*Scheduler, 3),
// 		coreMultipler: runtime.NumCPU() * 1,
// 		dispatcher:    1,
// 	}
// 	go ws.scale()
// 	// we start with only one
// 	ws.Schedulers[0] = NewScheduler(0, batchSize*ws.coreMultipler, defaultThroughput)
// 	ws.Schedulers[1] = NewScheduler(1, batchSize*ws.coreMultipler, defaultThroughput)
// 	ws.Schedulers[2] = NewScheduler(2, batchSize*ws.coreMultipler, defaultThroughput)
// 	return &ws
// }

// type SchedulerManager struct {
// 	schedulers        []*Scheduler
// 	resourceThreshold float64
// 	maxSchedulers     int
// 	mu                sync.Mutex
// 	dispatcher        int32 // atomic modulo dispatcher to different schedulers
// }

// func NewSchedulerManager(maxSchedulers int, resourceThreshold float64) *SchedulerManager {
// 	sm := &SchedulerManager{
// 		schedulers:        make([]*Scheduler, 1, maxSchedulers),
// 		resourceThreshold: resourceThreshold,
// 		maxSchedulers:     maxSchedulers,
// 	}
// 	sm.schedulers[0] = NewScheduler(0, batchSize*batchSize*(runtime.NumCPU()*16), defaultThroughput)
// 	return sm
// }

// func (sm *SchedulerManager) MonitorResources() {
// 	for {
// 		ticker := time.NewTicker(time.Millisecond * 50)
// 		select {
// 		case <-ticker.C:
// 			cpuPercents, memUsage, err := getSystemResourceUsage()
// 			if err != nil {
// 				// Handle the error appropriately
// 				continue
// 			}
// 			isCPUOverloaded := checkCPUOverload(cpuPercents)
// 			isMemoryOverloaded := memUsage > sm.resourceThreshold

// 			sm.mu.Lock()
// 			if isCPUOverloaded || isMemoryOverloaded {
// 				if len(sm.schedulers) > 1 {
// 					// Logic to scale down schedulers
// 					schedulerToRemove := sm.schedulers[len(sm.schedulers)-1]
// 					schedulerToRemove.Stop()
// 					sm.schedulers = sm.schedulers[:len(sm.schedulers)-1]
// 				}
// 			} else if len(sm.schedulers) < sm.maxSchedulers {
// 				// Logic to scale up schedulers
// 				newScheduler := NewScheduler(len(sm.schedulers)+1, batchSize*(runtime.NumCPU()*16), defaultThroughput)
// 				sm.schedulers = append(sm.schedulers, newScheduler)
// 			}
// 			sm.mu.Unlock()
// 		}
// 	}
// }

// func (w *SchedulerManager) Len() int {
// 	return len(w.schedulers)
// }

// func (w *SchedulerManager) Push(task AnonymousTask) {
// 	total := len(w.schedulers)
// 	if atomic.LoadInt32(&w.dispatcher) < int32(total) {
// 		atomic.AddInt32(&w.dispatcher, 1)
// 	} else {
// 		atomic.StoreInt32(&w.dispatcher, 1)
// 	}
// 	w.schedulers[int(atomic.LoadInt32(&w.dispatcher))%total].Push(task)
// }
