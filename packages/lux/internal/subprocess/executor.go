package subprocess

import (
	"context"
	"io"
)

type Process struct {
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
	Wait   func() error
	Kill   func() error
}

type Executor interface {
	Build(ctx context.Context, flake, binarySpec string) (string, error)
	Execute(ctx context.Context, path string, args []string, env map[string]string, workDir string) (*Process, error)
}
