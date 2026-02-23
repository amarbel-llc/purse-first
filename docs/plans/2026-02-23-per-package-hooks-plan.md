# Per-Package Hooks Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move tool routing from purse-first's central hook to native Claude Code plugin hooks owned by each package.

**Architecture:** Each package's `generate-plugin` emits `hooks/hooks.json` referencing a generated shell script that delegates to the package binary's `hook` subcommand. The matching logic moves from purse-first's `internal/mapping/` into go-mcp's `command/` package. Purse-first's central hook, post-hook, and session-end commands are removed.

**Tech Stack:** Go (go-mcp library, purse-first CLI), Nix (build expressions), Bash (hook scripts)

---

### Task 1: Add matching logic to go-mcp

Move `FindMatch` and supporting functions from purse-first into go-mcp so each
package binary can self-match tool invocations against its own ToolMapping
declarations.

**Files:**
- Create: `libs/go-mcp/command/match.go`
- Create: `libs/go-mcp/command/match_test.go`

**Step 1: Write failing tests**

```go
// libs/go-mcp/command/match_test.go
package command

import "testing"

func TestMatchByCommandPrefix(t *testing.T) {
	mappings := []ToolMapping{
		{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking repo status"},
	}
	m := FindToolMatch(mappings, "Bash", "", "git status --short")
	if m == nil {
		t.Fatal("expected match")
	}
	if m.UseWhen != "checking repo status" {
		t.Fatalf("got UseWhen=%q", m.UseWhen)
	}
}

func TestMatchByExtension(t *testing.T) {
	mappings := []ToolMapping{
		{Replaces: "Read", Extensions: []string{".go"}, UseWhen: "reading Go files"},
	}
	m := FindToolMatch(mappings, "Read", "/foo/bar.go", "")
	if m == nil {
		t.Fatal("expected match")
	}
}

func TestNoMatchWrongTool(t *testing.T) {
	mappings := []ToolMapping{
		{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking repo status"},
	}
	m := FindToolMatch(mappings, "Read", "", "git status")
	if m != nil {
		t.Fatal("expected no match")
	}
}

func TestNoMatchWrongPrefix(t *testing.T) {
	mappings := []ToolMapping{
		{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking repo status"},
	}
	m := FindToolMatch(mappings, "Bash", "", "docker ps")
	if m != nil {
		t.Fatal("expected no match")
	}
}

func TestMatchCatchAll(t *testing.T) {
	mappings := []ToolMapping{
		{Replaces: "Bash", UseWhen: "use MCP instead"},
	}
	m := FindToolMatch(mappings, "Bash", "", "anything")
	if m == nil {
		t.Fatal("expected catch-all match")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd libs/go-mcp && go test ./command/ -run TestMatch -v`
Expected: FAIL — `FindToolMatch` undefined

**Step 3: Write implementation**

```go
// libs/go-mcp/command/match.go
package command

import (
	"path/filepath"
	"strings"
)

// FindToolMatch checks whether a tool invocation matches any of the given
// ToolMapping declarations. Returns the first match, or nil.
func FindToolMatch(mappings []ToolMapping, toolName, filePath, command string) *ToolMapping {
	for i := range mappings {
		m := &mappings[i]
		if m.Replaces != toolName {
			continue
		}
		if matchesCriteria(m, filePath, command) {
			return m
		}
	}
	return nil
}

func matchesCriteria(m *ToolMapping, filePath, command string) bool {
	hasExtensions := len(m.Extensions) > 0
	hasPrefixes := len(m.CommandPrefixes) > 0

	if !hasExtensions && !hasPrefixes {
		return true
	}

	if hasExtensions && matchesExtension(m.Extensions, filePath) {
		return true
	}

	if hasPrefixes && matchesPrefix(m.CommandPrefixes, command) {
		return true
	}

	return false
}

func matchesExtension(extensions []string, filePath string) bool {
	if filePath == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, e := range extensions {
		if strings.ToLower(e) == ext {
			return true
		}
	}
	return false
}

func matchesPrefix(prefixes []string, command string) bool {
	if command == "" {
		return false
	}
	for _, p := range prefixes {
		if strings.HasPrefix(command, p) {
			return true
		}
	}
	return false
}
```

**Step 4: Run tests to verify they pass**

Run: `cd libs/go-mcp && go test ./command/ -run TestMatch -v`
Expected: PASS

**Step 5: Commit**

```
feat(go-mcp): add FindToolMatch for per-package hook matching
```

---

### Task 2: Add hook handler to go-mcp command.App

Add a `HandleHook` method to `command.App` that reads HookInput from stdin,
matches against the app's ToolMappings, and writes the deny/allow response.

**Files:**
- Create: `libs/go-mcp/command/hook.go`
- Create: `libs/go-mcp/command/hook_test.go`

**Step 1: Write failing tests**

```go
// libs/go-mcp/command/hook_test.go
package command

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleHookDeniesMatch(t *testing.T) {
	app := &App{
		Name: "grit",
		Commands: []Command{
			{
				Name: "status",
				MapsTools: []ToolMapping{
					{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking repo status"},
				},
			},
		},
	}

	input := `{"tool_name":"Bash","tool_input":{"command":"git status --short"}}`
	var out bytes.Buffer
	err := app.HandleHook(strings.NewReader(input), &out)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(out.Bytes(), &result)
	hso, _ := result["hookSpecificOutput"].(map[string]any)
	if hso["permissionDecision"] != "deny" {
		t.Fatalf("expected deny, got %v", hso["permissionDecision"])
	}
	reason, _ := hso["permissionDecisionReason"].(string)
	if !strings.Contains(reason, "mcp__plugin_grit_grit__status") {
		t.Fatalf("expected tool suggestion, got %q", reason)
	}
}

func TestHandleHookAllowsNoMatch(t *testing.T) {
	app := &App{
		Name: "grit",
		Commands: []Command{
			{
				Name: "status",
				MapsTools: []ToolMapping{
					{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking repo status"},
				},
			},
		},
	}

	input := `{"tool_name":"Bash","tool_input":{"command":"docker ps"}}`
	var out bytes.Buffer
	err := app.HandleHook(strings.NewReader(input), &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected empty output for allow, got %q", out.String())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd libs/go-mcp && go test ./command/ -run TestHandleHook -v`
Expected: FAIL — `HandleHook` undefined

**Step 3: Write implementation**

```go
// libs/go-mcp/command/hook.go
package command

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type hookInput struct {
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput hookDecision `json:"hookSpecificOutput"`
}

type hookDecision struct {
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

// HandleHook reads a PreToolUse HookInput from r, matches against the app's
// ToolMappings, and writes a deny response to w if matched. Writes nothing
// (allow) if no match.
func (a *App) HandleHook(r io.Reader, w io.Writer) error {
	var input hookInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return fmt.Errorf("decoding hook input: %w", err)
	}

	filePath := extractStringField(input.ToolInput, "file_path", "path", "pattern")
	command := extractStringField(input.ToolInput, "command")

	allMappings := a.AllToolMappings()
	m := FindToolMatch(allMappings, input.ToolName, filePath, command)
	if m == nil {
		return nil
	}

	prefix := fmt.Sprintf("mcp__plugin_%s_%s", a.Name, a.Name)
	reason := formatDenyReason(prefix, m)

	return json.NewEncoder(w).Encode(hookOutput{
		HookSpecificOutput: hookDecision{
			PermissionDecision:       "deny",
			PermissionDecisionReason: reason,
		},
	})
}

// AllToolMappings collects ToolMappings from all commands, annotated with
// the command name for tool suggestion formatting.
func (a *App) AllToolMappings() []ToolMapping {
	var all []ToolMapping
	for _, cmd := range a.AllCommands() {
		for _, tm := range cmd.MapsTools {
			if tm.toolName == "" {
				tm.toolName = cmd.Name
			}
			all = append(all, tm)
		}
	}
	return all
}

func formatDenyReason(prefix string, m *ToolMapping) string {
	var b strings.Builder
	b.WriteString("Use the MCP tool instead:\n")
	b.WriteString(fmt.Sprintf("- %s__%s: %s", prefix, m.toolName, m.UseWhen))
	return b.String()
}

func extractStringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}
```

Note: Add `toolName string` unexported field to `ToolMapping` struct in
`command.go` so `AllToolMappings` can annotate with the command name.

**Step 4: Run tests to verify they pass**

Run: `cd libs/go-mcp && go test ./command/ -run TestHandleHook -v`
Expected: PASS

**Step 5: Commit**

```
feat(go-mcp): add HandleHook for per-package PreToolUse handling
```

---

### Task 3: Generate hooks/hooks.json in generate-plugin

Extend `App.GeneratePlugin` (or `GenerateAll`) to emit `hooks/hooks.json` and
a `hooks/pre-tool-use` shell wrapper alongside the existing `plugin.json`.

**Files:**
- Modify: `libs/go-mcp/command/generate_plugin.go`
- Create: `libs/go-mcp/command/generate_hooks.go`
- Create: `libs/go-mcp/command/generate_hooks_test.go`

**Step 1: Write failing test**

```go
// libs/go-mcp/command/generate_hooks_test.go
package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateHooksCreatesHooksJSON(t *testing.T) {
	app := &App{
		Name: "grit",
		Commands: []Command{
			{Name: "status", MapsTools: []ToolMapping{
				{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking repo"},
			}},
			{Name: "diff", MapsTools: []ToolMapping{
				{Replaces: "Bash", CommandPrefixes: []string{"git diff"}, UseWhen: "viewing changes"},
			}},
		},
	}

	dir := t.TempDir()
	if err := app.GenerateHooks(dir); err != nil {
		t.Fatal(err)
	}

	hooksPath := filepath.Join(dir, app.Name, "hooks", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not created: %v", err)
	}

	var hooksFile map[string]any
	json.Unmarshal(data, &hooksFile)
	hooks := hooksFile["hooks"].(map[string]any)
	pre := hooks["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("expected 1 PreToolUse entry, got %d", len(pre))
	}

	entry := pre[0].(map[string]any)
	if entry["matcher"] != "Bash" {
		t.Fatalf("expected matcher=Bash, got %v", entry["matcher"])
	}
}

func TestGenerateHooksSkipsWhenNoMappings(t *testing.T) {
	app := &App{
		Name: "simple",
		Commands: []Command{
			{Name: "run"},
		},
	}

	dir := t.TempDir()
	if err := app.GenerateHooks(dir); err != nil {
		t.Fatal(err)
	}

	hooksPath := filepath.Join(dir, app.Name, "hooks", "hooks.json")
	if _, err := os.Stat(hooksPath); !os.IsNotExist(err) {
		t.Fatal("hooks.json should not be created when no mappings exist")
	}
}

func TestGenerateHooksCreatesBinaryWrapper(t *testing.T) {
	app := &App{
		Name: "grit",
		Commands: []Command{
			{Name: "status", MapsTools: []ToolMapping{
				{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking repo"},
			}},
		},
	}

	dir := t.TempDir()
	if err := app.GenerateHooks(dir); err != nil {
		t.Fatal(err)
	}

	wrapperPath := filepath.Join(dir, app.Name, "hooks", "pre-tool-use")
	info, err := os.Stat(wrapperPath)
	if err != nil {
		t.Fatalf("pre-tool-use not created: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("pre-tool-use should be executable")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd libs/go-mcp && go test ./command/ -run TestGenerateHooks -v`
Expected: FAIL — `GenerateHooks` undefined

**Step 3: Write implementation**

```go
// libs/go-mcp/command/generate_hooks.go
package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GenerateHooks emits hooks/hooks.json and hooks/pre-tool-use under
// {dir}/{app.Name}/ for packages that have ToolMapping declarations.
func (a *App) GenerateHooks(dir string) error {
	replaces := a.collectReplaces()
	if len(replaces) == 0 {
		return nil
	}

	matcher := strings.Join(replaces, "|")
	hooksDir := filepath.Join(dir, a.Name, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks dir: %w", err)
	}

	// Write hooks.json
	hooksJSON := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []map[string]any{
				{
					"matcher": matcher,
					"hooks": []map[string]any{
						{
							"type":    "command",
							"command": "'${CLAUDE_PLUGIN_ROOT}/hooks/pre-tool-use'",
							"timeout": 5,
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(hooksJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling hooks.json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), data, 0o644); err != nil {
		return fmt.Errorf("writing hooks.json: %w", err)
	}

	// Write pre-tool-use wrapper script.
	// The actual binary path gets baked in by the nix build's postInstall.
	// This placeholder uses $0's directory to find the binary.
	self, _ := os.Executable()
	script := fmt.Sprintf("#!/bin/sh\nexec '%s' hook\n", self)
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-tool-use"), []byte(script), 0o755); err != nil {
		return fmt.Errorf("writing pre-tool-use: %w", err)
	}

	return nil
}

func (a *App) collectReplaces() []string {
	seen := make(map[string]bool)
	for _, cmd := range a.AllCommands() {
		for _, tm := range cmd.MapsTools {
			seen[tm.Replaces] = true
		}
	}
	var result []string
	for r := range seen {
		result = append(result, r)
	}
	sort.Strings(result)
	return result
}
```

Also update `GenerateAll` in `generate_plugin.go` to call `GenerateHooks`:

```go
func (a *App) GenerateAll(dir string) error {
	outDir := filepath.Join(dir, "share", "purse-first")
	if err := a.GeneratePlugin(outDir); err != nil {
		return err
	}
	if err := a.GenerateMappings(outDir); err != nil {
		return err
	}
	if err := a.GenerateHooks(outDir); err != nil {
		return err
	}
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd libs/go-mcp && go test ./command/ -run TestGenerateHooks -v`
Expected: PASS

**Step 5: Commit**

```
feat(go-mcp): generate hooks/hooks.json from ToolMapping declarations
```

---

### Task 4: Wire `hook` subcommand in each package

Each package binary needs to handle `hook` as a CLI subcommand that calls
`app.HandleHook`. Most packages already have a `generate-plugin` entry point
pattern. Add `hook` alongside it.

**Files:**
- Modify: `packages/grit/cmd/grit/main.go`
- Modify: `packages/get-hubbed/cmd/get-hubbed/main.go`
- Modify: `packages/lux/cmd/lux/main.go`
- Check: `packages/chix/` (Rust — separate handling)

**Step 1: Examine current pattern in grit**

Grit's main.go (lines 39-43) already handles `generate-plugin` via flag args:

```go
if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
    if err := app.GenerateAll(flag.Arg(1)); err != nil {
        log.Fatalf("generating plugin: %v", err)
    }
    return
}
```

**Step 2: Add hook handling**

Add before the MCP server start, after `generate-plugin`:

```go
if flag.NArg() >= 1 && flag.Arg(0) == "hook" {
    if err := app.HandleHook(os.Stdin, os.Stdout); err != nil {
        log.Fatalf("handling hook: %v", err)
    }
    return
}
```

Apply same pattern to get-hubbed and lux.

**Step 3: Test manually**

```bash
echo '{"tool_name":"Bash","tool_input":{"command":"git status"}}' | \
  go run ./packages/grit/cmd/grit hook
```

Expected: JSON with `permissionDecision: "deny"`

```bash
echo '{"tool_name":"Bash","tool_input":{"command":"docker ps"}}' | \
  go run ./packages/grit/cmd/grit hook
```

Expected: Empty output (allow)

**Step 4: Commit**

```
feat: wire hook subcommand in grit, get-hubbed, and lux
```

---

### Task 5: Remove purse-first central hook infrastructure

Remove the central hook/mapping packages and CLI subcommands from purse-first.

**Files:**
- Delete: `internal/hook/handler.go`
- Delete: `internal/hook/handler_test.go`
- Delete: `internal/hook/install.go`
- Delete: `internal/hook/install_hooks_test.go`
- Delete: `internal/hook/notify.go`
- Delete: `internal/hook/post_handler.go`
- Delete: `internal/hook/session_handler.go`
- Delete: `internal/mapping/types.go`
- Delete: `internal/mapping/loader.go`
- Delete: `internal/mapping/loader_test.go`
- Modify: `cmd/purse-first/main.go` — remove `hook`, `post-hook`, `session-end`, `uninstall-hooks` commands
- Modify: `internal/localplugin/generate.go` — remove hook.Install call
- Modify: `internal/install/install.go` — remove hook.Uninstall call

**Step 1: Remove CLI subcommands from main.go**

Remove `hookCmd`, `postHookCmd`, `sessionEndCmd`, `uninstallHooksCmd`
definitions (lines 31-68) and their registration in `root.AddCommand` (line
235).

**Step 2: Remove hook.Install from install-local**

In `internal/localplugin/generate.go`, remove the "Install hooks" block (lines
116-130) and the `hook` import. Adjust `PlanAhead` counts accordingly.

**Step 3: Remove hook.Uninstall from install**

In `internal/install/install.go`, remove the `NoHooks` conditional that calls
`hook.Uninstall` (around line 149-156) and the `hook` import.

**Step 4: Delete internal/hook/ and internal/mapping/ directories**

```bash
rm -rf internal/hook/ internal/mapping/
```

**Step 5: Verify build**

Run: `go build ./cmd/purse-first/`
Expected: Clean build

**Step 6: Commit**

```
refactor: remove central hook infrastructure from purse-first

Per-package hooks replace the central router. Each package now owns
its own PreToolUse hook via Claude Code's native plugin hook system.
```

---

### Task 6: Update nix builds to include hooks directory

The nix package expressions need to ensure `hooks/` is included in the output
alongside `plugin.json`. Since `generate-plugin` already writes to the output
dir, this should work automatically, but verify and fix if needed.

**Files:**
- Check: `lib/packages/grit.nix`
- Check: `lib/packages/get-hubbed.nix`
- Check: `lib/packages/lux.nix`
- Check: `lib/mkMarketplace.nix`

**Step 1: Verify grit nix build includes hooks**

Run: `nix build .#grit && ls result/share/purse-first/grit/hooks/`
Expected: `hooks.json` and `pre-tool-use`

**Step 2: Verify marketplace build symlinks hooks**

Run: `nix build .#default && ls result/share/purse-first/grit/hooks/`
Expected: Same files accessible through marketplace output

**Step 3: Verify marketplace-no-hooks strips them**

The existing `marketplace-no-hooks` variant in `mkMarketplace.nix` already
strips hooks directories (lines 146-170). Verify it still works:

Run: `nix build .#marketplace-no-hooks && ls result/share/purse-first/grit/`
Expected: No `hooks/` directory

**Step 4: Fix any issues found and commit**

```
build: verify hooks directory included in nix package outputs
```

---

### Task 7: Update purse-first install to not register hooks

The `purse-first install` command should no longer register central hooks.
The `--no-hooks` flag and `NoHooks` option become unnecessary since there
are no purse-first hooks to install.

**Files:**
- Modify: `internal/install/install.go` — remove NoHooks logic
- Modify: `cmd/purse-first/main.go` — remove --no-hooks flag if present

**Step 1: Remove NoHooks from install options and implementation**

**Step 2: Verify install works**

Run: `go run ./cmd/purse-first install --help`
Expected: No `--no-hooks` flag

**Step 3: Commit**

```
refactor: remove --no-hooks flag from install command
```

---

### Task 8: Clean up .claude/settings.json and test end-to-end

Remove stale purse-first hooks from the project settings and verify the full
flow works.

**Files:**
- Modify: `.claude/settings.json` — remove purse-first hook entries

**Step 1: Clean settings**

Remove all hook entries from `.claude/settings.json` that reference
`purse-first`.

**Step 2: Build and install**

```bash
nix build
./result/bin/purse-first install
```

**Step 3: Restart Claude Code and test**

Start a new Claude Code session. Verify:
- `git status` bash command gets denied with suggestion to use grit MCP
- `cat foo.go` gets denied with suggestion to use lux MCP (if lux has Read mappings)
- Regular commands not covered by any mapping pass through

**Step 4: Commit**

```
chore: remove stale purse-first hooks from project settings
```

---

### Task 9: Update bob skills and docs

Update the using-packages skill and any docs that reference the central hook
architecture.

**Files:**
- Modify: `skills/using-packages/SKILL.md`
- Modify: `docs/purse-first-protocol.md` (if it references hooks)

**Step 1: Update using-packages skill**

Remove references to:
- `.purse-first/` override directories
- `~/.local/state/purse-first/` global overrides
- `purse-first hook` command
- Central hook matching logic
- `PURSE_FIRST_PLUGINS_DIR` for mapping resolution

Replace with explanation that each package owns its own hooks.

**Step 2: Commit**

```
docs: update skills and docs for per-package hooks
```
