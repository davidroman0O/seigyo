package pcbuffer

import (
	"runtime"
	"testing"
	"time"
)

// go test -v -timeout=30s -count=1 -run TestBuffer
func TestBuffer(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	bufferSize := 2048
	// buffers := 4
	duration := 5 * time.Second
	var pushCount, popCount int

	bufferSegment := NewBufferSegment[interface{}](bufferSize)

	bufferSegment.pushSegment(1)

	t.Logf("Push operations: %d", pushCount)
	t.Logf("Pop operations: %d", popCount)
	t.Logf("Push throughput: %d ops/sec", pushCount/int(duration.Seconds()))
	t.Logf("Pop throughput: %d ops/sec", popCount/int(duration.Seconds()))
}
