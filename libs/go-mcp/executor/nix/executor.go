// Package nix provides a Nix-based executor implementation.
package nix

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/amarbel-llc/go-lib-mcp/executor"
)

// Executor builds and executes processes using Nix flakes.
// It caches built executable paths to avoid redundant builds.
type Executor struct {
	cache   map[string]string
	cacheMu sync.RWMutex
}

// New creates a new Nix executor.
func New() *Executor {
	return &Executor{
		cache: make(map[string]string),
	}
}

// Build builds a Nix flake and returns the path to the executable.
// The spec parameter should be a Nix flake reference (e.g., "nixpkgs#gopls").
// Results are cached to avoid rebuilding the same flake multiple times.
func (e *Executor) Build(ctx context.Context, flake string) (string, error) {
	// Check cache first
	e.cacheMu.RLock()
	if path, ok := e.cache[flake]; ok {
		e.cacheMu.RUnlock()
		return path, nil
	}
	e.cacheMu.RUnlock()

	// Build the flake
	cmd := exec.CommandContext(ctx, "nix", "build", flake, "--no-link", "--print-out-paths")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("nix build failed: %w\n%s", err, stderr.String())
	}

	// Parse output path
	outPath := strings.TrimSpace(stdout.String())
	if outPath == "" {
		return "", fmt.Errorf("nix build returned empty path")
	}

	// Handle multiple output paths (take first line)
	lines := strings.Split(outPath, "\n")
	outPath = strings.TrimSpace(lines[0])

	// Find the executable in the output path
	binPath, err := findExecutable(outPath)
	if err != nil {
		return "", err
	}

	// Cache the result
	e.cacheMu.Lock()
	e.cache[flake] = binPath
	e.cacheMu.Unlock()

	return binPath, nil
}

// findExecutable locates an executable within a Nix store path.
// It first checks for a bin/ directory, then falls back to checking if the
// store path itself is executable.
func findExecutable(storePath string) (string, error) {
	binDir := filepath.Join(storePath, "bin")

	// Check if bin/ directory exists
	entries, err := os.ReadDir(binDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No bin/ directory - check if the store path itself is executable
			info, statErr := os.Stat(storePath)
			if statErr == nil && info.Mode()&0111 != 0 {
				return storePath, nil
			}
			return "", fmt.Errorf("no bin directory and store path not executable: %s", storePath)
		}
		return "", fmt.Errorf("reading bin directory: %w", err)
	}

	// Find first executable file in bin/
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		binPath := filepath.Join(binDir, entry.Name())
		info, err := os.Stat(binPath)
		if err != nil {
			continue
		}
		if info.Mode()&0111 != 0 {
			return binPath, nil
		}
	}

	return "", fmt.Errorf("no executable found in %s/bin", storePath)
}

// Execute starts a process with the given executable path and arguments.
func (e *Executor) Execute(ctx context.Context, path string, args []string) (*executor.Process, error) {
	cmd := exec.CommandContext(ctx, path, args...)

	// Set up pipes for stdin, stdout, stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("starting process: %w", err)
	}

	return &executor.Process{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Wait:   cmd.Wait,
		Kill: func() error {
			if cmd.Process != nil {
				return cmd.Process.Kill()
			}
			return nil
		},
	}, nil
}

// ClearCache clears the build cache.
func (e *Executor) ClearCache() {
	e.cacheMu.Lock()
	e.cache = make(map[string]string)
	e.cacheMu.Unlock()
}

// CachedPath returns the cached executable path for a flake, if any.
func (e *Executor) CachedPath(flake string) (string, bool) {
	e.cacheMu.RLock()
	defer e.cacheMu.RUnlock()
	path, ok := e.cache[flake]
	return path, ok
}
