// Package executor provides abstractions for building and executing processes.
// This is useful for MCP servers that need to manage subprocesses.
package executor

import (
	"context"
	"io"
)

// Process represents a running process.
type Process struct {
	// Stdin is the process standard input.
	Stdin io.WriteCloser

	// Stdout is the process standard output.
	Stdout io.ReadCloser

	// Stderr is the process standard error.
	Stderr io.ReadCloser

	// Wait waits for the process to exit and returns any error.
	Wait func() error

	// Kill terminates the process.
	Kill func() error
}

// Executor builds and executes processes.
// Different implementations can provide different build/execution strategies
// (e.g., Nix flakes, direct binary execution, containers, etc.).
type Executor interface {
	// Build resolves a specification (e.g., Nix flake reference) to an executable path.
	// The spec format depends on the implementation.
	Build(ctx context.Context, spec string) (string, error)

	// Execute starts a process with the given executable path and arguments.
	Execute(ctx context.Context, path string, args []string) (*Process, error)
}
