package process

import (
	"runtime"
	"testing"
	"time"
)

// go test -v -timeout=10s -count=1 -run TestProcess
func TestProcess(t *testing.T) {

	runtime.GOMAXPROCS(runtime.NumCPU())
	duration := 5 * time.Second
	var pushCount, popCount int

	// meh i will test later
	ed := NewProcessor[int]()

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
