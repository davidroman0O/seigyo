package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/davidroman0O/seigyo"
)

type Global struct{}

func main() {
	// Create a channel to receive OS signals.
	sigCh := make(chan os.Signal, 1)

	// Notify sigCh when receiving SIGINT or SIGTERM signals.
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ctrl := seigyo.New(&Global{})
	ctrl.RegisterProcess("ping", seigyo.ProcessConfig[*Global]{
		Process: &PingPongProcess{pid: "ping"},
	})
	ctrl.RegisterProcess("pong", seigyo.ProcessConfig[*Global]{
		Process: &PingPongProcess{pid: "pong"},
	})

	go func() {
		// Wait for an OS signal.
		sig := <-sigCh
		// Handle the received OS signal gracefully.
		fmt.Printf("Received signal: %v. Shutting down...\n", sig)
		ctrl.Stop()
	}()

	errCh := ctrl.Start()

	for err := range errCh {
		fmt.Println(err)
	}

	fmt.Println("stopped")
}

// ExampleProcess is a simple implementation of the Process interface.
type PingPongProcess struct {
	pid string
}

// Init initializes the process.
func (p *PingPongProcess) Init(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{})) error {
	fmt.Println(p.pid, "initializing")
	return nil
}

func (p *PingPongProcess) Run(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{}), shutdownCh <-chan struct{}, errCh chan<- error) error {
	fmt.Println(p.pid, "running")

	var target string
	if p.pid == "ping" {
		target = "pong"
	} else {
		target = "ping"
	}
	sender(target, "hello")

	errCh <- fmt.Errorf("some error")

	<-shutdownCh

	fmt.Println(p.pid, "finished")

	return nil
}

// Deinit deinitializes the process.
func (p *PingPongProcess) Deinit(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{})) error {
	fmt.Println(p.pid, "deinitializing")
	return nil
}

// Received handles data received from another process.
func (p *PingPongProcess) Received(pid string, data interface{}) error {
	fmt.Println(p.pid, "received data from", pid, ":", data)
	return nil
}
