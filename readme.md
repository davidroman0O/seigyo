# Work in progress

I'm trying to figure how i can integrate an actor-model implementation with my notion of stakable `Process`. It has to be efficient and performant, i'm doing lot of tests in the playground!

# Seigyo

Seigyo, deriving from the Japanese word for "Control" (制御), is a Go module meticulously crafted to orchestrate and manage the lifecycle of concurrent processes. It ensures smooth communication and control across various application workflows, providing a robust foundation for building reliable concurrent applications.

## Features

- **Process Lifecycle Management**: Define, initialize, run, and deinitialize processes with ease.
- **Graceful Shutdown**: Ensure smooth shutdowns and resource cleanup with integrated signal handling.
- **Inter-Process Communication**: Facilitate communication between different processes.
- **Error Handling**: Robust error handling and recovery mechanisms for processes.
- **Configurable Retries**: Customize retry logic for process initialization, running, and deinitialization.

## Getting Started

### Prerequisites

- Go (version 1.18 or higher recommended)

### Installation

Install Seigyo using `go get`:

```sh
go get github.com/davidroman0O/seigyo
```

### Basic Usage

Here's a quick example to get you started with Seigyo:

```go
package main

import (
	"context"
	"fmt"
	"github.com/davidroman0O/seigyo"
)

type MyProcess struct{}

func (p *MyProcess) Init(ctx context.Context, stateGetter func() interface{}, stateMutator func(mutateFunc func(interface{}) interface{}), sender func(pid string, data interface{})) error {
	fmt.Println("Process Initialized")
	return nil
}

func (p *MyProcess) Run(ctx context.Context, stateGetter func() interface{}, stateMutator func(mutateFunc func(interface{}) interface{}), sender func(pid string, data interface{}), shutdownCh chan struct{}, errCh chan<- error) error {
	fmt.Println("Process Running")
	return nil
}

func (p *MyProcess) Deinit(ctx context.Context, stateGetter func() interface{}, stateMutator func(mutateFunc func(interface{}) interface{}), sender func(pid string, data interface{})) error {
	fmt.Println("Process Deinitialized")
	return nil
}

func (p *MyProcess) Received(pid string, data interface{}) error {
	fmt.Printf("Received data from %s: %v\n", pid, data)
	return nil
}

func main() {
	controller := seigyo.New(nil)
	process := &MyProcess{}

	// Register and start your process here...
}
```

## Examples

### Example 1: Basic Process Management

```go
// Example code for basic process management...
```

### Example 2: Inter-Process Communication

```go
// Example code for inter-process communication...
```

## Documentation

Detailed documentation can be found at [GoDoc: Seigyo](https://pkg.go.dev/github.com/davidroman0O/seigyo).

If you need some examples, you [look at the Seigyo example repository](https://github.com/davidroman0O/seigyo-examples).

## Contributing

We welcome contributions from the community! Please read our [Contributing Guide](CONTRIBUTING.md) for more details on how to contribute to Seigyo.

## License

Seigyo is licensed under the [MIT License](LICENSE).
