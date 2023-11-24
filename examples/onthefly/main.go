package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/davidroman0O/seigyo"
)

type Global struct{}

func main() {

	ctrl := seigyo.New[*Global, interface{}](&Global{})
	ctrl.RegisterProcess("basic", seigyo.ProcessConfig[*Global, interface{}]{
		Process: &BasicProcess{pid: "basic"},
	})

	errCh := ctrl.Start()

	go func() {
		// Add and start a new process on the fly.
		if err := ctrl.AddAndStart("newProcess", seigyo.ProcessConfig[*Global, interface{}]{
			Process: &BasicProcess{pid: "newProcess"},
		}); err != nil {
			fmt.Println("Error starting new process:", err)
		}
	}()

	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(time.Millisecond * 150)
			shutdownSent, stopped, errored, err := ctrl.State("newProcess")
			if err != nil {
				fmt.Println("Error getting state of process:", err)
				return
			}
			fmt.Println(shutdownSent, stopped, errored)
		}
	}()

	go func() {
		time.Sleep(time.Second * 1)
		// Stop an individual process.
		if err := ctrl.StopProcess("newProcess"); err != nil {
			fmt.Println("Error stopping process:", err)
		}
	}()

	for err := range errCh {
		fmt.Println(err)
	}

	ctrl.Stop()
	shutdownSent, stopped, errored, err := ctrl.State("newProcess")
	if err != nil {
		fmt.Println("Error getting state of process:", err)
		return
	}
	fmt.Println(shutdownSent, stopped, errored)
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
		time.Sleep(time.Millisecond * 150)
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
