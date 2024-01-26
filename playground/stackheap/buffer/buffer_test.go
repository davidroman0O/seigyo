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
	duration := 2 * time.Second

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
					time.Sleep(1 * time.Millisecond) // Briefly sleep if no messages
					continue
				}
				pushCount += len(basic)
			}
		}
	}()

	start := time.Now()
	for {
		if !buffer.HasMessages() {
			time.Sleep(1 * time.Millisecond) // Briefly sleep if no messages
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
		if time.Since(start) >= duration {
			break
		}
	}

	close(done) // Signal to stop pushing

	// buffer_test.go:73: Push operations: 116968448
	// buffer_test.go:74: Pop operations: 109495776
	// buffer_test.go:75: Push throughput: 58484224 ops/sec
	// buffer_test.go:76: Pop throughput: 54747888 ops/sec

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

// func (b *Buffer[T]) PopN(n int) ([]T, error) {
// 	b.mu.Lock()
// 	defer b.mu.Unlock()

// 	var items []T
// 	for i := 0; i < n; i++ {
// 		if !b.full && b.head == b.tail {
// 			// Stop if the buffer is empty
// 			break
// 		}
// 		item := b.items[b.head]
// 		b.items[b.head] = *new(T) // Zero the item (optional, for garbage collection)
// 		b.head = (b.head + 1) % b.maxSize
// 		b.full = false
// 		items = append(items, item)
// 	}

// 	if len(items) == 0 {
// 		return nil, errors.New("buffer is empty")
// 	}

//		return items, nil
//	}
// func (b *Buffer[T]) PopN(n int) ([]T, error) {
// 	b.mu.Lock()
// 	defer b.mu.Unlock()

// 	if !b.full && b.head == b.tail {
// 		return nil, errors.New("buffer is empty") // No items to pop
// 	}

// 	var items []T
// 	for i := 0; i < n && (b.full || b.head != b.tail); i++ {
// 		item := b.items[b.head]
// 		b.head = (b.head + 1) % b.maxSize
// 		b.full = false
// 		items = append(items, item)
// 	}

// 	return items, nil
// }
// func (hb *HierarchicalBuffer[T]) PopN(n int) ([]T, error) {
// 	var items []T

// 	hb.mu.Lock()
// 	defer hb.mu.Unlock()

// 	start := hb.readIndex
// 	for n > 0 {
// 		buffer := hb.buffers[hb.readIndex]
// 		popped, err := buffer.PopN(n)
// 		if err == nil {
// 			items = append(items, popped...)
// 			n -= len(popped)
// 		}

// 		// Move to next buffer
// 		hb.readIndex = (hb.readIndex + 1) % len(hb.buffers)

// 		// If we have checked all buffers and found no items, break
// 		if hb.readIndex == start && len(items) == 0 {
// 			return nil, errors.New("all buffers are empty")
// 		}
// 	}

//		return items, nil
//	}
// func (hb *HierarchicalBuffer[T]) PopN(n int) ([]T, error) {
// 	hb.mu.Lock()
// 	defer hb.mu.Unlock()

// 	var items []T
// 	start := hb.readIndex
// 	for n > 0 {
// 		buffer := hb.buffers[hb.readIndex]
// 		popped, err := buffer.PopN(n)
// 		if err == nil && len(popped) > 0 {
// 			items = append(items, popped...)
// 			n -= len(popped)
// 		}

// 		hb.readIndex = (hb.readIndex + 1) % len(hb.buffers)

// 		if hb.readIndex == start && len(items) == 0 {
// 			return nil, errors.New("all buffers are empty")
// 		}
// 	}

// 	return items, nil
// }

// func (hb *HierarchicalBuffer[T]) PopN(n int) ([]T, error) {
// 	var items []T

// 	hb.mu.Lock()
// 	defer hb.mu.Unlock()

// 	for n > 0 && hb.readIndex < len(hb.buffers) {
// 		popped, err := hb.buffers[hb.readIndex].PopN(n)
// 		if err != nil {
// 			hb.readIndex++
// 			continue
// 		}
// 		items = append(items, popped...)
// 		n -= len(popped)
// 		if n <= 0 {
// 			break
// 		}
// 	}

// 	if len(items) == 0 {
// 		return nil, errors.New("no items popped")
// 	}

// 	return items, nil
// }

// func main() {
// 	// Example usage
// 	hBuffer := NewHierarchicalBuffer[int](5, 1) // Buffers of size 5, initially 1 buffer

// 	for i := 0; i < 10; i++ {
// 		hBuffer.Push(i)
// 	}

// 	for i := 0; i < 10; i++ {
// 		item, err := hBuffer.Pop()
// 		if err != nil {
// 			panic(err)
// 		}
// 		println(item)
// 	}
// }
