package main

import (
	"context"
	"fmt"
	"log"

	"github.com/davidroman0O/seigyo"
)

type Global struct{}

func main() {

	ctrl := seigyo.New(&Global{})
	ctrl.RegisterProcess("basic", seigyo.ProcessConfig[*Global]{
		Process: &BasicProcess{pid: "basic"},
	})

	errCh := ctrl.Start()

	for err := range errCh {
		fmt.Println(err)
	}

	ctrl.Stop()
}

// ExampleProcess is a simple implementation of the Process interface.
type BasicProcess struct {
	pid string
}

// Init initializes the process.
func (p *BasicProcess) Init(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{})) error {
	log.Println(p.pid, "initializing")
	return nil
}

func (p *BasicProcess) Run(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{}), shutdownCh chan struct{}, errCh chan<- error, selfShutdown func()) error {
	log.Println(p.pid, "running")

	for i := 0; i < 5; i++ {
		log.Println(p.pid, "doing work", i)
	}

	select {
	case <-shutdownCh:
		log.Println(p.pid, "received shutdown signal")
	case <-ctx.Done():
		log.Println(p.pid, "context done")
	default:
		log.Println(p.pid, "work done, exiting")
	}

	log.Println(p.pid, "finished")

	return nil
}

// Deinit deinitializes the process.
func (p *BasicProcess) Deinit(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{})) error {
	log.Println(p.pid, "deinitializing")
	return nil
}

// Received handles data received from another process.
func (p *BasicProcess) Received(pid string, data interface{}) error {
	log.Println(p.pid, "received data from", pid, ":", data)
	return nil
}
