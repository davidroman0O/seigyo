package main

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// go test -v -timeout=7s -count=1 -run TestScheduler
func TestScheduler(t *testing.T) {
	duration := 5 * time.Second

	var pushCount, popCount int

	basic := []int{}
	for i := 0; i < 1024; i++ {
		basic = append(basic, i)
	}

	// Pushing in a separate goroutine
	go func() {
		for !IsReady() {
			time.Sleep(100 * time.Microsecond)
		}
		fmt.Println("send")

		var wg sync.WaitGroup

		pushN := true

		// this is simulating senders
		coreMultipler := runtime.NumCPU()
		if pushN {

			for idx := 0; idx < coreMultipler; idx++ {
				go func() {
					for {
						basic := []AnonymousTask{}
						for i := 0; i < 1024; i++ {
							wg.Add(1)
							basic = append(basic, ShortWork(func() error {
								defer wg.Done()
								popCount++
								return nil
							}))
						}
						pushCount += len(basic)
						DispatchN(basic)
					}
				}()
			}
		} else {
			for idx := 0; idx < coreMultipler; idx++ {
				go func() {
					for {

						wg.Add(1)
						pushCount++
						Dispatch(
							ShortWork(func() error {
								defer wg.Done()
								popCount++
								return nil
							}),
						)

					}
				}()
			}
		}
	}()

	go Run(
		WithDefaultThroughput(1024),
	)

	time.Sleep(duration)

	Stop()

	// buffer_test.go:73: Push operations: 116968448
	// buffer_test.go:74: Pop operations: 109495776
	// buffer_test.go:75: Push throughput: 58484224 ops/sec
	// buffer_test.go:76: Pop throughput: 54747888 ops/sec

	t.Logf("Push operations: %d", pushCount)
	t.Logf("Pop operations: %d", popCount)
	t.Logf("Push throughput: %d ops/sec", pushCount/int(duration.Seconds()))
	t.Logf("Pop throughput: %d ops/sec", popCount/int(duration.Seconds()))
}
