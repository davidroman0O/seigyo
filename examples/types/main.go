package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/davidroman0O/seigyo"
)

type Global struct{}

type MsgPing string
type MsgPong string

func main() {

	ctrl := seigyo.New[*Global, interface{}](&Global{})
	ctrl.RegisterProcess("ping", seigyo.ProcessConfig[*Global, interface{}]{
		Process: &PingPongProcess{pid: "ping"},
	})
	ctrl.RegisterProcess("pong", seigyo.ProcessConfig[*Global, interface{}]{
		Process: &PingPongProcess{pid: "pong"},
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
	fmt.Println("finished communication")
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

func (p *PingPongProcess) Run(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{}), shutdownCh chan struct{}, errCh chan<- error, selfShutdown func()) error {
	fmt.Println(p.pid, "running")

	if p.pid == "ping" {
		target := MsgPong("pong")
		sender(string(target), target)
	} else {
		target := MsgPing("ping")
		sender(string(target), target)
	}

	errCh <- fmt.Errorf("some error")

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
	switch data.(type) {
	case MsgPing:
	case MsgPong:
	}
	return nil
}
