package main

import (
	"context"
	"fmt"
	"time"

	"github.com/davidroman0O/seigyo"
)

type Global struct{}

func main() {
	ctrl := seigyo.New(&Global{})

	ctrl.RegisterProcess("ping", seigyo.ProcessConfig[*Global]{
		Process: &PingProcess{pid: "ping"},
	})
	ctrl.RegisterProcess("pong", seigyo.ProcessConfig[*Global]{
		Process: &PongProcess{pid: "pong"},
	})

	errCh := ctrl.Start()

	for err := range errCh {
		fmt.Println(err)
	}

	fmt.Println("finished communication signal")
}

// ExampleProcess is a simple implementation of the Process interface.
type PingProcess struct {
	pid  string
	exit bool
}

// Init initializes the process.
func (p *PingProcess) Init(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{})) error {
	fmt.Println(p.pid, "initializing")
	return nil
}

func (p *PingProcess) Run(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{}), shutdownCh chan struct{}, errCh chan<- error) error {
	fmt.Println(p.pid, "running")

	go func() {
		errCh <- fmt.Errorf("dying")
		fmt.Println("killing myself")
		fmt.Println("in 5")
		time.Sleep(time.Second * 1)
		fmt.Println("in 4")
		time.Sleep(time.Second * 1)
		fmt.Println("in 3")
		time.Sleep(time.Second * 1)
		fmt.Println("in 2")
		time.Sleep(time.Second * 1)
		fmt.Println("in 1")
		time.Sleep(time.Second * 1)
		fmt.Println("bang")
		close(shutdownCh)
	}()

	<-shutdownCh

	return nil
}

// Deinit deinitializes the process.
func (p *PingProcess) Deinit(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{})) error {
	fmt.Println(p.pid, "deinitializing")
	return nil
}

// Received handles data received from another process.
func (p *PingProcess) Received(pid string, data interface{}) error {
	fmt.Println(p.pid, "received data from", pid, ":", data)
	switch msg := data.(type) {
	case string:
		if msg == "die" {
			fmt.Println("pong want me to die")
			p.exit = true
		}
	}
	return nil
}

// ExampleProcess is a simple implementation of the Process interface.
type PongProcess struct {
	pid string
}

// Init initializes the process.
func (p *PongProcess) Init(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{})) error {
	fmt.Println(p.pid, "initializing")
	return nil
}

func (p *PongProcess) Run(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{}), shutdownCh chan struct{}, errCh chan<- error) error {
	fmt.Println(p.pid, "running")
	sender("ping", "die")

	return nil
}

// Deinit deinitializes the process.
func (p *PongProcess) Deinit(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{})) error {
	fmt.Println(p.pid, "deinitializing")
	return nil
}

// Received handles data received from another process.
func (p *PongProcess) Received(pid string, data interface{}) error {
	fmt.Println(p.pid, "received data from", pid, ":", data)
	return nil
}
