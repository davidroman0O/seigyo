package buffer

import (
	"sync/atomic"
	"testing"
	"time"
)

// go test -bench=BenchmarkPushPop

func BenchmarkPushPop(b *testing.B) {
	buffer := NewHierarchicalBuffer[int](1024, 1) // Adjust size as needed

	// Run the benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer.Push(i)
		_, _ = buffer.Pop()
	}
}

func TestBuffer(t *testing.T) {
	buffer := NewHierarchicalBuffer[int](1024, 1) // Adjust buffer size and initial buffers as needed
	duration := 1 * time.Second

	done := make(chan bool)
	var pushCount, popCount int

	basic := []int{}
	for i := 0; i < 1024; i++ {
		basic = append(basic, i)
	}

	// Pushing in a separate goroutine
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				if err := buffer.PushN(basic); err != nil {
					t.Log("Push error:", err)
					continue
				}
				pushCount += len(basic)
			}
		}
	}()

	start := time.Now()
	for {
		if !buffer.HasMessages() {
			continue
		}
		items, err := buffer.PopN(1014 * 16)
		if err != nil {
			t.Log("Pop error:", err)
			if time.Since(start) >= duration {
				break
			}
			continue
		}
		popCount += len(items)
		// fmt.Println(time.Since(start) >= duration)
		if time.Since(start) >= duration {
			break
		}
	}

	close(done) // Signal to stop pushing

	// buffer_test.go:62: Push operations: 16 299 446
	// buffer_test.go:63: Pop operations: 14 420 096
	// buffer_test.go:64: Push throughput: 3 259 889 ops/sec
	// buffer_test.go:65: Pop throughput: 2 884 019 ops/sec
	t.Logf("Push operations: %d", pushCount)
	t.Logf("Pop operations: %d", popCount)
	t.Logf("Push throughput: %d ops/sec", pushCount/int(duration.Seconds()))
	t.Logf("Pop throughput: %d ops/sec", popCount/int(duration.Seconds()))
}

func TestThroughput(t *testing.T) {
	buffer := NewHierarchicalBuffer[int](1024, 1)
	var pushCount, popCount int64

	basic := []int{}
	for i := 0; i < 1024; i++ {
		basic = append(basic, i)
	}

	// Start pushing and popping in separate goroutines
	go func() {
		for {
			buffer.PushN(basic) // Omitting error handling for brevity
			atomic.AddInt64(&pushCount, int64(len(basic)))
			time.Sleep(time.Millisecond) // Adjust the sleep duration to control the rate
		}
	}()

	go func() {
		for {
			items, _ := buffer.PopN(1024 * 16) // Omitting error handling for brevity
			atomic.AddInt64(&popCount, int64(len(items)))
			time.Sleep(time.Millisecond) // Adjust the sleep duration to control the rate
		}
	}()

	// Run the test for a specified duration
	testDuration := time.Second
	time.Sleep(testDuration)

	// Stop the goroutines by closing the buffer or using a context (omitted for brevity)

	// Calculate and output the throughput
	pushThroughput := float64(pushCount) / testDuration.Seconds()
	popThroughput := float64(popCount) / testDuration.Seconds()

	t.Logf("Push Throughput: %f ops/sec", pushThroughput)
	t.Logf("Pop Throughput: %f ops/sec", popThroughput)
}
