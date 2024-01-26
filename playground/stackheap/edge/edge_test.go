package edge

import (
	"runtime"
	"testing"
	"time"
)

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

	for i := 0; i < 2000; i++ {
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
	}

	time.Sleep(duration)
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
