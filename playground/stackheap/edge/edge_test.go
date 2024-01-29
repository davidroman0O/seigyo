package edge

import (
	"runtime"
	"testing"
	"time"
)

/// I will remove the `edge` package, it was a good experiment to understand how i can optmize it

// go test -v -timeout=7s -count=1 -run TestEdging
func TestEdging(t *testing.T) {

	runtime.GOMAXPROCS(runtime.NumCPU())
	duration := 5 * time.Second
	var pushCount, popCount int

	ed := NewEdge[int]()

	basic := []int{}
	for i := 0; i < 1024; i++ {
		basic = append(basic, i)
	}
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for {
				ed.in <- basic
				pushCount += len(basic)
			}
		}()
	}

	go func() {
		for {
			popCount += len(<-ed.out)
		}
	}()

	time.Sleep(duration)
	t.Logf("Push operations: %d", pushCount)
	t.Logf("Pop operations: %d", popCount)
	t.Logf("Push throughput: %d ops/sec", pushCount/int(duration.Seconds()))
	t.Logf("Pop throughput: %d ops/sec", popCount/int(duration.Seconds()))

}

// go test -v -timeout=10s -count=1 -run TestMultipleEdging
func TestMultipleEdging(t *testing.T) {

	runtime.GOMAXPROCS(runtime.NumCPU())
	duration := 5 * time.Second
	var pushCount, popCount int
	trigger := make(chan bool)

	// What i can learn from this one is that I don't need to have BIG LANES of buffers
	// I can just have many smaller lanes of buffers that will be used to dispatch messages
	// It's faster than i expected
	for i := 0; i < 2000; i++ {
		ed := NewEdge[int]()
		basic := []int{}
		for i := 0; i < 1024; i++ {
			basic = append(basic, i)
		}
		for i := 0; i < runtime.NumCPU(); i++ {
			go func() {
				for {
					select {
					case <-trigger:
						// Triggered, do something
						return
					default:
						ed.in <- basic
						// it seems that a for loop isn't disturbing it too much
						for i, v := range basic {
							basic[i] += v
						}
						pushCount += len(basic)
					}
				}
			}()
		}
		go func() {
			for {
				select {
				case <-trigger:
					// Triggered, do something
					return
				default:
					popCount += len(<-ed.out)
					// if time.Since(start) >= duration {
					// 	// tickerPop.Stop()
					// 	break
					// }
				}
			}
		}()
	}

	time.Sleep(duration)

	// Trigger the channel
	trigger <- true

	t.Logf("Push operations: %d", pushCount)
	t.Logf("Pop operations: %d", popCount)
	t.Logf("Push throughput: %d ops/sec", pushCount/int(duration.Seconds()))
	t.Logf("Pop throughput: %d ops/sec", popCount/int(duration.Seconds()))
}

// go test -v -timeout=7s -count=1 -run TestAccumulator
func TestAccumulator(t *testing.T) {

	runtime.GOMAXPROCS(runtime.NumCPU())
	duration := 5 * time.Second
	var pushCount, popCount int

	ed := NewAccumulator[int]()

	basic := []int{}
	for i := 0; i < 1024; i++ {
		basic = append(basic, i)
	}

	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for {
				ed.in <- basic
				pushCount += len(basic)
			}
		}()
	}

	go func() {
		for {
			popCount += len(<-ed.out)
		}
	}()

	time.Sleep(duration)
	t.Logf("Push operations: %d", pushCount)
	t.Logf("Pop operations: %d", popCount)
	t.Logf("Push throughput: %d ops/sec", pushCount/int(duration.Seconds()))
	t.Logf("Pop throughput: %d ops/sec", popCount/int(duration.Seconds()))

}

// go test -v -timeout=10s -count=1 -run TestMultipleAccumulator
func TestMultipleAccumulator(t *testing.T) {

	runtime.GOMAXPROCS(runtime.NumCPU())
	duration := 5 * time.Second
	var pushCount, popCount int

	for i := 0; i < 2000; i++ {
		ed := NewAccumulator[int]()
		basic := []int{}
		for i := 0; i < 1024; i++ {
			basic = append(basic, i)
		}
		for i := 0; i < runtime.NumCPU(); i++ {
			go func() {
				for {
					ed.in <- basic
					pushCount += len(basic)
				}
			}()
		}
		go func() {
			for {
				popCount += len(<-ed.out)
			}
		}()
	}

	time.Sleep(duration)
	t.Logf("Push operations: %d", pushCount)
	t.Logf("Pop operations: %d", popCount)
	t.Logf("Push throughput: %d ops/sec", pushCount/int(duration.Seconds()))
	t.Logf("Pop throughput: %d ops/sec", popCount/int(duration.Seconds()))
}
