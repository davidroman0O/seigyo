package seigyo

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"testing"
)

func TestBasicProcess(t *testing.T) {
	ctrl := New[*GlobalState, interface{}](&GlobalState{})

	ctrl.RegisterProcess("basic", ProcessConfig[*GlobalState, interface{}]{
		Process: &BasicProcess{},
	})

	errCh := ctrl.Start()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		<-sigCh // Wait for SIGINT
		ctrl.Stop()
		fmt.Println("stopped")
	}()

	for err := range errCh {
		fmt.Println(err)
	}
}

// GlobalState holds the global state of the application.
type GlobalState struct {
	ActiveProcesses int
}

// IncrementActiveProcesses is a function that increments the count of active processes.
// It's intended to be used with the stateMutator function provided by the Controller.
func IncrementActiveProcesses(currentState *GlobalState) *GlobalState {
	currentState.ActiveProcesses++
	return currentState
}

// DecrementActiveProcesses is a function that decrements the count of active processes.
// It's intended to be used with the stateMutator function provided by the Controller.
func DecrementActiveProcesses(currentState *GlobalState) *GlobalState {
	currentState.ActiveProcesses--
	return currentState
}

// ExampleProcess is a simple implementation of the Process interface.
type BasicProcess struct {
	pid string
}

// Init initializes the process.
func (p *BasicProcess) Init(ctx context.Context, stateGetter func() *GlobalState, stateMutator func(mutateFunc func(*GlobalState) *GlobalState), sender func(pid string, data interface{})) error {
	log.Println(p.pid, "initializing")
	stateMutator(IncrementActiveProcesses)
	return nil
}

// Run runs the process.
func (p *BasicProcess) Run(ctx context.Context, stateGetter func() *GlobalState, stateMutator func(mutateFunc func(*GlobalState) *GlobalState), sender func(pid string, data interface{}), shutdownCh chan struct{}, errCh chan<- error, selfShutdown func()) error {
	log.Println(p.pid, "running")
	for i := 0; i < 5; i++ {
		log.Println(p.pid, "doing work", i)
	}

	<-shutdownCh
	fmt.Println("process1 asked to shutdown")
	log.Println(p.pid, "finished work")

	errCh <- nil // Send nil to signal successful completion
	return nil
}

// Deinit deinitializes the process.
func (p *BasicProcess) Deinit(ctx context.Context, stateGetter func() *GlobalState, stateMutator func(mutateFunc func(*GlobalState) *GlobalState), sender func(pid string, data interface{})) error {
	log.Println(p.pid, "deinitializing")
	stateMutator(DecrementActiveProcesses)
	return nil
}

// Received handles data received from another process.
func (p *BasicProcess) Received(pid string, data interface{}) error {
	log.Println(p.pid, "received data from", pid, ":", data)
	return nil
}

//
