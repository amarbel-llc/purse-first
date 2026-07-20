# Dev MCP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace `MapsBash` with `MapsTools`, extend `GenerateMappings()` to support file-extension-based interception, add `tool_prefix` to the mapping/hook system, and add a library-provided `dev-mcp` command.

**Architecture:** The `command.Command` struct gets a unified `MapsTools` field replacing `MapsBash`. `GenerateMappings()` produces entries with `extensions` and `command_prefixes`. The hook handler uses an optional `tool_prefix` from mapping files to format tool names. A hidden `dev-mcp` command is auto-registered on every `command.App`, reading build artifacts and generating project-local `.mcp.json`, `.purse-first/`, and `.claude/settings.json`.

**Tech Stack:** Go, purse-first go-mcp/command library, JSON

---

### Task 1: Replace `BashMapping` with `ToolMapping` in command.go

**Files:**
- Modify: `libs/go-mcp/command/command.go:43-71`

**Step 1: Write the failing test**

No new test file needed — existing tests in `generate_mappings_test.go` and `cli_test.go` will fail after the type rename.

**Step 2: Replace `BashMapping` with `ToolMapping` and `MapsBash` with `MapsTools`**

In `libs/go-mcp/command/command.go`, replace:

```go
// BashMapping declares a bash command prefix that should be intercepted
// and redirected to this command's MCP tool.
type BashMapping struct {
	Prefixes []string // e.g., "git status"
	UseWhen  string   // shown to Claude in mapping denial
}
```

with:

```go
// ToolMapping declares that this command's MCP tool should intercept
// a specific Claude Code tool under certain conditions.
type ToolMapping struct {
	Replaces        string   // Claude Code tool to intercept: "Read", "Grep", "Glob", "Bash"
	Extensions      []string // file extensions to match, e.g. [".go", ".py"]
	CommandPrefixes []string // bash command prefixes, e.g. ["git status"]
	UseWhen         string   // shown to Claude in denial reason
}
```

And in the `Command` struct, replace:

```go
	MapsBash []BashMapping
```

with:

```go
	MapsTools []ToolMapping
```

**Step 3: Fix compilation errors in generate_mappings.go**

In `libs/go-mcp/command/generate_mappings.go`, replace `cmd.MapsBash` with `cmd.MapsTools` and update the loop to use `ToolMapping` fields:

```go
func (a *App) GenerateMappings(dir string) error {
	var entries []mappingEntry

	for _, cmd := range a.AllCommands() {
		if cmd.Hidden {
			continue
		}
		for _, tm := range cmd.MapsTools {
			entries = append(entries, mappingEntry{
				Replaces:        tm.Replaces,
				Extensions:      tm.Extensions,
				CommandPrefixes: tm.CommandPrefixes,
				Tools: []mappingToolSuggestion{
					{Name: cmd.Name, UseWhen: tm.UseWhen},
				},
				Reason: "Use the " + a.Name + " MCP tool instead",
			})
		}
	}

	if len(entries) == 0 {
		return nil
	}

	mf := mappingFile{
		Server:   a.Name,
		Mappings: entries,
	}

	pluginDir := filepath.Join(dir, a.Name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(filepath.Join(pluginDir, "mappings.json"), data, 0o644)
}
```

Also add `Extensions` to `mappingEntry`:

```go
type mappingEntry struct {
	Replaces        string                  `json:"replaces"`
	Extensions      []string                `json:"extensions,omitempty"`
	CommandPrefixes []string                `json:"command_prefixes,omitempty"`
	Tools           []mappingToolSuggestion `json:"tools"`
	Reason          string                  `json:"reason"`
}
```

**Step 4: Update generate.go doc comment**

In `libs/go-mcp/command/generate.go`, replace `MapsBash` with `MapsTools` in the comment on line 11.

**Step 5: Fix test references**

In `libs/go-mcp/command/generate_mappings_test.go`, replace all `MapsBash: []BashMapping{` with `MapsTools: []ToolMapping{` and add `Replaces: "Bash"` to each entry:

```go
MapsTools: []ToolMapping{
	{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking repository status"},
},
```

In `libs/go-mcp/command/cli_test.go`, apply the same transformation.

**Step 6: Run tests to verify**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && go test ./libs/go-mcp/command/...`
Expected: PASS

**Step 7: Commit**

```
git add libs/go-mcp/command/command.go libs/go-mcp/command/generate_mappings.go libs/go-mcp/command/generate_mappings_test.go libs/go-mcp/command/generate.go libs/go-mcp/command/cli_test.go
git commit -m "Replace MapsBash with MapsTools for unified tool interception"
```

---

### Task 2: Add `Extensions` support to `GenerateMappings` tests

**Files:**
- Modify: `libs/go-mcp/command/generate_mappings_test.go`

**Step 1: Write the failing test**

Add a new test to `libs/go-mcp/command/generate_mappings_test.go`:

```go
func TestGenerateMappingsWithExtensions(t *testing.T) {
	app := NewApp("lux", "LSP multiplexer")

	app.AddCommand(&Command{
		Name:        "hover",
		Description: Description{Short: "Get type info"},
		MapsTools: []ToolMapping{
			{Replaces: "Read", Extensions: []string{".go", ".py"}, UseWhen: "getting type info"},
		},
	})

	app.AddCommand(&Command{
		Name:        "document_symbols",
		Description: Description{Short: "Get symbols"},
		MapsTools: []ToolMapping{
			{Replaces: "Read", Extensions: []string{".go", ".py"}, UseWhen: "understanding file structure"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateMappings(dir); err != nil {
		t.Fatalf("GenerateMappings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "lux", "mappings.json"))
	if err != nil {
		t.Fatalf("read mappings.json: %v", err)
	}

	var mf struct {
		Server   string `json:"server"`
		Mappings []struct {
			Replaces   string   `json:"replaces"`
			Extensions []string `json:"extensions"`
			Tools      []struct {
				Name    string `json:"name"`
				UseWhen string `json:"use_when"`
			} `json:"tools"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(data, &mf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if mf.Server != "lux" {
		t.Errorf("server = %q, want lux", mf.Server)
	}

	// Two separate entries (no consolidation yet — one per MapsTools entry)
	if len(mf.Mappings) != 2 {
		t.Fatalf("mappings len = %d, want 2", len(mf.Mappings))
	}

	for _, m := range mf.Mappings {
		if m.Replaces != "Read" {
			t.Errorf("replaces = %q, want Read", m.Replaces)
		}
		if len(m.Extensions) != 2 || m.Extensions[0] != ".go" {
			t.Errorf("extensions = %v, want [.go .py]", m.Extensions)
		}
	}
}
```

**Step 2: Run test to verify it passes**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && go test ./libs/go-mcp/command/... -run TestGenerateMappingsWithExtensions -v`
Expected: PASS (the implementation from Task 1 already handles extensions)

**Step 3: Commit**

```
git add libs/go-mcp/command/generate_mappings_test.go
git commit -m "Add test for GenerateMappings with file extension mappings"
```

---

### Task 3: Add `ToolPrefix` to mapping types and handler

**Files:**
- Modify: `internal/mapping/types.go:16-19`
- Modify: `internal/mapping/loader.go:118-121`
- Modify: `internal/hook/handler.go:41-52,80-92`

**Step 1: Write the failing test**

Create a test in `internal/hook/handler_test.go` (or add to existing). First check if it exists:

File: `internal/hook/handler_test.go` — if it doesn't exist, create it:

```go
package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFormatDenyReasonUsesToolPrefix(t *testing.T) {
	projectDir := t.TempDir()
	purseDir := filepath.Join(projectDir, ".purse-first")
	os.MkdirAll(purseDir, 0o755)

	mapping := `{
		"server": "lux",
		"tool_prefix": "mcp__lux-dev",
		"mappings": [{
			"replaces": "Read",
			"extensions": [".go"],
			"tools": [{"name": "hover", "use_when": "getting type info"}],
			"reason": "Use lux-dev"
		}]
	}`
	os.WriteFile(filepath.Join(purseDir, "lux.json"), []byte(mapping), 0o644)

	input := `{"session_id":"test","tool_name":"Read","tool_input":{"file_path":"/path/to/foo.go"},"hook_event_name":"PreToolUse"}`

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader([]byte(input)), &stdout, projectDir)
	if err != nil {
		t.Fatalf("HandlePreToolUse: %v", err)
	}

	var output struct {
		HookSpecificOutput struct {
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	reason := output.HookSpecificOutput.PermissionDecisionReason
	if !bytes.Contains([]byte(reason), []byte("mcp__lux-dev__hover")) {
		t.Errorf("reason = %q, want to contain mcp__lux-dev__hover", reason)
	}
}

func TestFormatDenyReasonDefaultPrefix(t *testing.T) {
	projectDir := t.TempDir()
	purseDir := filepath.Join(projectDir, ".purse-first")
	os.MkdirAll(purseDir, 0o755)

	mapping := `{
		"server": "lux",
		"mappings": [{
			"replaces": "Read",
			"extensions": [".go"],
			"tools": [{"name": "hover", "use_when": "getting type info"}],
			"reason": "Use lux"
		}]
	}`
	os.WriteFile(filepath.Join(purseDir, "lux.json"), []byte(mapping), 0o644)

	input := `{"session_id":"test","tool_name":"Read","tool_input":{"file_path":"/path/to/foo.go"},"hook_event_name":"PreToolUse"}`

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader([]byte(input)), &stdout, projectDir)
	if err != nil {
		t.Fatalf("HandlePreToolUse: %v", err)
	}

	var output struct {
		HookSpecificOutput struct {
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	reason := output.HookSpecificOutput.PermissionDecisionReason
	if !bytes.Contains([]byte(reason), []byte("mcp__plugin_lux_lux__hover")) {
		t.Errorf("reason = %q, want to contain mcp__plugin_lux_lux__hover", reason)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && go test ./internal/hook/... -run TestFormatDenyReason -v`
Expected: FAIL — `ToolPrefix` field doesn't exist yet, `formatDenyReason` ignores it.

**Step 3: Add `ToolPrefix` to `MappingFile`**

In `internal/mapping/types.go`, change:

```go
type MappingFile struct {
	Server   string    `json:"server"`
	Mappings []Mapping `json:"mappings"`
}
```

to:

```go
type MappingFile struct {
	Server     string    `json:"server"`
	ToolPrefix string    `json:"tool_prefix,omitempty"`
	Mappings   []Mapping `json:"mappings"`
}
```

**Step 4: Add `ToolPrefix` to `Match`**

In `internal/mapping/loader.go`, change the `Match` struct:

```go
type Match struct {
	Server  string
	Mapping Mapping
}
```

to:

```go
type Match struct {
	Server     string
	ToolPrefix string
	Mapping    Mapping
}
```

And in `FindMatch`, populate it:

```go
func FindMatch(files []MappingFile, toolName, filePath, command string) *Match {
	for _, f := range files {
		for _, m := range f.Mappings {
			if m.Replaces != toolName {
				continue
			}

			if matchesCriteria(m, filePath, command) {
				return &Match{Server: f.Server, ToolPrefix: f.ToolPrefix, Mapping: m}
			}
		}
	}

	return nil
}
```

**Step 5: Update `formatDenyReason` and `HandlePreToolUse`**

In `internal/hook/handler.go`, change `formatDenyReason`:

```go
func formatDenyReason(toolPrefix string, m mapping.Mapping) string {
	var b strings.Builder

	b.WriteString(m.Reason)
	b.WriteString(":\n")

	for _, t := range m.Tools {
		fmt.Fprintf(&b, "- %s__%s: %s\n", toolPrefix, t.Name, t.UseWhen)
	}

	return strings.TrimRight(b.String(), "\n")
}
```

And update the call site in `HandlePreToolUse`:

```go
	match := mapping.FindMatch(files, input.ToolName, filePath, command)
	if match == nil {
		// No matching rule → passthrough
		return nil
	}

	prefix := match.ToolPrefix
	if prefix == "" {
		prefix = fmt.Sprintf("mcp__plugin_%s_%s", match.Server, match.Server)
	}

	output := decision.HookOutput{
		HookSpecificOutput: decision.HookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "deny",
			PermissionDecisionReason: formatDenyReason(prefix, match.Mapping),
		},
	}
```

**Step 6: Run tests to verify**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && go test ./internal/hook/... ./internal/mapping/... -v`
Expected: PASS

**Step 7: Commit**

```
git add internal/mapping/types.go internal/mapping/loader.go internal/hook/handler.go internal/hook/handler_test.go
git commit -m "Add tool_prefix to mapping files for configurable MCP tool name format"
```

---

### Task 4: Add `dev-mcp` command to `command.App`

**Files:**
- Create: `libs/go-mcp/command/dev_mcp.go`
- Create: `libs/go-mcp/command/dev_mcp_test.go`
- Modify: `libs/go-mcp/command/app.go` (register the hidden command)

**Step 1: Write the failing test**

Create `libs/go-mcp/command/dev_mcp_test.go`:

```go
package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDevMCPGeneratesArtifacts(t *testing.T) {
	// Simulate a nix build output directory
	buildDir := t.TempDir()

	// Create bin/
	binDir := filepath.Join(buildDir, "bin")
	os.MkdirAll(binDir, 0o755)
	os.WriteFile(filepath.Join(binDir, "lux"), []byte("#!/bin/sh\n"), 0o755)

	// Create share/purse-first/lux/plugin.json
	pluginDir := filepath.Join(buildDir, "share", "purse-first", "lux")
	os.MkdirAll(pluginDir, 0o755)

	plugin := pluginManifest{
		Name: "lux",
		McpServers: map[string]pluginMcpServer{
			"lux": {Type: "stdio", Command: "lux", Args: []string{"mcp", "stdio"}},
		},
	}
	pluginData, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), pluginData, 0o644)

	// Create share/purse-first/lux/mappings.json
	mappings := mappingFile{
		Server: "lux",
		Mappings: []mappingEntry{
			{
				Replaces:   "Read",
				Extensions: []string{".go"},
				Tools:      []mappingToolSuggestion{{Name: "hover", UseWhen: "getting type info"}},
				Reason:     "Use the lux MCP tool instead",
			},
		},
	}
	mappingsData, _ := json.MarshalIndent(mappings, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "mappings.json"), mappingsData, 0o644)

	// Run dev-mcp generation
	projectDir := t.TempDir()
	err := generateDevMCP(buildDir, projectDir, "dev")
	if err != nil {
		t.Fatalf("generateDevMCP: %v", err)
	}

	// Verify .mcp.json
	mcpData, err := os.ReadFile(filepath.Join(projectDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}

	var mcpConfig struct {
		McpServers map[string]struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(mcpData, &mcpConfig); err != nil {
		t.Fatalf("unmarshal .mcp.json: %v", err)
	}

	server, ok := mcpConfig.McpServers["lux-dev"]
	if !ok {
		t.Fatal("expected lux-dev server in .mcp.json")
	}
	if server.Type != "stdio" {
		t.Errorf("type = %q, want stdio", server.Type)
	}
	expectedBin := filepath.Join(buildDir, "bin", "lux")
	if server.Command != expectedBin {
		t.Errorf("command = %q, want %q", server.Command, expectedBin)
	}

	// Verify .purse-first/lux.json
	purseData, err := os.ReadFile(filepath.Join(projectDir, ".purse-first", "lux.json"))
	if err != nil {
		t.Fatalf("read .purse-first/lux.json: %v", err)
	}

	var purseMappings struct {
		Server     string `json:"server"`
		ToolPrefix string `json:"tool_prefix"`
	}
	if err := json.Unmarshal(purseData, &purseMappings); err != nil {
		t.Fatalf("unmarshal .purse-first/lux.json: %v", err)
	}

	if purseMappings.ToolPrefix != "mcp__lux-dev" {
		t.Errorf("tool_prefix = %q, want mcp__lux-dev", purseMappings.ToolPrefix)
	}
}

func TestDevMCPClean(t *testing.T) {
	projectDir := t.TempDir()

	// Create the artifacts
	os.WriteFile(filepath.Join(projectDir, ".mcp.json"), []byte("{}"), 0o644)
	os.MkdirAll(filepath.Join(projectDir, ".purse-first"), 0o755)
	os.WriteFile(filepath.Join(projectDir, ".purse-first", "lux.json"), []byte("{}"), 0o644)

	err := cleanDevMCP(projectDir)
	if err != nil {
		t.Fatalf("cleanDevMCP: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".mcp.json")); !os.IsNotExist(err) {
		t.Error(".mcp.json should be removed")
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".purse-first")); !os.IsNotExist(err) {
		t.Error(".purse-first/ should be removed")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && go test ./libs/go-mcp/command/... -run TestDevMCP -v`
Expected: FAIL — `generateDevMCP` and `cleanDevMCP` don't exist.

**Step 3: Implement `dev_mcp.go`**

Create `libs/go-mcp/command/dev_mcp.go`:

```go
package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type devMcpConfig struct {
	McpServers map[string]pluginMcpServer `json:"mcpServers"`
}

type devMappingFile struct {
	Server     string         `json:"server"`
	ToolPrefix string         `json:"tool_prefix"`
	Mappings   []mappingEntry `json:"mappings"`
}

func resolveBuildDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving executable: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks: %w", err)
	}

	// Binary is at {buildDir}/bin/{name}, so buildDir is two levels up
	return filepath.Dir(filepath.Dir(resolved)), nil
}

func generateDevMCP(buildDir, projectDir, suffix string) error {
	// Read plugin.json
	pluginDirs, err := filepath.Glob(filepath.Join(buildDir, "share", "purse-first", "*", "plugin.json"))
	if err != nil || len(pluginDirs) == 0 {
		return fmt.Errorf("no plugin.json found in %s/share/purse-first/*/", buildDir)
	}

	pluginPath := pluginDirs[0]
	pluginData, err := os.ReadFile(pluginPath)
	if err != nil {
		return fmt.Errorf("reading plugin.json: %w", err)
	}

	var manifest pluginManifest
	if err := json.Unmarshal(pluginData, &manifest); err != nil {
		return fmt.Errorf("parsing plugin.json: %w", err)
	}

	name := manifest.Name
	serverKey := name + "-" + suffix

	// Find the binary
	binDir := filepath.Join(buildDir, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("no binaries found in %s", binDir)
	}
	binaryPath := filepath.Join(binDir, entries[0].Name())

	// Get MCP args from the plugin manifest
	var mcpArgs []string
	for _, server := range manifest.McpServers {
		mcpArgs = server.Args
		break
	}

	// 1. Write .mcp.json
	mcpConfig := devMcpConfig{
		McpServers: map[string]pluginMcpServer{
			serverKey: {
				Type:    "stdio",
				Command: binaryPath,
				Args:    mcpArgs,
			},
		},
	}

	mcpData, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling .mcp.json: %w", err)
	}
	mcpData = append(mcpData, '\n')

	if err := os.WriteFile(filepath.Join(projectDir, ".mcp.json"), mcpData, 0o644); err != nil {
		return fmt.Errorf("writing .mcp.json: %w", err)
	}

	// 2. Write .purse-first/<name>.json with tool_prefix
	mappingsPath := filepath.Join(buildDir, "share", "purse-first", name, "mappings.json")
	mappingsData, err := os.ReadFile(mappingsPath)
	if err == nil {
		var sourceMappings mappingFile
		if err := json.Unmarshal(mappingsData, &sourceMappings); err == nil {
			devMappings := devMappingFile{
				Server:     sourceMappings.Server,
				ToolPrefix: "mcp__" + serverKey,
				Mappings:   sourceMappings.Mappings,
			}

			purseDir := filepath.Join(projectDir, ".purse-first")
			if err := os.MkdirAll(purseDir, 0o755); err != nil {
				return fmt.Errorf("creating .purse-first/: %w", err)
			}

			devData, err := json.MarshalIndent(devMappings, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling dev mappings: %w", err)
			}
			devData = append(devData, '\n')

			if err := os.WriteFile(filepath.Join(purseDir, name+".json"), devData, 0o644); err != nil {
				return fmt.Errorf("writing dev mappings: %w", err)
			}
		}
	}
	// If no mappings.json exists, skip — not all plugins have mappings

	return nil
}

func cleanDevMCP(projectDir string) error {
	os.Remove(filepath.Join(projectDir, ".mcp.json"))
	os.RemoveAll(filepath.Join(projectDir, ".purse-first"))
	return nil
}
```

**Step 4: Run tests to verify**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && go test ./libs/go-mcp/command/... -run TestDevMCP -v`
Expected: PASS

**Step 5: Commit**

```
git add libs/go-mcp/command/dev_mcp.go libs/go-mcp/command/dev_mcp_test.go
git commit -m "Add dev-mcp generation and cleanup functions"
```

---

### Task 5: Register `dev-mcp` as a hidden command on `App`

**Files:**
- Modify: `libs/go-mcp/command/app.go`
- Modify: `libs/go-mcp/command/dev_mcp.go`

**Step 1: Write the failing test**

Add to `libs/go-mcp/command/dev_mcp_test.go`:

```go
func TestAppHasDevMCPCommand(t *testing.T) {
	app := NewApp("lux", "LSP multiplexer")

	cmd, ok := app.GetCommand("dev-mcp")
	if !ok {
		t.Fatal("dev-mcp command not registered")
	}
	if !cmd.Hidden {
		t.Error("dev-mcp should be hidden")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && go test ./libs/go-mcp/command/... -run TestAppHasDevMCPCommand -v`
Expected: FAIL — `dev-mcp` not registered.

**Step 3: Register the command in `NewApp`**

In `libs/go-mcp/command/app.go`, update `NewApp`:

```go
func NewApp(name, short string) *App {
	a := &App{
		Name:        name,
		Description: Description{Short: short},
		commands:    make(map[string]*Command),
	}

	a.addDevMCPCommand()

	return a
}
```

In `libs/go-mcp/command/dev_mcp.go`, add the registration function:

```go
func (a *App) addDevMCPCommand() {
	a.AddCommand(&Command{
		Name:   "dev-mcp",
		Hidden: true,
		Description: Description{
			Short: "Generate project-local MCP config for dev testing",
		},
		Params: []Param{
			{Name: "suffix", Type: String, Description: "Suffix for the MCP server name", Default: "dev"},
			{Name: "clean", Type: Bool, Description: "Remove generated dev artifacts", Default: false},
		},
	})
}
```

**Step 4: Run test to verify**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && go test ./libs/go-mcp/command/... -run TestAppHasDevMCPCommand -v`
Expected: PASS

**Step 5: Run all tests**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && go test ./libs/go-mcp/command/... -v`
Expected: PASS

**Step 6: Commit**

```
git add libs/go-mcp/command/app.go libs/go-mcp/command/dev_mcp.go libs/go-mcp/command/dev_mcp_test.go
git commit -m "Register dev-mcp as hidden command on App"
```

---

### Task 6: Update grit to use `MapsTools`

**Files:**
- Modify: `(grit repo) internal/tools/status.go`
- Modify: `(grit repo) internal/tools/staging.go`
- Modify: `(grit repo) internal/tools/commit.go`
- Modify: `(grit repo) internal/tools/branch.go`
- Modify: `(grit repo) internal/tools/log.go`
- Modify: `(grit repo) internal/tools/rebase.go`
- Modify: `(grit repo) internal/tools/remote.go`
- Modify: `(grit repo) internal/tools/rev_parse.go`

This task is in the grit repo (`/Users/sfriedenberg/eng/repos/grit`), not purse-first.

**Step 1: Bulk find-and-replace across all files**

In each file, replace:

```go
MapsBash: []command.BashMapping{
	{Prefixes: []string{"..."}, UseWhen: "..."},
},
```

with:

```go
MapsTools: []command.ToolMapping{
	{Replaces: "Bash", CommandPrefixes: []string{"..."}, UseWhen: "..."},
},
```

The transformation is mechanical: add `Replaces: "Bash"` and rename `Prefixes` to `CommandPrefixes`.

**Step 2: Update grit's go.mod to point to local purse-first**

Run: `cd /Users/sfriedenberg/eng/repos/grit && go mod edit -replace code.linenisgreat.com/purse-first=/Users/sfriedenberg/eng/repos/purse-first`

**Step 3: Build and test**

Run: `cd /Users/sfriedenberg/eng/repos/grit && go build ./... && go test ./...`
Expected: PASS

**Step 4: Commit**

```
git commit -am "Migrate MapsBash to MapsTools"
```

Note: The `go.mod` replace directive is temporary for local development. It should be reverted before merging, once purse-first is published with the new API.

---

### Task 7: Update BATS tests for `tool_prefix`

**Files:**
- Modify: `zz-tests_bats/hook_io.bats`
- Modify: `zz-tests_bats/common.bash`

**Step 1: Update the lux mapping fixture in `hook_io.bats`**

The test setup creates a `.purse-first/lux.json` with mappings. Update the fixture to include a `tool_prefix` variant test:

Add a new test to `zz-tests_bats/hook_io.bats`:

```bash
function deny_read_go_with_tool_prefix_uses_prefix { # @test
  # Create a mapping with tool_prefix set (dev mode)
  cat >"$project_dir/.purse-first/lux.json" <<'MAPPING'
{
  "server": "lux",
  "tool_prefix": "mcp__lux-dev",
  "mappings": [
    {
      "replaces": "Read",
      "extensions": [".go"],
      "tools": [
        {"name": "hover", "use_when": "getting type info"}
      ],
      "reason": "Use lux-dev MCP tools"
    }
  ]
}
MAPPING

  local payload
  payload=$(hook_payload Read file_path=/path/to/foo.go)

  run sh -c 'cd "$3" && echo "$1" | "$2" hook' -- "$payload" "$purse_first" "$project_dir"
  assert_success
  assert_output --partial "mcp__lux-dev__hover"
}
```

**Step 2: Build and run**

Run:
```
cd /Users/sfriedenberg/eng/repos/purse-first
nix build
bats zz-tests_bats/hook_io.bats
```
Expected: PASS

**Step 3: Commit**

```
git add zz-tests_bats/hook_io.bats
git commit -m "Add BATS test for tool_prefix in hook deny output"
```

---

### Task 8: Update skills and docs references

**Files:**
- Modify: `skills/go-cli-framework/SKILL.md`
- Modify: `skills/go-cli-framework/examples/command-app.go`
- Modify: `skills/go-cli-framework/references/api-reference.md`

**Step 1: Update all `MapsBash`/`BashMapping` references to `MapsTools`/`ToolMapping`**

In each file, replace `MapsBash` with `MapsTools`, `BashMapping` with `ToolMapping`, add `Replaces: "Bash"` to existing examples, and add a `Read`/`Grep` example showing extensions.

**Step 2: Verify no stale references remain**

Run: `grep -r "MapsBash\|BashMapping" /Users/sfriedenberg/eng/repos/purse-first/skills/`
Expected: no output

**Step 3: Commit**

```
git add skills/
git commit -m "Update skills docs for MapsTools migration"
```
