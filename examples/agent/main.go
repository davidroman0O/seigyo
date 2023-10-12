package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

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

func (p *AgentProcess) Run(ctx context.Context, stateGetter func() *Global, stateMutator func(mutateFunc func(*Global) *Global), sender func(pid string, data interface{}), shutdownCh chan struct{}, errCh chan<- error) error {
	log.Println("agent running")
	<-shutdownCh
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

	agent := seigyo.New[*Global](&Global{})

	agent.RegisterProcess("agent", seigyo.ProcessConfig[*Global]{
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
