package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// this part can reach 30M msg processed per second on my machine
func simpleSchedulers() {
	coreMultipler := runtime.NumCPU() * 16

	fmt.Println(runtime.NumCPU(), coreMultipler)

	schedulers := make([]*Scheduler, coreMultipler)

	for idx := 0; idx < coreMultipler; idx++ {
		schedulers[idx] = NewScheduler(1024, 8, 1024*4) // Each scheduler can steal from any other
	}

	// senders has to scale as much as the schedulers
	var wg sync.WaitGroup
	for idx := 0; idx < coreMultipler; idx++ {
		go func() {
			for i := 0; i < 1000000; i++ {
				wg.Add(1)
				schedulers[i%coreMultipler].
					Push(
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

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	fmt.Println("start")

	withCore := false

	if withCore {
		// // why do it have drop compared to `simpleScheduler`?
		go func() {
			for !IsReady() {
				time.Sleep(100 * time.Microsecond)
			}
			fmt.Println("send")

			var wg sync.WaitGroup

			// this is simulating senders
			coreMultipler := runtime.NumCPU()
			for idx := 0; idx < coreMultipler; idx++ {
				go func() {
					for {
						{
							// basic := []AnonymousTask{}
							// for i := 0; i < 1024; i++ {
							// 	wg.Add(1)
							// 	basic = append(basic, ShortWork(func() error {
							// 		defer wg.Done()
							// 		return nil
							// 	}))
							// }
						}

						{
							wg.Add(1)
							Dispatch(
								ShortWork(func() error {
									defer wg.Done()
									return nil
								}),
							)
						}
					}
				}()
			}
			wg.Wait()
		}()

		Run(
			WithDefaultQueueSize(1024),
			WithDefaultThroughput(1024*4),
		)
	} else {
		simpleSchedulers()
	}

	fmt.Println("stop")
}
