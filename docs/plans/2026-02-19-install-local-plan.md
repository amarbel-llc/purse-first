# install-local Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace `generate-local-plugin` with `install-local` — a single command that sets up skills, MCP servers, and hooks for local development.

**Architecture:** `install-local` orchestrates three steps in the `localplugin` package: (1) discover skills and update `plugin.json`, (2) read `mcpServers` from `plugin.json` and write them to project-scoped `.claude/settings.json` with `go run` commands, (3) install hooks to the same settings file. TAP-14 output via tap-dancer.

**Tech Stack:** Go, tap-dancer (`github.com/amarbel-llc/tap-dancer/go`), existing `hook.Install`

---

### Task 1: Add tap-dancer Go dependency

**Files:**
- Modify: `go.mod`
- Modify: `gomod2nix.toml`

**Step 1: Add the dependency**

Run:
```bash
nix develop --command go get github.com/amarbel-llc/tap-dancer/go@latest
```

**Step 2: Tidy modules**

Run:
```bash
nix develop --command go mod tidy
```

**Step 3: Regenerate gomod2nix.toml**

Run:
```bash
nix develop --command gomod2nix
```

**Step 4: Verify the dependency resolves**

Run:
```bash
nix develop --command go build ./...
```

Expected: exits 0, no errors.

**Step 5: Commit**

```bash
git add go.mod go.sum gomod2nix.toml
git commit -m "deps: add tap-dancer Go library"
```

---

### Task 2: Write MCP-to-settings installation logic

**Files:**
- Create: `internal/localplugin/mcp.go`
- Create: `internal/localplugin/mcp_test.go`

**Step 1: Write the failing test**

Create `internal/localplugin/mcp_test.go`:

```go
package localplugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallMCPServersWritesToSettings(t *testing.T) {
	root := t.TempDir()

	// Create plugin.json with an MCP server
	pluginDir := filepath.Join(root, ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)

	plugin := map[string]any{
		"name": "test-plugin",
		"mcpServers": map[string]any{
			"test-plugin": map[string]any{
				"type":    "stdio",
				"command": "test-plugin",
				"args":    []any{"mcp", "stdio"},
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	// Create settings directory
	settingsDir := filepath.Join(root, ".claude")
	os.MkdirAll(settingsDir, 0o755)
	settingsPath := filepath.Join(settingsDir, "settings.json")

	count, err := installMCPServers(root, settingsPath)
	if err != nil {
		t.Fatalf("installMCPServers: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 server installed, got %d", count)
	}

	// Verify settings.json
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		t.Fatalf("parsing settings.json: %v", err)
	}

	mcpServers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers not found in settings.json")
	}

	server, ok := mcpServers["test-plugin"].(map[string]any)
	if !ok {
		t.Fatal("test-plugin server not found")
	}

	if server["command"] != "go" {
		t.Errorf("command = %q, want \"go\"", server["command"])
	}

	args, _ := server["args"].([]any)
	if len(args) < 2 || args[0] != "run" || args[1] != "./cmd/test-plugin" {
		t.Errorf("args = %v, want [run ./cmd/test-plugin mcp stdio]", args)
	}
}

func TestInstallMCPServersNoServers(t *testing.T) {
	root := t.TempDir()

	pluginDir := filepath.Join(root, ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)

	plugin := map[string]any{"name": "skill-only"}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	settingsPath := filepath.Join(root, ".claude", "settings.json")

	count, err := installMCPServers(root, settingsPath)
	if err != nil {
		t.Fatalf("installMCPServers: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 servers, got %d", count)
	}
}

func TestInstallMCPServersPreservesExistingSettings(t *testing.T) {
	root := t.TempDir()

	// Create plugin.json with an MCP server
	pluginDir := filepath.Join(root, ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)

	plugin := map[string]any{
		"name": "my-mcp",
		"mcpServers": map[string]any{
			"my-mcp": map[string]any{
				"type":    "stdio",
				"command": "my-mcp",
				"args":    []any{"serve"},
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	// Create existing settings.json with other content
	settingsDir := filepath.Join(root, ".claude")
	os.MkdirAll(settingsDir, 0o755)
	settingsPath := filepath.Join(settingsDir, "settings.json")

	existing := map[string]any{
		"permissions": map[string]any{"allow": []string{"Read"}},
		"mcpServers": map[string]any{
			"other-server": map[string]any{
				"command": "other",
				"args":    []any{},
			},
		},
	}
	existingData, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(settingsPath, existingData, 0o644)

	count, err := installMCPServers(root, settingsPath)
	if err != nil {
		t.Fatalf("installMCPServers: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 server, got %d", count)
	}

	settingsData, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	json.Unmarshal(settingsData, &settings)

	// Check existing server preserved
	mcpServers := settings["mcpServers"].(map[string]any)
	if _, ok := mcpServers["other-server"]; !ok {
		t.Error("existing other-server was removed")
	}

	// Check permissions preserved
	if _, ok := settings["permissions"]; !ok {
		t.Error("existing permissions were removed")
	}
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
nix develop --command go test ./internal/localplugin/... -run TestInstallMCP -v
```

Expected: FAIL — `installMCPServers` undefined.

**Step 3: Write the implementation**

Create `internal/localplugin/mcp.go`:

```go
package localplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// installMCPServers reads mcpServers from plugin.json and writes them to
// settingsPath (.claude/settings.json) with commands rewritten to use
// "go run ./cmd/<name>".
func installMCPServers(root, settingsPath string) (int, error) {
	pluginPath := filepath.Join(root, ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		return 0, fmt.Errorf("reading plugin.json: %w", err)
	}

	var plugin map[string]any
	if err := json.Unmarshal(data, &plugin); err != nil {
		return 0, fmt.Errorf("parsing plugin.json: %w", err)
	}

	mcpServersRaw, ok := plugin["mcpServers"].(map[string]any)
	if !ok || len(mcpServersRaw) == 0 {
		return 0, nil
	}

	// Read existing settings
	settings := make(map[string]any)
	if settingsData, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(settingsData, &settings)
	}

	existingServers, _ := settings["mcpServers"].(map[string]any)
	if existingServers == nil {
		existingServers = make(map[string]any)
	}

	count := 0
	for name, serverRaw := range mcpServersRaw {
		serverMap, ok := serverRaw.(map[string]any)
		if !ok {
			continue
		}

		// Rewrite command to "go run ./cmd/<name>"
		goArgs := []any{"run", "./cmd/" + name}

		// Append original args
		if origArgs, ok := serverMap["args"].([]any); ok {
			goArgs = append(goArgs, origArgs...)
		}

		existingServers[name] = map[string]any{
			"type":    "stdio",
			"command": "go",
			"args":    goArgs,
			"env":     map[string]any{},
		}
		count++
	}

	settings["mcpServers"] = existingServers

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return 0, fmt.Errorf("creating settings directory: %w", err)
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("marshaling settings: %w", err)
	}
	out = append(out, '\n')

	return count, os.WriteFile(settingsPath, out, 0o644)
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
nix develop --command go test ./internal/localplugin/... -run TestInstallMCP -v
```

Expected: PASS (all 3 tests).

**Step 5: Commit**

```bash
git add internal/localplugin/mcp.go internal/localplugin/mcp_test.go
git commit -m "feat(localplugin): add MCP server installation to settings.json"
```

---

### Task 3: Write InstallLocal orchestrator with TAP-14 output

**Files:**
- Modify: `internal/localplugin/generate.go`
- Create: `internal/localplugin/install_local_test.go`

**Step 1: Write the failing test**

Create `internal/localplugin/install_local_test.go`:

```go
package localplugin

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallLocalSkillsAndHooks(t *testing.T) {
	root := t.TempDir()

	// Create a skill
	skillDir := filepath.Join(root, "skills", "my-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill"), 0o644)

	// Create plugin.json (no MCP servers)
	pluginDir := filepath.Join(root, ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)
	plugin := map[string]any{"name": "test-pkg"}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	var buf bytes.Buffer
	err := InstallLocal(&buf, root)
	if err != nil {
		t.Fatalf("InstallLocal: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "TAP version 14") {
		t.Error("missing TAP version header")
	}
	if !strings.Contains(output, "1..3") {
		t.Error("missing test plan")
	}
	if !strings.Contains(output, "ok 1") {
		t.Error("missing ok 1 for skills")
	}
	// MCP should be skipped (no servers)
	if !strings.Contains(output, "ok 2") {
		t.Error("missing ok 2 for MCP")
	}
	if !strings.Contains(output, "# SKIP") {
		t.Error("MCP step should be SKIP when no servers declared")
	}
	if !strings.Contains(output, "ok 3") {
		t.Error("missing ok 3 for hooks")
	}

	// Verify plugin.json was updated with skills
	pluginData, _ := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	var got map[string]any
	json.Unmarshal(pluginData, &got)
	skills, _ := got["skills"].([]any)
	if len(skills) != 1 {
		t.Errorf("expected 1 skill in plugin.json, got %d", len(skills))
	}
}

func TestInstallLocalWithMCPServers(t *testing.T) {
	root := t.TempDir()

	// Create plugin.json with MCP server
	pluginDir := filepath.Join(root, ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)
	plugin := map[string]any{
		"name": "my-mcp",
		"mcpServers": map[string]any{
			"my-mcp": map[string]any{
				"type":    "stdio",
				"command": "my-mcp",
				"args":    []any{"mcp", "stdio"},
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	var buf bytes.Buffer
	err := InstallLocal(&buf, root)
	if err != nil {
		t.Fatalf("InstallLocal: %v", err)
	}

	output := buf.String()

	// MCP step should NOT be skipped
	if strings.Contains(output, "# SKIP") {
		t.Error("MCP step should not be SKIP when servers are declared")
	}
	if !strings.Contains(output, "1 server") {
		t.Error("expected '1 server' in MCP step description")
	}

	// Verify .claude/settings.json was created with MCP entry
	settingsData, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	var settings map[string]any
	json.Unmarshal(settingsData, &settings)

	mcpServers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers not in settings.json")
	}

	if _, ok := mcpServers["my-mcp"]; !ok {
		t.Error("my-mcp not found in settings.json mcpServers")
	}
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
nix develop --command go test ./internal/localplugin/... -run TestInstallLocal -v
```

Expected: FAIL — `InstallLocal` undefined.

**Step 3: Write the implementation**

Add to `internal/localplugin/generate.go`:

```go
// At top, add to imports:
// "io"
// "fmt"
// "path/filepath"
// tap "github.com/amarbel-llc/tap-dancer/go"
// "github.com/amarbel-llc/purse-first/internal/hook"

// InstallLocal sets up the local development environment: discovers skills,
// installs MCP servers, and registers hooks in project-scoped settings.
func InstallLocal(w io.Writer, root string) error {
	tw := tap.NewWriter(w)
	tw.PlanAhead(3)

	// 1. Discover and update skills
	pluginPath := filepath.Join(root, ".claude-plugin", "plugin.json")
	if err := Generate(root, pluginPath); err != nil {
		tw.NotOk("discover and update skills in plugin.json", map[string]string{
			"error": err.Error(),
		})
		return err
	}
	tw.Ok("discover and update skills in plugin.json")

	// 2. Install MCP servers
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	count, err := installMCPServers(root, settingsPath)
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

	// 3. Install hooks
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

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
nix develop --command go test ./internal/localplugin/... -run TestInstallLocal -v
```

Expected: PASS (both tests).

**Step 5: Run all localplugin tests**

Run:
```bash
nix develop --command go test ./internal/localplugin/... -v
```

Expected: PASS (all existing + new tests).

**Step 6: Commit**

```bash
git add internal/localplugin/generate.go internal/localplugin/install_local_test.go
git commit -m "feat(localplugin): add InstallLocal orchestrator with TAP-14 output"
```

---

### Task 4: Replace CLI command and update justfile

**Files:**
- Modify: `cmd/purse-first/main.go`
- Modify: `justfile`

**Step 1: Replace the CLI command**

In `cmd/purse-first/main.go`, replace the `genLocalPluginCmd` block (lines 119-143) with:

```go
	var installLocalRoot string

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

			return localplugin.InstallLocal(os.Stderr, installLocalRoot)
		},
	}

	installLocalCmd.Flags().StringVar(&installLocalRoot, "root", "", "repository root (defaults to cwd)")
```

Also update the `root.AddCommand(...)` call to replace `genLocalPluginCmd` with `installLocalCmd`.

Remove the `localPluginRoot` variable declaration (line 119).

**Step 2: Remove `generate-local-plugin` from justfile**

Remove lines 113-115:

```
# Generate local plugin.json with discovered skills
generate-local-plugin:
    nix develop --command go run ./cmd/purse-first generate-local-plugin
```

**Step 3: Verify the CLI builds**

Run:
```bash
nix develop --command go build ./cmd/purse-first
```

Expected: exits 0, produces `purse-first` binary.

**Step 4: Verify the new command is registered**

Run:
```bash
nix develop --command go run ./cmd/purse-first install-local --help
```

Expected: shows usage for `install-local` with `--root` flag.

**Step 5: Verify old command is gone**

Run:
```bash
nix develop --command go run ./cmd/purse-first generate-local-plugin 2>&1 || true
```

Expected: `unknown command "generate-local-plugin"`.

**Step 6: Run full test suite**

Run:
```bash
nix develop --command go test ./... -v
```

Expected: PASS.

**Step 7: Commit**

```bash
git add cmd/purse-first/main.go justfile
git commit -m "feat: replace generate-local-plugin with install-local command"
```

---

### Task 5: Update mkMarketplace.nix reference

**Files:**
- Modify: `lib/mkMarketplace.nix`

The `mkMarketplace.nix` calls `generate-local-plugin` during the Nix build. This must be updated.

**Step 1: Find the reference**

In `lib/mkMarketplace.nix`, find the line calling `generate-local-plugin` and check whether it should use `install-local` or keep using `generate-local-plugin`.

Since the Nix build context only needs skill discovery (not MCP install or hooks — those are runtime concerns), and `generate-local-plugin` is being removed as a CLI command, we need to decide:
- Option A: Keep `generate-local-plugin` as a hidden/internal CLI command for Nix builds
- Option B: Have `install-local` accept a `--skills-only` flag
- Option C: Have the Nix build call `install-local` (hooks and MCP writes would fail harmlessly or be no-ops in the Nix sandbox)

Read `lib/mkMarketplace.nix` to determine the usage pattern, then apply the simplest fix.

**Step 2: Check the Nix build reference**

Read `lib/mkMarketplace.nix` and find the `generate-local-plugin` call. The Nix sandbox does not have a home directory or `.claude/` path, so hooks and MCP install would fail. The safest approach is to keep `Generate()` as the Go function (already preserved) and add a `--skills-only` flag to `install-local` that only runs step 1.

Alternatively: rename the CLI command but keep both `generate-local-plugin` (hidden alias) and `install-local` to avoid a breaking change in the Nix build.

**Step 3: Implement the solution**

Add `generate-local-plugin` as a hidden alias in `cmd/purse-first/main.go`:

```go
	// Hidden alias for Nix build compatibility
	genLocalPluginCmd := &cobra.Command{
		Use:    "generate-local-plugin",
		Hidden: true,
		Short:  "Deprecated: use install-local",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := installLocalRoot
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}
				root = cwd
			}

			pluginPath := filepath.Join(root, ".claude-plugin", "plugin.json")
			if err := localplugin.Generate(root, pluginPath); err != nil {
				return fmt.Errorf("generating local package manifest: %w", err)
			}

			fmt.Fprintf(os.Stderr, "updated %s\n", pluginPath)
			return nil
		},
	}

	genLocalPluginCmd.Flags().StringVar(&installLocalRoot, "root", "", "repository root (defaults to cwd)")
```

Add `genLocalPluginCmd` to `root.AddCommand(...)`.

**Step 4: Verify Nix build**

Run:
```bash
nix build --show-trace
```

Expected: builds successfully.

**Step 5: Commit**

```bash
git add cmd/purse-first/main.go
git commit -m "fix: keep generate-local-plugin as hidden alias for Nix build compat"
```

---

### Task 6: Verify end-to-end

**Step 1: Clean build**

Run:
```bash
rm -rf result && nix build
```

Expected: exits 0.

**Step 2: Run full test suite**

Run:
```bash
nix develop --command go test ./... -v
```

Expected: all tests pass.

**Step 3: Run install-local from the repo root**

Run:
```bash
nix develop --command go run ./cmd/purse-first install-local
```

Expected: TAP-14 output showing skills discovered, MCP skipped (bob has no mcpServers), hooks installed.

**Step 4: Verify .claude/settings.json has hooks**

Run:
```bash
cat .claude/settings.json
```

Expected: contains `hooks` section with `purse-first` entries.

**Step 5: Final commit if any fixups needed**

Only if adjustments were made during verification.
