# Runtime Dependencies for LSP Servers — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let LSP servers declare nix flake references as runtime dependencies that get built and added to PATH when the LSP subprocess spawns, fixing gopls "no views" errors caused by missing `go` binary.

**Architecture:** Two features: (1) env variable expansion in `NixExecutor.Execute` as a stopgap, (2) `runtime_deps` config field in `lsps.toml` that resolves flake references to store paths and prepends their `bin/` dirs to PATH. A `rebuild` MCP tool lets users flush caches and restart LSPs without restarting lux.

**Tech Stack:** Go, TOML config, Nix flake builds, MCP tool registration via go-lib-mcp

---

## Task 1: Env variable expansion in NixExecutor.Execute

**Files:**
- Modify: `internal/subprocess/nix.go:134-143`
- Test: `internal/subprocess/nix_test.go`

### Step 1: Write failing test for env expansion

Add to `internal/subprocess/nix_test.go`:

```go
func TestExpandEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envSetup map[string]string
		expected string
	}{
		{
			name:     "no variables",
			input:    "/usr/bin:/bin",
			expected: "/usr/bin:/bin",
		},
		{
			name:     "expand dollar-brace syntax",
			input:    "${HOME}/go/bin:${PATH}",
			envSetup: map[string]string{"HOME": "/users/test", "PATH": "/usr/bin"},
			expected: "/users/test/go/bin:/usr/bin",
		},
		{
			name:     "expand dollar syntax",
			input:    "$HOME/go/bin",
			envSetup: map[string]string{"HOME": "/users/test"},
			expected: "/users/test/go/bin",
		},
		{
			name:     "missing variable expands to empty",
			input:    "${NONEXISTENT}:/bin",
			expected: ":/bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envSetup {
				t.Setenv(k, v)
			}
			result := expandEnvVars(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./internal/subprocess/ -run TestExpandEnvVars -v`
Expected: FAIL — `expandEnvVars` undefined

### Step 3: Implement expandEnvVars and wire into Execute

In `internal/subprocess/nix.go`, add:

```go
func expandEnvVars(s string) string {
	return os.Expand(s, os.Getenv)
}
```

Then modify `Execute` (lines 134-143) to expand env values:

```go
	// Set up environment variables
	if len(env) > 0 {
		// Start with current environment
		cmd.Env = os.Environ()

		// Add or override with custom env vars
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, expandEnvVars(v)))
		}
	}
```

### Step 4: Run test to verify it passes

Run: `go test ./internal/subprocess/ -run TestExpandEnvVars -v`
Expected: PASS

### Step 5: Commit

```
feat: expand environment variables in LSP env config

Allows ${VAR} and $VAR syntax in lsps.toml env values, enabling
users to reference PATH and other variables without hardcoding
nix store paths. Stopgap for gopls needing `go` on PATH.
```

---

## Task 2: Add `runtime_deps` field to LSP config

**Files:**
- Modify: `internal/config/config.go:19-36`
- Test: `internal/config/config_test.go`

### Step 1: Write failing test for runtime_deps TOML parsing

Add to `internal/config/config_test.go`:

```go
func TestLSP_RuntimeDeps_TOML(t *testing.T) {
	input := `
name = "gopls"
flake = "nixpkgs#gopls"
extensions = ["go"]
runtime_deps = ["nixpkgs#go"]
`
	var lsp LSP
	if err := toml.Unmarshal([]byte(input), &lsp); err != nil {
		t.Fatalf("failed to parse TOML: %v", err)
	}

	if len(lsp.RuntimeDeps) != 1 {
		t.Fatalf("expected 1 runtime dep, got %d", len(lsp.RuntimeDeps))
	}

	if lsp.RuntimeDeps[0] != "nixpkgs#go" {
		t.Errorf("expected runtime dep %q, got %q", "nixpkgs#go", lsp.RuntimeDeps[0])
	}
}

func TestLSP_RuntimeDeps_Empty(t *testing.T) {
	input := `
name = "nil"
flake = "nixpkgs#nil"
extensions = ["nix"]
`
	var lsp LSP
	if err := toml.Unmarshal([]byte(input), &lsp); err != nil {
		t.Fatalf("failed to parse TOML: %v", err)
	}

	if len(lsp.RuntimeDeps) != 0 {
		t.Errorf("expected 0 runtime deps, got %d", len(lsp.RuntimeDeps))
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./internal/config/ -run TestLSP_RuntimeDeps -v`
Expected: FAIL — `RuntimeDeps` field doesn't exist

### Step 3: Add RuntimeDeps field to LSP struct

In `internal/config/config.go`, add after line 35 (`EagerStart`):

```go
	RuntimeDeps []string `toml:"runtime_deps,omitempty"`
```

### Step 4: Run test to verify it passes

Run: `go test ./internal/config/ -run TestLSP_RuntimeDeps -v`
Expected: PASS

### Step 5: Commit

```
feat(config): add runtime_deps field to LSP config

Declares nix flake references that the LSP needs at runtime
(e.g., gopls needs `go`). Parsed from lsps.toml but not yet
wired to subprocess spawning.
```

---

## Task 3: Add RuntimeDeps to Pool.Register and LSPInstance

**Files:**
- Modify: `internal/subprocess/pool.go:46-71` (LSPInstance struct)
- Modify: `internal/subprocess/pool.go:96-115` (Register)
- Modify: `internal/subprocess/pool.go:124-178` (GetOrStart)
- Modify: `internal/subprocess/nix.go` (Build method — for resolving dep paths)
- Test: `internal/subprocess/pool_test.go` (new file or add to existing)

### Step 1: Write failing test for runtime deps path resolution

Create or add to `internal/subprocess/nix_test.go`:

```go
func TestBuildRuntimeDepsPath(t *testing.T) {
	executor := &mockBuildExecutor{
		paths: map[string]string{
			"nixpkgs#go": "/nix/store/fake-go",
		},
	}

	result, err := buildRuntimeDepsPath(context.Background(), executor, []string{"nixpkgs#go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "/nix/store/fake-go/bin"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildRuntimeDepsPath_Multiple(t *testing.T) {
	executor := &mockBuildExecutor{
		paths: map[string]string{
			"nixpkgs#go":    "/nix/store/fake-go",
			"nixpkgs#rustc": "/nix/store/fake-rustc",
		},
	}

	result, err := buildRuntimeDepsPath(context.Background(), executor, []string{"nixpkgs#go", "nixpkgs#rustc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "/nix/store/fake-go/bin:/nix/store/fake-rustc/bin"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildRuntimeDepsPath_Empty(t *testing.T) {
	executor := &mockBuildExecutor{}

	result, err := buildRuntimeDepsPath(context.Background(), executor, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

type mockBuildExecutor struct {
	paths map[string]string
}

func (m *mockBuildExecutor) Build(ctx context.Context, flake, binary string) (string, error) {
	if path, ok := m.paths[flake]; ok {
		return path, nil
	}
	return "", fmt.Errorf("unknown flake: %s", flake)
}

func (m *mockBuildExecutor) Execute(ctx context.Context, path string, args []string, env map[string]string, workDir string) (*Process, error) {
	return nil, nil
}

func (m *mockBuildExecutor) ClearCache() {}

func (m *mockBuildExecutor) CachedPath(flake string) (string, bool) {
	path, ok := m.paths[flake]
	return path, ok
}
```

### Step 2: Run test to verify it fails

Run: `go test ./internal/subprocess/ -run TestBuildRuntimeDepsPath -v`
Expected: FAIL — `buildRuntimeDepsPath` undefined

### Step 3: Implement buildRuntimeDepsPath

In `internal/subprocess/nix.go`, add:

```go
// buildRuntimeDepsPath builds each runtime dep flake and returns a
// colon-separated PATH string of their bin/ directories.
func buildRuntimeDepsPath(ctx context.Context, executor Executor, deps []string) (string, error) {
	if len(deps) == 0 {
		return "", nil
	}

	var paths []string
	for _, dep := range deps {
		storePath, err := executor.Build(ctx, dep, "")
		if err != nil {
			return "", fmt.Errorf("building runtime dep %s: %w", dep, err)
		}
		// Build returns the binary path; we need the store path's bin/ dir.
		// The store path is the parent of bin/.
		binDir := filepath.Join(filepath.Dir(storePath), "..", "bin")
		// Normalize: if Build returned a store path directly (no bin/),
		// use storePath/bin instead.
		binDir = filepath.Join(storePath, "bin")
		if info, err := os.Stat(binDir); err == nil && info.IsDir() {
			paths = append(paths, binDir)
		} else {
			// Fall back to using the directory containing the binary
			paths = append(paths, filepath.Dir(storePath))
		}
	}

	return strings.Join(paths, ":"), nil
}
```

Wait — `Build` returns a *binary* path (e.g., `/nix/store/xxx-go/bin/go`), not a store path. For runtime deps we want the store path so we can add `bin/` to PATH. Let me reconsider.

Actually, for `runtime_deps`, we don't need to find a specific binary — we just need the store path's `bin/` directory. We should use `nix build --print-out-paths` directly (which `Build` already does internally) but return the store path, not a specific binary.

Better approach — add a `BuildStorePath` method:

```go
func (e *NixExecutor) BuildStorePath(ctx context.Context, flake string) (string, error) {
	cacheKey := "storepath::" + flake

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
	storePath := strings.TrimSpace(lines[0])

	e.cacheMu.Lock()
	e.cache[cacheKey] = storePath
	e.cacheMu.Unlock()

	return storePath, nil
}
```

Then update `buildRuntimeDepsPath` to use it:

```go
func buildRuntimeDepsPath(ctx context.Context, executor *NixExecutor, deps []string) (string, error) {
	if len(deps) == 0 {
		return "", nil
	}

	var paths []string
	for _, dep := range deps {
		storePath, err := executor.BuildStorePath(ctx, dep)
		if err != nil {
			return "", fmt.Errorf("building runtime dep %s: %w", dep, err)
		}
		paths = append(paths, filepath.Join(storePath, "bin"))
	}

	return strings.Join(paths, ":"), nil
}
```

Update the mock and Executor interface accordingly. Add `BuildStorePath` to the Executor interface in `executor.go`:

```go
type Executor interface {
	Build(ctx context.Context, flake, binarySpec string) (string, error)
	BuildStorePath(ctx context.Context, flake string) (string, error)
	Execute(ctx context.Context, path string, args []string, env map[string]string, workDir string) (*Process, error)
	ClearCache()
}
```

Note: `ClearCache` also needs to be added to the interface since the `rebuild` tool will need it. Currently it's only on the concrete type.

### Step 4: Run tests to verify they pass

Run: `go test ./internal/subprocess/ -run TestBuildRuntimeDepsPath -v`
Expected: PASS

### Step 5: Add RuntimeDeps to LSPInstance and Register

In `internal/subprocess/pool.go`, add `RuntimeDeps []string` to `LSPInstance` struct (after line 51, the `Env` field).

Update `Register` signature to accept `runtimeDeps []string` and store it.

Update `GetOrStart` to resolve runtime deps before Execute:

```go
	// After building the LSP binary (line 161-166), before Execute (line 173):

	// Build runtime deps and construct PATH prefix
	depsPath, err := buildRuntimeDepsPath(inst.ctx, p.executor.(*NixExecutor), inst.RuntimeDeps)
	if err != nil {
		inst.State = LSPStateFailed
		inst.Error = err
		return nil, fmt.Errorf("building runtime deps for %s: %w", name, err)
	}

	// Merge runtime deps PATH into env
	env := inst.Env
	if depsPath != "" {
		if env == nil {
			env = make(map[string]string)
		} else {
			// Copy to avoid mutating the original
			envCopy := make(map[string]string, len(env))
			for k, v := range env {
				envCopy[k] = v
			}
			env = envCopy
		}
		if existing, ok := env["PATH"]; ok {
			env["PATH"] = depsPath + ":" + existing
		} else {
			env["PATH"] = depsPath + ":${PATH}"
		}
	}

	proc, err := p.executor.Execute(inst.ctx, binPath, inst.Args, env, workDir)
```

### Step 6: Update callers of Register

In `internal/server/server.go` (lines 62 and 147) and `internal/mcp/server.go` (line 54), add `l.RuntimeDeps` to the Register call.

### Step 7: Run all tests

Run: `go test ./...`
Expected: PASS (some tests may need mock updates for the new Executor interface methods)

### Step 8: Commit

```
feat: resolve runtime_deps flakes and prepend to LSP PATH

When an LSP declares runtime_deps in lsps.toml, lux builds each
flake reference and adds their bin/ directories to the subprocess
PATH. Fixes gopls "no views" when `go` is not on the system PATH.
```

---

## Task 4: Pre-build runtime deps in warmup

**Files:**
- Modify: `internal/warmup/warmup.go:14-26`
- Test: `internal/warmup/warmup_test.go`

### Step 1: Write failing test

Add to `internal/warmup/warmup_test.go`:

```go
func TestPreBuildAll_IncludesRuntimeDeps(t *testing.T) {
	cfg := &config.Config{
		LSPs: []config.LSP{
			{
				Name:        "gopls",
				Flake:       "nixpkgs#gopls",
				RuntimeDeps: []string{"nixpkgs#go"},
			},
		},
	}

	executor := newMockExecutor()
	PreBuildAll(context.Background(), cfg, executor)

	executor.mu.Lock()
	defer executor.mu.Unlock()

	if executor.builds["nixpkgs#gopls"] != 1 {
		t.Errorf("expected 1 build for gopls, got %d", executor.builds["nixpkgs#gopls"])
	}
	if executor.storeBuilds["nixpkgs#go"] != 1 {
		t.Errorf("expected 1 store build for go, got %d", executor.storeBuilds["nixpkgs#go"])
	}
}
```

Update mock executor to track `BuildStorePath` calls:

```go
type mockExecutor struct {
	mu          sync.Mutex
	builds      map[string]int
	storeBuilds map[string]int
}

func (m *mockExecutor) BuildStorePath(ctx context.Context, flake string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storeBuilds[flake]++
	return "/nix/store/fake-" + flake, nil
}
```

### Step 2: Run test to verify it fails

Run: `go test ./internal/warmup/ -run TestPreBuildAll_IncludesRuntimeDeps -v`
Expected: FAIL

### Step 3: Extend PreBuildAll

In `internal/warmup/warmup.go`:

```go
func PreBuildAll(ctx context.Context, cfg *config.Config, executor subprocess.Executor) {
	var wg sync.WaitGroup
	for _, l := range cfg.LSPs {
		wg.Add(1)
		go func(flake, binary, name string) {
			defer wg.Done()
			if _, err := executor.Build(ctx, flake, binary); err != nil {
				fmt.Fprintf(os.Stderr, "[lux] pre-build %s: %v\n", name, err)
			}
		}(l.Flake, l.Binary, l.Name)

		for _, dep := range l.RuntimeDeps {
			wg.Add(1)
			go func(flake, lspName string) {
				defer wg.Done()
				if _, err := executor.BuildStorePath(ctx, flake); err != nil {
					fmt.Fprintf(os.Stderr, "[lux] pre-build runtime dep %s for %s: %v\n", flake, lspName, err)
				}
			}(dep, l.Name)
		}
	}
	wg.Wait()
}
```

### Step 4: Run tests

Run: `go test ./internal/warmup/ -v`
Expected: PASS

### Step 5: Commit

```
feat(warmup): pre-build runtime deps alongside LSP flakes

Runtime deps are built in parallel with their parent LSP during
warmup, so the first tool call doesn't block on nix build.
```

---

## Task 5: Add `rebuild` MCP tool

**Files:**
- Modify: `internal/tools/registry.go`
- Create: `internal/tools/rebuild.go`
- Modify: `internal/tools/bridge.go` (expose rebuild capability)
- Test: `internal/tools/rebuild_test.go`

### Step 1: Write failing test

Create `internal/tools/rebuild_test.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRebuildHandler_ParsesArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantLSP  string
		wantAll  bool
	}{
		{"specific LSP", `{"lsp": "gopls"}`, "gopls", false},
		{"all LSPs", `{}`, "", true},
		{"explicit all", `{"lsp": ""}`, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a rebuildArgs
			if err := json.Unmarshal([]byte(tt.args), &a); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			if a.LSP != tt.wantLSP {
				t.Errorf("LSP: expected %q, got %q", tt.wantLSP, a.LSP)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

Run: `go test ./internal/tools/ -run TestRebuildHandler -v`
Expected: FAIL — `rebuildArgs` undefined

### Step 3: Implement rebuild tool

Create `internal/tools/rebuild.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/go-lib-mcp/command"
)

type rebuildArgs struct {
	LSP string `json:"lsp"`
}

func (b *Bridge) Rebuild(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var a rebuildArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	b.pool.Executor().ClearCache()

	var names []string
	if a.LSP != "" {
		names = []string{a.LSP}
	} else {
		names = b.pool.Names()
	}

	var stopped []string
	var errors []string

	for _, name := range names {
		if err := b.pool.Stop(name); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
		} else {
			stopped = append(stopped, name)
		}
	}

	var msg strings.Builder
	if len(stopped) > 0 {
		fmt.Fprintf(&msg, "Stopped and cleared cache for: %s\n", strings.Join(stopped, ", "))
		msg.WriteString("LSP servers will rebuild and restart on next tool call.")
	}
	if len(errors) > 0 {
		fmt.Fprintf(&msg, "\nErrors: %s", strings.Join(errors, "; "))
	}

	return command.TextResult(msg.String()), nil
}
```

Note: `b.pool.Executor()` method doesn't exist yet. Add to `Pool`:

```go
func (p *Pool) Executor() Executor {
	return p.executor
}
```

### Step 4: Register the rebuild tool

In `internal/tools/registry.go`, add in `RegisterAll`:

```go
	app.AddCommand(&command.Command{
		Name: "rebuild",
		Description: command.Description{
			Short: "Clear build caches and restart LSP servers. Use after updating flake inputs or changing lsps.toml. Call with {\"lsp\": \"gopls\"} to rebuild a specific LSP, or {} to rebuild all.",
		},
		Params: []command.Param{
			{Name: "lsp", Description: "LSP name to rebuild (empty = all)", Required: false},
		},
		Run: bridge.Rebuild,
	})
```

### Step 5: Run tests

Run: `go test ./internal/tools/ -run TestRebuildHandler -v`
Expected: PASS

### Step 6: Commit

```
feat: add rebuild MCP tool for cache clearing and LSP restart

Exposes a `rebuild` tool that clears nix build caches and stops
LSP servers so they restart fresh on next call. Enables updating
runtime deps without restarting the lux process.
```

---

## Task 6: Integration test — gopls with runtime_deps

This is a manual verification step. After all code changes:

### Step 1: Update lsps.toml

Add `runtime_deps` to gopls config in `~/.config/lux/lsps.toml`:

```toml
[[lsp]]
  name = "gopls"
  flake = "nixpkgs#gopls"
  extensions = ["go"]
  language_ids = ["go"]
  runtime_deps = ["nixpkgs#go"]

  [lsp.settings]
    gofumpt = true
    # ... rest unchanged
```

### Step 2: Build and install lux

Run from lux repo:
```bash
just build
```

Or: `nix build .#default`

### Step 3: Verify gopls works

Restart Claude Code (to pick up new lux binary), then test:

```
lux hover on a .go file
```

Expected: hover information returned instead of "no views"

### Step 4: Test rebuild tool

Call the rebuild MCP tool, then verify gopls restarts cleanly on next call.

### Step 5: Commit lsps.toml change (if desired)

Note: lsps.toml is user config, not committed to the repo. But document the recommended config in README.

---

## Task 7: Update README with runtime_deps documentation

**Files:**
- Modify: `README.md`

### Step 1: Add runtime_deps to config documentation

Add a section explaining:
- What `runtime_deps` does
- Why gopls needs it (go binary)
- Example config
- The `rebuild` tool

### Step 2: Commit

```
docs: document runtime_deps config and rebuild tool
```

---

## Execution Order

Tasks 1-5 are sequential (each builds on the previous). Task 6 is manual verification. Task 7 is docs.

Dependencies:
- Task 1 (env expansion): standalone
- Task 2 (config field): standalone
- Task 3 (pool wiring): depends on Task 2
- Task 4 (warmup): depends on Task 3
- Task 5 (rebuild tool): depends on Task 3
- Task 6 (integration test): depends on all above
- Task 7 (docs): depends on Task 6
