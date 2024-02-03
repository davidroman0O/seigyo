package buffer

import (
	"runtime"
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

// go test -v -timeout=30s -count=1 -run TestBufferProducerConsumer
// buffer_test.go:85: Push operations: 2993054720
// buffer_test.go:86: Pop operations: 1011866624
// buffer_test.go:87: Push throughput: 598631219 ops/sec
// buffer_test.go:88: Pop throughput: 202374144 ops/sec
// update:
// buffer_test.go:87: Push operations: 5171462144
// buffer_test.go:88: Pop operations: 1883049984
// buffer_test.go:89: Push throughput: 1034341580 ops/sec
// buffer_test.go:90: Pop throughput: 376617984 ops/sec
func TestBufferProducerConsumer(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	bufferSize := 1024
	buffers := 4
	duration := 5 * time.Second
	var pushCount, popCount int

	basic := []interface{}{}
	for i := 0; i < 1024; i++ {
		basic = append(basic, i)
	}

	trigger := make(chan bool)

	mpmc := []*ProducerConsumer[any]{}

	// I conclude that i need to have multiple smaller buffers (of buffers) which produce and consume
	// I should now wrap that concept into another wrapper of PC that can increase and decrease
	// Also it should have channels to dynamically adjust depending of the op/s

	// 16 core and 32 logical
	for idx := 0; idx < runtime.NumCPU()*100; idx++ {
		buffer := NewProducerConsumer[any](bufferSize, buffers, bufferSize*2)
		mpmc = append(mpmc, buffer)
		for i := 0; i < runtime.NumCPU(); i++ {
			// producer
			go func(pc *ProducerConsumer[any]) {
				for {
					select {
					case <-trigger:
						// Triggered, do something
						return
					default:
						pc.producer <- basic
						pushCount += len(basic)
						runtime.Gosched()
					}
				}
			}(buffer)
		}
		for i := 0; i < runtime.NumCPU(); i++ {
			// consumer
			go func(pc *ProducerConsumer[any]) {
				for {
					select {
					case <-trigger:
						// Triggered, do something
						return
					default:
						popCount += len(<-pc.consumer)
						runtime.Gosched()
					}
				}
			}(buffer)
		}
	}

	time.Sleep(duration)
	// Trigger the channel
	trigger <- true

	t.Logf("Push operations: %d", pushCount)
	t.Logf("Pop operations: %d", popCount)
	t.Logf("Push throughput: %d ops/sec", pushCount/int(duration.Seconds()))
	t.Logf("Pop throughput: %d ops/sec", popCount/int(duration.Seconds()))
}

// go test -v -timeout=7s -count=1 -run TestBufferTicker
// It seems that using the microseconds + goshed is increasing my ops from 30M to 45M+
// I think I need to rethink my model to have an accumulator of msg in front of a buffer
// like 1 acc to x pushers that will be consumed by y actors
// I need to make a system that autoregulate their amount per core and per thoughput to avoid looping on empty messages
func TestBufferTicker(t *testing.T) {

	runtime.GOMAXPROCS(runtime.NumCPU())

	bufferSize := 1024
	buffers := 4

	buffer := NewHierarchicalBuffer[int](bufferSize, buffers) // Adjust buffer size and initial buffers as needed
	duration := 5 * time.Second

	var pushCount, popCount int

	basic := []int{}
	for i := 0; i < 1024; i++ {
		basic = append(basic, i)
	}

	pushN := true

	start := time.Now()
	// Pushing in a separate goroutine
	if pushN {
		for i := 0; i < runtime.NumCPU()*2; i++ {
			// faster
			go func() {
				ticker := time.NewTicker(1 * time.Microsecond)
				for {
					select {
					case <-ticker.C:
						if err := buffer.PushN(basic); err != nil {
							t.Log("Push error:", err)
							time.Sleep(1 * time.Millisecond) // Briefly sleep if no messages
							continue
						}
						pushCount += len(basic)
						if time.Since(start) >= duration {
							ticker.Stop()
							break
						}
						// runtime.Gosched()
					}
				}
			}()
		}
	} else {
		for i := 0; i < runtime.NumCPU()*2; i++ {
			go func() {
				ticker := time.NewTicker(1 * time.Microsecond)
				for {
					select {
					case <-ticker.C:
						if err := buffer.Push(1); err != nil {
							t.Log("Push error:", err)
							time.Sleep(1 * time.Millisecond) // Briefly sleep if no messages
							continue
						}
						pushCount++
						if time.Since(start) >= duration {
							ticker.Stop()
							break
						}
						runtime.Gosched()
					}
				}
			}()
		}
	}

	withoutElements := 0

	for i := 0; i < runtime.NumCPU()*1; i++ {
		go func() {
			for {
				tickerPop := time.NewTicker(1 * time.Microsecond)
				select {
				case <-tickerPop.C:
					if !buffer.HasMessages() {
						time.Sleep(1 * time.Microsecond) // Briefly sleep if no messages
						continue
					}
					items, err := buffer.PopN(bufferSize * 6)
					if err != nil {
						// t.Log("Pop error:", err)
						withoutElements++
						if time.Since(start) >= duration {
							tickerPop.Stop()
							break
						}
						continue
					}
					popCount += len(items)
					if time.Since(start) >= duration {
						tickerPop.Stop()
						break
					}
					// runtime.Gosched()
				}
			}
		}()
	}

	time.Sleep(duration)

	// buffer_test.go:73: Push operations: 116968448
	// buffer_test.go:74: Pop operations: 109495776
	// buffer_test.go:75: Push throughput: 58484224 ops/sec
	// buffer_test.go:76: Pop throughput: 54747888 ops/sec

	t.Logf("Nothing operations: %d", withoutElements)
	t.Logf("Push operations: %d", pushCount)
	t.Logf("Pop operations: %d", popCount)
	t.Logf("Push throughput: %d ops/sec", pushCount/int(duration.Seconds()))
	t.Logf("Pop throughput: %d ops/sec", popCount/int(duration.Seconds()))
}

// go test -v -timeout=7s -count=1 -run TestBuffer
func TestBuffer(t *testing.T) {

	runtime.GOMAXPROCS(runtime.NumCPU())

	bufferSize := 1024
	buffers := 4

	buffer := NewHierarchicalBuffer[int](bufferSize, buffers) // Adjust buffer size and initial buffers as needed
	duration := 5 * time.Second

	done := make(chan bool)
	var pushCount, popCount int

	basic := []int{}
	for i := 0; i < 1024; i++ {
		basic = append(basic, i)
	}

	pushN := true

	// Pushing in a separate goroutine
	if pushN {
		for i := 0; i < runtime.NumCPU(); i++ {
			// faster
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
		}
	} else {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					if err := buffer.Push(1); err != nil {
						t.Log("Push error:", err)
						time.Sleep(1 * time.Millisecond) // Briefly sleep if no messages
						continue
					}
					pushCount++
				}
			}
		}()
	}

	start := time.Now()
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for {
				if !buffer.HasMessages() {
					time.Sleep(1 * time.Millisecond) // Briefly sleep if no messages
					continue
				}
				items, err := buffer.PopN(bufferSize * 4)
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
		}()
	}

	time.Sleep(duration)
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
