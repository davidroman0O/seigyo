package process

import (
	"fmt"
	"runtime"
	"time"

	"github.com/davidroman0O/seigyo/playground/stackheap/buffer"
	"github.com/davidroman0O/seigyo/playground/stackheap/pid"
)

/// It would be ridiculous to have one `goroutine` per actor lmao
/// I'm going to observe the inboxes and trigger the actors
/// In my mind, one Actor contains multiple processors to ease the development (cf `tellask` test syntax)
/// todo more, too tired right now to continue
/// you can't make the actor-model arch without workers
/// it is efficient to pass objects between goroutines into hierarchial buffers, Accumulators will be useful to pass batched data every x micro

// supposedly the instance of an actor
// it got it's own list and a copy of the func
type Processor struct {
	pid.PID
	*buffer.HierarchicalBuffer[interface{}]
	call interface{}
}

func NewProcessor(name string, fn interface{}) *Processor {
	p := &Processor{
		PID:                *pid.NewPID(nil, name),
		HierarchicalBuffer: buffer.NewHierarchicalBuffer[interface{}](1024, 8),
		call:               fn,
	}
	return p
}

type WorkerProcessor struct {
	Processors map[pid.PID]Processor
}

func NewWorker() *WorkerProcessor {
	w := &WorkerProcessor{
		Processors: map[pid.PID]Processor{},
	}
	go func() {
		ticker := time.NewTicker(1 * time.Microsecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for i := 0; i < len(w.Processors); i++ {

				}
				runtime.Gosched()
				// case <-e.Context.Done():
				// 	return // Exit the function when context is cancelled
			}
		}
	}()
	return w
}

func (w *WorkerProcessor) AddProcessor(p Processor) error {
	if _, ok := w.Processors[p.PID]; ok {
		return fmt.Errorf("already exists")
	}
	w.Processors[p.PID] = p
	return nil
}
