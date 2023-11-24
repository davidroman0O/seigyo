package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davidroman0O/seigyo"
)

type Global struct{}

// `AgentProcess` represent a permanent process that will wait for instructions from client
type AgentProcess struct {
	pid string
}

func (p *AgentProcess) Init(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{})) error {
	log.Println("agent initializing")
	return nil
}

func (p *AgentProcess) Run(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{}), shutdownCh chan struct{}, errCh chan<- error, selfShutdown func()) error {
	log.Println("agent running")
	go func() {
		time.Sleep(time.Second * 2)
		log.Println("agent kill myself")
		time.Sleep(time.Second * 2)
		selfShutdown()
	}()

	// Wait for a shutdown signal from either the external shutdown channel or the context.
	select {
	case <-shutdownCh:
		fmt.Println("agent shutdown via shutdownCh")
	case <-ctx.Done():
		fmt.Println("agent shutdown via context cancellation")
	}

	log.Println("agent shutdown")
	return nil
}

func (p *AgentProcess) Deinit(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{})) error {
	log.Println("agent deinitialization")
	return nil
}

func (p *AgentProcess) Received(pid string, data interface{}) error {

	return nil
}

func main() {
	// Create a channel to receive OS signals.
	sigCh := make(chan os.Signal, 1)

	// Notify sigCh when receiving SIGINT or SIGTERM signals.
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	agent := seigyo.New[*Global, interface{}](&Global{})

	agent.RegisterProcess("agent", seigyo.ProcessConfig[*Global, interface{}]{
		Process: &AgentProcess{
			pid: "agent",
		},
		ShouldRecover:         true,
		RunMaxRetries:         99,
		InitMaxRetries:        99,
		DeinitMaxRetries:      99,
		MessageSendMaxRetries: 99,
	})

	errCh := agent.Start()

	go func() {
		<-sigCh
		agent.Stop()
	}()

	for err := range errCh {
		log.Println("error:", err)
	}

	agent.Stop()
}
