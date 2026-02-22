package subprocess

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type NixExecutor struct {
	cache   map[string]string
	cacheMu sync.RWMutex
}

func NewNixExecutor() *NixExecutor {
	return &NixExecutor{
		cache: make(map[string]string),
	}
}

func (e *NixExecutor) Build(ctx context.Context, flake, binarySpec string) (string, error) {
	cacheKey := flake
	if binarySpec != "" {
		cacheKey = flake + "::" + binarySpec
	}

	e.cacheMu.RLock()
	if path, ok := e.cache[cacheKey]; ok {
		e.cacheMu.RUnlock()
		return path, nil
	}
	e.cacheMu.RUnlock()

	cmd := exec.CommandContext(ctx, "nix", "build", flake, "--no-link", "--print-out-paths")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("nix build failed: %w\n%s", err, stderr.String())
	}

	outPath := strings.TrimSpace(stdout.String())
	if outPath == "" {
		return "", fmt.Errorf("nix build returned empty path")
	}

	lines := strings.Split(outPath, "\n")
	outPath = strings.TrimSpace(lines[0])

	binPath, err := findExecutable(outPath, binarySpec)
	if err != nil {
		return "", err
	}

	e.cacheMu.Lock()
	e.cache[cacheKey] = binPath
	e.cacheMu.Unlock()

	return binPath, nil
}

func findExecutable(storePath, binarySpec string) (string, error) {
	if binarySpec != "" {
		var candidatePath string

		if strings.Contains(binarySpec, "/") {
			candidatePath = filepath.Join(storePath, binarySpec)
		} else {
			candidatePath = filepath.Join(storePath, "bin", binarySpec)
		}

		cleanPath := filepath.Clean(candidatePath)
		if !strings.HasPrefix(cleanPath, filepath.Clean(storePath)) {
			return "", fmt.Errorf("binary path %q escapes store path", binarySpec)
		}

		info, err := os.Stat(candidatePath)
		if err != nil {
			return "", fmt.Errorf("binary %q not found: %w", binarySpec, err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("binary %q is a directory", binarySpec)
		}
		if info.Mode()&0111 == 0 {
			return "", fmt.Errorf("binary %q is not executable", binarySpec)
		}

		return candidatePath, nil
	}

	binDir := filepath.Join(storePath, "bin")

	entries, err := os.ReadDir(binDir)
	if err != nil {
		if os.IsNotExist(err) {
			info, statErr := os.Stat(storePath)
			if statErr == nil && info.Mode()&0111 != 0 {
				return storePath, nil
			}
			return "", fmt.Errorf("no bin directory and store path not executable: %s", storePath)
		}
		return "", fmt.Errorf("reading bin directory: %w", err)
	}

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

func (e *NixExecutor) Execute(ctx context.Context, path string, args []string, env map[string]string, workDir string) (*Process, error) {
	cmd := exec.CommandContext(ctx, path, args...)

	if workDir != "" {
		cmd.Dir = workDir
	}

	// Set up environment variables
	if len(env) > 0 {
		// Start with current environment
		cmd.Env = os.Environ()

		// Add or override with custom env vars
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

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

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("starting process: %w", err)
	}

	return &Process{
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

func (e *NixExecutor) ClearCache() {
	e.cacheMu.Lock()
	e.cache = make(map[string]string)
	e.cacheMu.Unlock()
}

func (e *NixExecutor) CachedPath(flake string) (string, bool) {
	e.cacheMu.RLock()
	defer e.cacheMu.RUnlock()
	path, ok := e.cache[flake]
	return path, ok
}
