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

type Messages interface {
	MsgPing | MsgPong
}

func main() {

	ctrl := seigyo.New[*Global, Messages](&Global{})
	ctrl.RegisterProcess("ping", seigyo.ProcessConfig[*Global, Messages]{
		Process: &PingPongProcess{pid: "ping"},
	})
	ctrl.RegisterProcess("pong", seigyo.ProcessConfig[*Global, Messages]{
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

	var target string
	if p.pid == "ping" {
		target = "pong"
	} else {
		target = "ping"
	}
	sender(target, "hello")

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
	return nil
}
