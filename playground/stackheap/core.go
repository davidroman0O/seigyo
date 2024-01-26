package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
)

/// `core` is the central piece that organize the schedulers
/// i don't think we need a "core" struct since it is 100% related to the runtime

var schedulers []*Scheduler
var totalSchedulers int32

// global throughput
var metricThrouput int64

var dispatcher int32

// coreContext and cancelFunc to manage goroutines lifecycle
var coreContext context.Context
var cancelFunc context.CancelFunc

var stopSignal chan os.Signal

var config configuration

func coreMetricsPerSecond(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			count := atomic.LoadInt64(&metricThrouput)
			fmt.Printf("Tasks processed in the last second: %d\n", count)
			atomic.StoreInt64(&metricThrouput, 0) // Reset the counter
		case <-ctx.Done():
			return // Exit the function when context is cancelled
		}
	}
}

func init() {
	coreContext, cancelFunc = context.WithCancel(context.Background())

	// Setting up signal handling
	// this should be optional
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, syscall.SIGTERM)

	config = configuration{
		queueSize:  1024 * 4 * runtime.NumCPU(),
		throughput: 1024,
	}

	// prepare schedulers
	schedulers = []*Scheduler{}

	addScheduler(config.queueSize)

	go func() {
		<-stopSignal
		cancelFunc() // Cancel the context when signal is received
	}()

	go coreMetricsPerSecond(coreContext)
}

type configuration struct {
	throughput int
	queueSize  int
}

type seigyoConfiguration func(c *configuration) error

func WithDefaultQueueSize(queueSize int) seigyoConfiguration {
	return func(c *configuration) error {
		c.queueSize = queueSize
		return nil
	}
}

func WithDefaultThroughput(throughput int) seigyoConfiguration {
	return func(c *configuration) error {
		c.throughput = throughput
		return nil
	}
}

func IsReady() bool {
	return len(schedulers) > 0
}

// it's the main `wait` function
func Run(opts ...seigyoConfiguration) error {
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return err
		}
	}

	// TODO @droman: we should have an automatic scaler to adjust all the numbers
	// coreMultipler := runtime.NumCPU()
	for i := 0; i < 20; i++ {
		addScheduler(config.queueSize)
	}

	<-coreContext.Done() // Wait here until the context is cancelled

	return nil
}

func Stop() {
	cancelFunc()
	if stopSignal != nil {
		close(stopSignal)
	}
}

func addScheduler(queueSize int) {
	schedulers = append(schedulers, NewScheduler(queueSize, 8, 1024*4))
	atomic.AddInt32(&totalSchedulers, 1)
}

// Add a new task to the `core`s
func Dispatch(task AnonymousTask) {
	total := atomic.LoadInt32(&totalSchedulers)
	if atomic.LoadInt32(&dispatcher) < total {
		atomic.AddInt32(&dispatcher, 1)
	} else {
		atomic.StoreInt32(&dispatcher, 1)
	}
	schedulers[int(atomic.LoadInt32(&dispatcher)%total)].
		Push(task)
}

func DispatchN(tasks []AnonymousTask) {
	total := atomic.LoadInt32(&totalSchedulers)
	if atomic.LoadInt32(&dispatcher) < total {
		atomic.AddInt32(&dispatcher, 1)
	} else {
		atomic.StoreInt32(&dispatcher, 1)
	}
	schedulers[int(atomic.LoadInt32(&dispatcher)%total)].
		PushN(tasks)
}
