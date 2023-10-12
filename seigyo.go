package seigyo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// Process is an interface that describes a generic process with lifecycle methods
// and communication capabilities.
type Process[T any] interface {
	Init(ctx context.Context,
		stateGetter func() T,
		stateMutator func(mutateFunc func(T) T),
		sender func(pid string,
			data interface{})) error
	Run(ctx context.Context,
		stateGetter func() T,
		stateMutator func(mutateFunc func(T) T),
		sender func(pid string, data interface{}),
		shutdownCh chan struct{},
		errCh chan<- error,
		selfShutdown func(),
	) error
	Deinit(ctx context.Context,
		stateGetter func() T,
		stateMutator func(mutateFunc func(T) T),
		sender func(pid string,
			data interface{})) error
	Received(pid string, data interface{}) error
}

type ProcessConfig[T any] struct {
	Process               Process[T]
	ShouldRecover         bool
	Timeout               time.Duration // General timeout for all phases if specific ones are not set.
	InitTimeout           time.Duration
	RunTimeout            time.Duration
	DeinitTimeout         time.Duration
	InitMaxRetries        int
	RunMaxRetries         int
	DeinitMaxRetries      int
	InitRetryDelay        time.Duration
	RunRetryDelay         time.Duration
	DeinitRetryDelay      time.Duration
	MessageSendMaxRetries int
	MessageSendRetryDelay time.Duration
}

// Seigyo manages processes, global state, and communication between processes.
type Seigyo[T any] struct {
	processes    map[string]ProcessConfig[T]
	state        T
	stateMu      sync.Mutex
	shutdownChs  map[string]chan struct{}
	shutdownSent map[string]bool
	stopped      map[string]bool
	errored      map[string]bool
	errCh        chan error
	closed       bool
	wg           sync.WaitGroup
}

// `New` creates a new Controller.
func New[T any](initialState T) *Seigyo[T] {
	return &Seigyo[T]{
		processes:    make(map[string]ProcessConfig[T]),
		shutdownChs:  make(map[string]chan struct{}),
		shutdownSent: make(map[string]bool),
		stopped:      make(map[string]bool),
		errored:      make(map[string]bool),
		state:        initialState,
	}
}

func (c *Seigyo[T]) State(pid string) (bool, bool, bool, error) {

	var exists bool

	c.stateMu.Lock()
	_, exists = c.processes[pid]
	c.stateMu.Unlock()

	if !exists {
		return false, false, false, fmt.Errorf("no process found with PID: %s", pid)
	}

	var shutdownSent bool
	var stopped bool
	var errored bool

	shutdownSent, exists = c.shutdownSent[pid]
	if !exists {
		return false, false, false, fmt.Errorf("no shutdown found with PID: %s", pid)
	}
	stopped, exists = c.stopped[pid]
	if !exists {
		return false, false, false, fmt.Errorf("no stopped found with PID: %s", pid)
	}
	errored, exists = c.errored[pid]
	if !exists {
		return false, false, false, fmt.Errorf("no errored found with PID: %s", pid)
	}

	return shutdownSent, stopped, errored, nil
}

func (c *Seigyo[T]) Processes() []string {

	c.stateMu.Lock()
	keys := make([]string, 0, len(c.processes))
	for k := range c.processes {
		keys = append(keys, k)
	}
	c.stateMu.Unlock()

	return keys
}

func (c *Seigyo[T]) Send(frompid string, pid string, data interface{}) error {

	c.stateMu.Lock()
	targetProcessConfig, exists := c.processes[pid]
	c.stateMu.Unlock()

	if !exists {
		return fmt.Errorf("no process found with PID: %s", pid)
	}

	// Define a function to send a message with optional panic recovery and retry.
	sendMessage := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic while sending message: %v", r)
			}
		}()
		// Send the message.
		targetProcessConfig.Process.Received(frompid, data)
		log.Printf("sending message to %v \n", pid)
		return nil
	}

	// Retry sending the message if an error occurs or panic is recovered.
	retries := 0
	var lastErr error
	for retries < targetProcessConfig.MessageSendMaxRetries {
		lastErr = sendMessage()
		if lastErr == nil {
			return nil
		}
		retries++
		if targetProcessConfig.MessageSendRetryDelay > 0 {
			time.Sleep(targetProcessConfig.MessageSendRetryDelay)
		}
	}
	return fmt.Errorf("error sending message: %w", lastErr)
}

// RegisterProcess registers a new process with the controller.
func (c *Seigyo[T]) RegisterProcess(pid string, config ProcessConfig[T]) error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if config.InitMaxRetries == 0 {
		config.InitMaxRetries = 1
	}

	if config.RunMaxRetries == 0 {
		config.RunMaxRetries = 1
	}

	if config.DeinitMaxRetries == 0 {
		config.DeinitMaxRetries = 1
	}

	if config.MessageSendMaxRetries == 0 {
		config.MessageSendMaxRetries = 1
	}

	if config.InitRetryDelay == 0 {
		config.InitRetryDelay = time.Nanosecond * 1
	}

	if config.RunRetryDelay == 0 {
		config.RunRetryDelay = time.Nanosecond * 1
	}

	if config.DeinitRetryDelay == 0 {
		config.DeinitRetryDelay = time.Nanosecond * 1
	}

	if _, exists := c.processes[pid]; exists {
		return errors.New("process with pid already exists")
	}

	c.processes[pid] = config
	c.shutdownChs[pid] = make(chan struct{})
	c.shutdownSent[pid] = false
	c.stopped[pid] = false
	c.errored[pid] = false
	return nil
}

// Start initializes and runs all registered processes.
func (c *Seigyo[T]) Start() chan error {
	c.errCh = make(chan error, len(c.processes)*2) // Buffered to hold errors from all processes.

	for pid, config := range c.processes {
		c.wg.Add(1)
		go c.startProcess(pid, config)
	}

	// Close error channel once all processes have finished.
	go func() {
		c.wg.Wait()
		if !c.closed {
			c.closed = true
			close(c.errCh)
		}
	}()

	return c.errCh
}

func (c *Seigyo[T]) AddAndStart(pid string, config ProcessConfig[T]) error {
	c.wg.Add(1)

	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if config.InitMaxRetries == 0 {
		config.InitMaxRetries = 1
	}

	if config.RunMaxRetries == 0 {
		config.RunMaxRetries = 1
	}

	if config.DeinitMaxRetries == 0 {
		config.DeinitMaxRetries = 1
	}

	if config.MessageSendMaxRetries == 0 {
		config.MessageSendMaxRetries = 1
	}

	if config.InitRetryDelay == 0 {
		config.InitRetryDelay = time.Nanosecond * 1
	}

	if config.RunRetryDelay == 0 {
		config.RunRetryDelay = time.Nanosecond * 1
	}

	if config.DeinitRetryDelay == 0 {
		config.DeinitRetryDelay = time.Nanosecond * 1
	}

	if _, exists := c.processes[pid]; exists {
		return errors.New("process with pid already exists")
	}

	c.processes[pid] = config
	c.shutdownChs[pid] = make(chan struct{})
	c.shutdownSent[pid] = false
	c.stopped[pid] = false
	c.errored[pid] = false

	// Start the new process.
	go c.startProcess(pid, config)

	return nil
}

func (c *Seigyo[T]) startProcess(pid string, config ProcessConfig[T]) {
	defer c.wg.Done()

	shutdownCh := c.shutdownChs[pid]
	stateGetter := func() T {
		c.stateMu.Lock()
		defer c.stateMu.Unlock()
		return c.state
	}

	stateMutator := func(mutateFunc func(T) T) {
		c.stateMu.Lock()
		defer c.stateMu.Unlock()
		c.state = mutateFunc(c.state)
	}

	sender := func(targetPid string, data interface{}) {
		log.Println("sending to", targetPid)
		go func() {
			if err := c.Send(pid, targetPid, data); err != nil {
				c.errCh <- err
			}
		}()
	}

	// Create a cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure resources are cleaned up.

	// Define a helper function to execute a phase with optional panic recovery, retry, and timeout.
	executePhase := func(phaseName string, phaseFunc func(context.Context) error, timeout time.Duration, maxRetries int, retryDelay time.Duration) error {
		retries := 0
		var lastErr error
		for retries < maxRetries {
			func() {
				defer func() {
					if r := recover(); r != nil {
						lastErr = fmt.Errorf("panic during %s: %v", phaseName, r)
						if config.ShouldRecover {
							c.errCh <- fmt.Errorf("process %s: %w", pid, lastErr)
						} else {
							panic(r)
						}
					}
				}()

				// Create a context with an optional timeout.
				var phaseCtx context.Context
				if timeout > 0 {
					var cancelTimeout func()
					phaseCtx, cancelTimeout = context.WithTimeout(ctx, timeout)
					defer cancelTimeout()
				} else {
					phaseCtx = ctx
				}

				lastErr = phaseFunc(phaseCtx)
			}()
			if lastErr == nil {
				return nil
			}
			retries++
			time.Sleep(retryDelay)
		}
		return fmt.Errorf("error during %s: %w", phaseName, lastErr)
	}

	// Initialize the process with panic recovery, retry, and optional timeout.
	if err := executePhase("initialization", func(ctx context.Context) error {
		return config.Process.Init(ctx, stateGetter, stateMutator, sender)
	}, config.InitTimeout, config.InitMaxRetries, config.InitRetryDelay); err != nil {
		c.errCh <- fmt.Errorf("process %s: %w", pid, err)
		c.stateMu.Lock()
		c.errored[pid] = true
		c.stateMu.Unlock()
		return
	}

	// Run the process with panic recovery, retry, and optional timeout.
	if err := executePhase("run", func(ctx context.Context) error {
		return config.Process.Run(ctx, stateGetter, stateMutator, sender, shutdownCh, c.errCh, cancel)
	}, config.RunTimeout, config.RunMaxRetries, config.RunRetryDelay); err != nil {
		c.errCh <- fmt.Errorf("process %s: %w", pid, err)
		c.stateMu.Lock()
		c.errored[pid] = true
		c.stateMu.Unlock()
	}

	// Deinitialize the process with panic recovery, retry, and optional timeout.
	if err := executePhase("deinitialization", func(ctx context.Context) error {
		return config.Process.Deinit(ctx, stateGetter, stateMutator, sender)
	}, config.DeinitTimeout, config.DeinitMaxRetries, config.DeinitRetryDelay); err != nil {
		c.errCh <- fmt.Errorf("process %s: %w", pid, err)
		c.stateMu.Lock()
		c.errored[pid] = true
		c.stateMu.Unlock()
	}

	// Mark the process as stopped.
	c.stateMu.Lock()
	c.stopped[pid] = true
	c.stateMu.Unlock()
}

func (c *Seigyo[T]) StopProcess(pid string) error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	// Check if the process exists.
	if _, exists := c.processes[pid]; !exists {
		return errors.New("process with pid does not exist")
	}

	// Signal the process to shut down if it hasn't been signaled yet.
	if !c.shutdownSent[pid] {
		close(c.shutdownChs[pid])
		c.shutdownSent[pid] = true
	}

	return nil
}

// Stop stops all registered processes.
func (c *Seigyo[T]) Stop() {
	c.stateMu.Lock()
	// Signal all processes to shut down.
	for pid, shutdownCh := range c.shutdownChs {
		if !c.shutdownSent[pid] {
			close(shutdownCh)
			c.shutdownSent[pid] = true
		}
	}
	c.stateMu.Unlock()

	// Wait for all processes to finish shutting down.
	var wg sync.WaitGroup
	for pid := range c.processes {
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()
			for {
				c.stateMu.Lock()
				stopped := c.stopped[pid]
				c.stateMu.Unlock()
				if stopped {
					break
				}
				time.Sleep(10 * time.Millisecond) // Polling interval
			}
		}(pid)
	}
	wg.Wait()

	if !c.closed {
		c.closed = true
		close(c.errCh)
	}
}
