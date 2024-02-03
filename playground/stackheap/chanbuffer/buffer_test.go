package chanbuffer

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// go test -v -timeout=30s -count=1 -run TestLaneBuffer
// buffer_test.go:85: Push operations: 2993054720
// buffer_test.go:86: Pop operations: 1011866624
// buffer_test.go:87: Push throughput: 598631219 ops/sec
// buffer_test.go:88: Pop throughput: 202374144 ops/sec
// update:
// buffer_test.go:87: Push operations: 5171462144
// buffer_test.go:88: Pop operations: 1883049984
// buffer_test.go:89: Push throughput: 1034341580 ops/sec
// buffer_test.go:90: Pop throughput: 376617984 ops/sec
func TestLaneBuffer(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	// bufferSize := 1024
	// buffers := 4
	duration := 5 * time.Second
	var pushCount, popCount int

	basic := []interface{}{}
	for i := 0; i < 1024; i++ {
		basic = append(basic, i)
	}

	// trigger := make(chan bool)
	var wg sync.WaitGroup
	laneB := NewLaneBuffer[any](1)
	now := time.Now()
	// go func() {
	for i := 0; i < runtime.NumCPU()*100; i++ {
		wg.Add(1)
		for i := 0; i < runtime.NumCPU(); i++ {
			go func() {
				defer wg.Done()
				for i := 0; i < 1024; i++ {
					laneB.Push(i)
					pushCount++
				}
			}()
		}
	}
	wg.Wait()
	fmt.Println("close it")
	laneB.Close()
	t.Logf("Push time: %v", time.Since(now).Microseconds())

	// }()

	// time.Sleep(duration)
	// Trigger the channel
	// trigger <- true

	t.Logf("Push operations: %d", pushCount)
	t.Logf("Pop operations: %d", popCount)
	t.Logf("Push throughput: %d ops/sec", pushCount/int(duration.Seconds()))
	t.Logf("Pop throughput: %d ops/sec", popCount/int(duration.Seconds()))
}
