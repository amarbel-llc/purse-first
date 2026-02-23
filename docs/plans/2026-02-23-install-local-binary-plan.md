# install-local --binary Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `package.toml` to lux and make `install-local` smarter with a `--binary` flag that runs `_generate` before installing.

**Architecture:** Add an options struct to `InstallLocal` with a `Binary` field. When set, exec `go run ./cmd/<binary> _generate .claude-plugin/`, glob for the generated `plugin.json`, then proceed with existing install steps. Refactor `installMCPServers` to accept a plugin path instead of hardcoding it.

**Tech Stack:** Go, os/exec, filepath.Glob, tap-dancer TAP-14 writer

---

### Task 1: Create lux package.toml

**Files:**
- Create: `packages/lux/package.toml`

**Step 1: Create the file**

```toml
name = "lux"
description = "LSP Multiplexer that routes requests to language servers based on file type"

[author]
name = "friedenberg"

[mcp.lux]
command = "lux"
args = ["mcp-stdio"]
```

**Step 2: Commit**

```bash
git add packages/lux/package.toml
git commit -m "feat(lux): add package.toml"
```

---

### Task 2: Refactor installMCPServers to accept pluginPath

`installMCPServers` in `internal/localplugin/mcp.go:13` hardcodes the
plugin.json path. Change its signature so callers pass the path.

**Files:**
- Modify: `internal/localplugin/mcp.go:13` — change `installMCPServers(root, settingsPath string)` to `installMCPServers(pluginPath, settingsPath string)`
- Modify: `internal/localplugin/generate.go:81` — update the call site
- Test: `internal/localplugin/mcp_test.go` (existing tests)

**Step 1: Update installMCPServers signature**

In `internal/localplugin/mcp.go`, change:

```go
func installMCPServers(root, settingsPath string) (int, error) {
	pluginPath := filepath.Join(root, ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(pluginPath)
```

To:

```go
func installMCPServers(pluginPath, settingsPath string) (int, error) {
	data, err := os.ReadFile(pluginPath)
```

**Step 2: Update the call site in generate.go**

In `internal/localplugin/generate.go:81`, change:

```go
count, err := installMCPServers(root, settingsPath)
```

To:

```go
count, err := installMCPServers(pluginPath, settingsPath)
```

The `pluginPath` variable is already defined on line 70.

**Step 3: Run existing tests to verify refactor is safe**

Run: `nix develop --command go test -v ./internal/localplugin/`
Expected: all existing tests pass (TestInstallLocalSkillsAndHooks, TestInstallLocalWithMCPServers, mcp_test.go tests)

**Step 4: Commit**

```bash
git add internal/localplugin/mcp.go internal/localplugin/generate.go
git commit -m "refactor(localplugin): pass pluginPath to installMCPServers"
```

---

### Task 3: Add InstallLocalOptions and update InstallLocal signature

**Files:**
- Modify: `internal/localplugin/generate.go:65` — new options struct, updated signature, generate step
- Modify: `internal/localplugin/install_local_test.go` — update existing test calls
- Modify: `cmd/purse-first/main.go:145` — update call site

**Step 1: Write failing test — update existing tests for new signature**

In `internal/localplugin/install_local_test.go`, update both existing test
functions to pass `InstallLocalOptions{}`:

Line 28: `err := InstallLocal(&buf, root)` → `err := InstallLocal(&buf, root, InstallLocalOptions{})`
Line 85: `err := InstallLocal(&buf, root)` → `err := InstallLocal(&buf, root, InstallLocalOptions{})`

Run: `nix develop --command go test -v ./internal/localplugin/`
Expected: FAIL — `InstallLocal` doesn't accept 3 args yet

**Step 2: Update InstallLocal signature and add options struct**

In `internal/localplugin/generate.go`, add the options struct and update
`InstallLocal`. Replace lines 63-105 with:

```go
// InstallLocalOptions configures the install-local command.
type InstallLocalOptions struct {
	Binary string // Go binary name under cmd/, triggers _generate
}

// InstallLocal sets up the local development environment: optionally generates
// plugin.json via _generate, discovers skills, installs MCP servers, and
// registers hooks in project-scoped settings.
func InstallLocal(w io.Writer, root string, opts InstallLocalOptions) error {
	tw := tap.NewWriter(w)

	pluginPath := filepath.Join(root, ".claude-plugin", "plugin.json")

	if opts.Binary != "" {
		tw.PlanAhead(4)

		generatedPath, err := runGenerate(root, opts.Binary)
		if err != nil {
			tw.NotOk(fmt.Sprintf("generate plugin.json via _generate (%s)", opts.Binary), map[string]string{
				"error": err.Error(),
			})
			return err
		}
		tw.Ok(fmt.Sprintf("generate plugin.json via _generate (%s)", opts.Binary))
		pluginPath = generatedPath
	} else {
		tw.PlanAhead(3)
	}

	// Discover and update skills
	if err := Generate(root, pluginPath); err != nil {
		tw.NotOk("discover and update skills in plugin.json", map[string]string{
			"error": err.Error(),
		})
		return err
	}
	tw.Ok("discover and update skills in plugin.json")

	// Install MCP servers
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	count, err := installMCPServers(pluginPath, settingsPath)
	if err != nil {
		tw.NotOk("install MCP servers to .claude/settings.json", map[string]string{
			"error": err.Error(),
		})
		return err
	}
	if count == 0 {
		tw.Skip("install MCP servers to .claude/settings.json", "no mcpServers declared")
	} else {
		tw.Ok(fmt.Sprintf("install MCP servers to .claude/settings.json (%d server%s)", count, plural(count)))
	}

	// Install hooks
	binaryPath := "go run ./cmd/purse-first"
	if err := hook.Install(binaryPath, true); err != nil {
		tw.NotOk("install hooks to .claude/settings.json", map[string]string{
			"error": err.Error(),
		})
		return err
	}
	tw.Ok("install hooks to .claude/settings.json")

	return nil
}
```

Add the `runGenerate` function and `findGeneratedPlugin` helper (new file
`internal/localplugin/binary.go`):

```go
package localplugin

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// runGenerate execs "go run ./cmd/<binary> _generate <outDir>" and returns the
// path to the generated plugin.json found via glob.
func runGenerate(root, binary string) (string, error) {
	outDir := filepath.Join(root, ".claude-plugin")

	cmd := exec.Command("go", "run", "./cmd/"+binary, "_generate", outDir)
	cmd.Dir = root

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("running _generate: %w\n%s", err, output)
	}

	return findGeneratedPlugin(outDir)
}

// findGeneratedPlugin globs for plugin.json under the _generate output tree.
func findGeneratedPlugin(outDir string) (string, error) {
	pattern := filepath.Join(outDir, "share", "purse-first", "*", "plugin.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("globbing for plugin.json: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no plugin.json found under %s", outDir)
	}

	if len(matches) > 1 {
		return "", fmt.Errorf("multiple plugin.json found under %s: %v", outDir, matches)
	}

	return matches[0], nil
}
```

Also add `"os/exec"` won't be in generate.go — it stays in the new binary.go file.

**Step 3: Update CLI call site**

In `cmd/purse-first/main.go:145`, change:

```go
return localplugin.InstallLocal(os.Stderr, installLocalRoot)
```

To:

```go
return localplugin.InstallLocal(os.Stderr, installLocalRoot, localplugin.InstallLocalOptions{})
```

**Step 4: Run tests**

Run: `nix develop --command go test -v ./internal/localplugin/`
Expected: PASS — existing tests work with empty options

**Step 5: Commit**

```bash
git add internal/localplugin/generate.go internal/localplugin/binary.go internal/localplugin/install_local_test.go cmd/purse-first/main.go
git commit -m "feat(localplugin): add InstallLocalOptions with Binary field and _generate support"
```

---

### Task 4: Add test for --binary flow

**Files:**
- Modify: `internal/localplugin/install_local_test.go` — add TestInstallLocalWithBinary
- Create: `internal/localplugin/binary_test.go` — test findGeneratedPlugin

**Step 1: Write test for findGeneratedPlugin**

Create `internal/localplugin/binary_test.go`:

```go
package localplugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFindGeneratedPlugin(t *testing.T) {
	outDir := t.TempDir()

	// Simulate _generate output structure
	pluginDir := filepath.Join(outDir, "share", "purse-first", "lux")
	os.MkdirAll(pluginDir, 0o755)
	plugin := map[string]any{
		"name": "lux",
		"mcpServers": map[string]any{
			"lux": map[string]any{
				"type":    "stdio",
				"command": "lux",
				"args":    []any{"mcp-stdio"},
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	got, err := findGeneratedPlugin(outDir)
	if err != nil {
		t.Fatalf("findGeneratedPlugin: %v", err)
	}

	expected := filepath.Join(pluginDir, "plugin.json")
	if got != expected {
		t.Errorf("got %s, want %s", got, expected)
	}
}

func TestFindGeneratedPluginNoMatch(t *testing.T) {
	outDir := t.TempDir()

	_, err := findGeneratedPlugin(outDir)
	if err == nil {
		t.Error("expected error for empty dir")
	}
}
```

**Step 2: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestFindGenerated ./internal/localplugin/`
Expected: PASS

**Step 3: Write test for full --binary flow (pre-created output)**

Add to `internal/localplugin/install_local_test.go`:

```go
func TestInstallLocalWithBinaryPreCreated(t *testing.T) {
	root := t.TempDir()

	// Pre-create the _generate output structure (simulating what _generate produces)
	pluginDir := filepath.Join(root, ".claude-plugin", "share", "purse-first", "test-mcp")
	os.MkdirAll(pluginDir, 0o755)
	plugin := map[string]any{
		"name": "test-mcp",
		"mcpServers": map[string]any{
			"test-mcp": map[string]any{
				"type":    "stdio",
				"command": "test-mcp",
				"args":    []any{"mcp", "stdio"},
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	// Use a no-op binary that doesn't actually run (output is pre-created).
	// We override runGenerate via a test hook to skip the exec and just return
	// the path.
	var buf bytes.Buffer

	// For this test, directly test the flow after generation by calling the
	// internal pieces. The exec path is tested via integration.
	generatedPath, err := findGeneratedPlugin(filepath.Join(root, ".claude-plugin"))
	if err != nil {
		t.Fatalf("findGeneratedPlugin: %v", err)
	}

	// Verify the discovered path is correct
	if filepath.Base(filepath.Dir(generatedPath)) != "test-mcp" {
		t.Errorf("expected test-mcp dir, got %s", filepath.Dir(generatedPath))
	}

	// Now test that installMCPServers works with the generated path
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	count, err := installMCPServers(generatedPath, settingsPath)
	if err != nil {
		t.Fatalf("installMCPServers: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 server, got %d", count)
	}

	// Verify settings.json
	settingsData, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	json.Unmarshal(settingsData, &settings)

	mcpServers, _ := settings["mcpServers"].(map[string]any)
	if _, ok := mcpServers["test-mcp"]; !ok {
		t.Error("test-mcp not found in settings.json")
	}

	_ = buf // buf unused in this direct test
}
```

**Step 4: Run tests**

Run: `nix develop --command go test -v ./internal/localplugin/`
Expected: PASS — all tests including new ones

**Step 5: Commit**

```bash
git add internal/localplugin/binary_test.go internal/localplugin/install_local_test.go
git commit -m "test(localplugin): add tests for findGeneratedPlugin and binary flow"
```

---

### Task 5: Wire up --binary flag in CLI

**Files:**
- Modify: `cmd/purse-first/main.go:131-149`

**Step 1: Add --binary flag**

In `cmd/purse-first/main.go`, change the install-local block. Replace lines
131-149:

```go
var (
	installLocalRoot   string
	installLocalBinary string
)

installLocalCmd := &cobra.Command{
	Use:   "install-local",
	Short: "Set up local dev environment: skills, MCP servers, and hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		if installLocalRoot == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			installLocalRoot = cwd
		}

		return localplugin.InstallLocal(os.Stderr, installLocalRoot, localplugin.InstallLocalOptions{
			Binary: installLocalBinary,
		})
	},
}

installLocalCmd.Flags().StringVar(&installLocalRoot, "root", "", "repository root (defaults to cwd)")
installLocalCmd.Flags().StringVar(&installLocalBinary, "binary", "", "Go binary name under cmd/ to run _generate")
```

**Step 2: Verify it compiles**

Run: `nix develop --command go build ./cmd/purse-first`
Expected: builds cleanly

**Step 3: Commit**

```bash
git add cmd/purse-first/main.go
git commit -m "feat(cli): add --binary flag to install-local command"
```

---

### Task 6: Integration test — run install-local on lux

**Step 1: Run install-local with --binary on lux**

From the purse-first repo root:

```bash
nix develop --command go run ./cmd/purse-first install-local --root packages/lux --binary lux
```

Expected output (approximately):
```
TAP version 14
1..4
ok 1 - generate plugin.json via _generate (lux)
ok 2 - discover and update skills in plugin.json
ok 3 - install MCP servers to .claude/settings.json (1 server)
ok 4 - install hooks to .claude/settings.json
```

**Step 2: Verify generated files**

Check that `.claude-plugin/share/purse-first/lux/plugin.json` exists under
`packages/lux/` and contains `mcpServers` with a `lux` entry.

Check that `packages/lux/.claude/settings.json` has the MCP server rewritten to
`go run ./cmd/lux mcp-stdio`.

**Step 3: Clean up generated files**

The generated `.claude-plugin/` and `.claude/settings.json` under `packages/lux/`
are development artifacts. Add them to `.gitignore` if not already ignored, or
remove them.

**Step 4: Final commit if any fixups needed**

```bash
git add -A
git commit -m "fix: integration test fixups for install-local --binary"
```
