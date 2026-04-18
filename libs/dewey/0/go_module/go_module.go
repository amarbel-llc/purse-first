// Package go_module exposes small helpers for discovering a Go module's
// import path from its go.mod file.
package go_module

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveModulePath returns explicit when non-empty, otherwise reads the
// module path from go.mod in dir. Errors when dir contains no go.mod and
// no explicit override was provided.
func ResolveModulePath(dir, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return "", fmt.Errorf("must be run from a directory containing go.mod")
	}

	return ReadModulePath(goModPath)
}

// ReadModulePath parses the `module <path>` directive from the file at
// goModPath and returns the path value.
func ReadModulePath(goModPath string) (string, error) {
	f, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("no module directive found in %s", goModPath)
}
