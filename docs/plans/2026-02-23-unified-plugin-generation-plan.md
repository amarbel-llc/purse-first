# Unified Plugin.json Generation — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate all manual `.claude-plugin/plugin.json` files by making binaries and `purse-first` CLI the single sources of truth for plugin manifest generation.

**Architecture:** Go MCP packages generate their own plugin.json via `command.App` (extended with description, author, skills). Non-Go packages use a `package.toml` source file processed by `purse-first generate-plugin`. Skills are always auto-discovered, never manually listed.

**Tech Stack:** Go (go-mcp command library), TOML (BurntSushi/toml), Nix (build expressions)

---

### Task 1: Extend `pluginManifest` and `App` with description, author, skills

**Files:**
- Modify: `libs/go-mcp/command/app.go:6-16`
- Modify: `libs/go-mcp/command/generate_plugin.go:9-50`
- Test: `libs/go-mcp/command/generate_plugin_test.go`

**Step 1: Write failing tests for description and author fields**

Add to `libs/go-mcp/command/generate_plugin_test.go`:

```go
func TestGeneratePluginWithDescriptionAndAuthor(t *testing.T) {
	app := NewApp("chix", "Nix MCP server")
	app.PluginDescription = "Nix MCP server and skills for Claude Code"
	app.PluginAuthor = "friedenberg"

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "chix", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	if plugin["description"] != "Nix MCP server and skills for Claude Code" {
		t.Errorf("description = %v, want 'Nix MCP server and skills for Claude Code'", plugin["description"])
	}

	author := plugin["author"].(map[string]any)
	if author["name"] != "friedenberg" {
		t.Errorf("author.name = %v, want friedenberg", author["name"])
	}
}

func TestGeneratePluginOmitsEmptyDescriptionAndAuthor(t *testing.T) {
	app := NewApp("grit", "Git operations")

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "grit", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	if _, ok := plugin["description"]; ok {
		t.Errorf("description should be omitted when empty")
	}
	if _, ok := plugin["author"]; ok {
		t.Errorf("author should be omitted when empty")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go test -v -run 'TestGeneratePluginWith(Description|Author)|TestGeneratePluginOmitsEmpty' ./libs/go-mcp/command/`
Expected: FAIL — `PluginDescription` and `PluginAuthor` fields don't exist

**Step 3: Add fields to App and extend pluginManifest and GeneratePlugin**

In `libs/go-mcp/command/app.go`, add fields to `App` struct:

```go
type App struct {
	Name        string
	Description Description
	Version     string
	MCPArgs     []string // extra args passed to the binary in plugin manifests
	MCPBinary   string   // binary name for plugin.json command; defaults to Name
	Params      []Param  // global flags
	Examples    []Example // app-level workflow examples
	PluginDescription string   // "description" in plugin.json; omitted if empty
	PluginAuthor      string   // "author.name" in plugin.json; omitted if empty
	commands       map[string]*Command
	canonicalNames map[*Command]string
}
```

In `libs/go-mcp/command/generate_plugin.go`, extend the manifest struct and generation:

```go
type pluginAuthor struct {
	Name string `json:"name"`
}

type pluginMcpServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type pluginManifest struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Author      *pluginAuthor              `json:"author,omitempty"`
	McpServers  map[string]pluginMcpServer `json:"mcpServers,omitempty"`
	Skills      []string                   `json:"skills,omitempty"`
}

func (a *App) GeneratePlugin(dir string) error {
	manifest := pluginManifest{
		Name:        a.Name,
		Description: a.PluginDescription,
	}

	if a.PluginAuthor != "" {
		manifest.Author = &pluginAuthor{Name: a.PluginAuthor}
	}

	// Only include mcpServers if the app declares itself as an MCP server.
	// Pure skill packages set no MCPArgs and no MCPBinary.
	cmdName := a.Name
	if a.MCPBinary != "" {
		cmdName = a.MCPBinary
	}

	if len(a.MCPArgs) > 0 || a.MCPBinary != "" || a.hasMCPCommands() {
		manifest.McpServers = map[string]pluginMcpServer{
			a.Name: {
				Type:    "stdio",
				Command: cmdName,
				Args:    a.MCPArgs,
			},
		}
	}

	pluginDir := filepath.Join(dir, a.Name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)
}
```

Note: `hasMCPCommands()` is a heuristic — if the app was created with `NewApp` it always has the dev-mcp command registered. A simpler approach: always include `mcpServers` when a non-empty `Name` exists (current behavior). Consider whether this heuristic is needed at all — grit/get-hubbed/lux always want mcpServers. The key point is that description and author get added.

**Step 4: Run tests to verify they pass**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go test -v -run 'TestGeneratePlugin' ./libs/go-mcp/command/`
Expected: ALL TestGeneratePlugin* tests PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/app.go libs/go-mcp/command/generate_plugin.go libs/go-mcp/command/generate_plugin_test.go
git commit -m "feat(go-mcp): add PluginDescription and PluginAuthor to App and GeneratePlugin"
```

---

### Task 2: Add skills discovery and copying to GenerateAll

**Files:**
- Modify: `libs/go-mcp/command/generate.go`
- Modify: `libs/go-mcp/command/generate_plugin.go`
- Test: `libs/go-mcp/command/generate_test.go`

**Step 1: Write failing test for skills discovery**

Add to `libs/go-mcp/command/generate_test.go`:

```go
func TestGenerateAllWithSkills(t *testing.T) {
	app := NewApp("robin", "BATS testing")
	app.PluginDescription = "Expert skill for BATS tests"
	app.PluginAuthor = "friedenberg"
	app.Version = "0.1.0"

	dir := t.TempDir()

	// Create a fake skills directory with SKILL.md files
	skillsDir := t.TempDir()
	os.MkdirAll(filepath.Join(skillsDir, "bats-testing"), 0o755)
	os.WriteFile(filepath.Join(skillsDir, "bats-testing", "SKILL.md"), []byte("# BATS Testing"), 0o644)
	os.MkdirAll(filepath.Join(skillsDir, "another-skill"), 0o755)
	os.WriteFile(filepath.Join(skillsDir, "another-skill", "SKILL.md"), []byte("# Another"), 0o644)

	if err := app.GenerateAllWithSkills(dir, skillsDir); err != nil {
		t.Fatalf("GenerateAllWithSkills: %v", err)
	}

	// Check plugin.json includes skills
	data, err := os.ReadFile(filepath.Join(dir, "share", "purse-first", "robin", "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}

	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	skills, ok := plugin["skills"].([]any)
	if !ok {
		t.Fatalf("skills not found or not array")
	}
	if len(skills) != 2 {
		t.Errorf("got %d skills, want 2", len(skills))
	}

	// Check skills were copied to output
	skillFile := filepath.Join(dir, "share", "purse-first", "robin", "skills", "bats-testing", "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Errorf("skill file not copied: %s", skillFile)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go test -v -run TestGenerateAllWithSkills ./libs/go-mcp/command/`
Expected: FAIL — `GenerateAllWithSkills` method doesn't exist

**Step 3: Implement GenerateAllWithSkills and skill discovery**

In `libs/go-mcp/command/generate_plugin.go`, add skill discovery:

```go
// discoverSkills finds skills by globbing for SKILL.md files in subdirectories.
func discoverSkills(skillsDir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(skillsDir, "*", "SKILL.md"))
	if err != nil {
		return nil, err
	}

	var skills []string
	for _, path := range matches {
		name := filepath.Base(filepath.Dir(path))
		skills = append(skills, "./skills/"+name)
	}

	sort.Strings(skills)
	return skills, nil
}
```

In `libs/go-mcp/command/generate.go`, add `GenerateAllWithSkills`:

```go
// GenerateAllWithSkills is like GenerateAll but also discovers skills from
// skillsDir, copies them into the output, and includes them in plugin.json.
func (a *App) GenerateAllWithSkills(dir, skillsDir string) error {
	purseDir := filepath.Join(dir, "share", "purse-first")

	if skillsDir != "" {
		skills, err := discoverSkills(skillsDir)
		if err != nil {
			return fmt.Errorf("discovering skills: %w", err)
		}
		a.pluginSkills = skills

		// Copy skills into output
		outputSkillsDir := filepath.Join(purseDir, a.Name, "skills")
		if err := copyDir(skillsDir, outputSkillsDir); err != nil {
			return fmt.Errorf("copying skills: %w", err)
		}
	}

	if err := a.GeneratePlugin(purseDir); err != nil {
		return err
	}

	if err := a.GenerateMappings(purseDir); err != nil {
		return err
	}

	if err := a.GenerateManpages(dir); err != nil {
		return err
	}

	return a.GenerateCompletions(dir)
}
```

Add `pluginSkills []string` field to `App` (unexported, set by `GenerateAllWithSkills`).

Update `GeneratePlugin` to include `a.pluginSkills` in the manifest's `Skills` field.

Add `copyDir` helper function that recursively copies a directory.

Add `"sort"` import to `generate_plugin.go`.

Update the existing `GenerateAll` to call `GenerateAllWithSkills(dir, "")` to avoid duplication.

**Step 4: Run tests to verify they pass**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go test -v ./libs/go-mcp/command/`
Expected: ALL tests PASS (including existing `TestGenerateAll`)

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate.go libs/go-mcp/command/generate_plugin.go libs/go-mcp/command/generate_test.go libs/go-mcp/command/app.go
git commit -m "feat(go-mcp): add GenerateAllWithSkills for skill discovery and copying"
```

---

### Task 3: Add `--skills-dir` flag to lux's `_generate` command

**Files:**
- Modify: `packages/lux/cmd/lux/app.go:57-79`

**Step 1: Update `_generate` command to accept `--skills-dir`**

In `packages/lux/cmd/lux/app.go`, update the `_generate` command:

```go
app.AddCommand(&command.Command{
	Name:   "_generate",
	Hidden: true,
	Description: command.Description{
		Short: "Generate plugin artifacts",
	},
	Params: []command.Param{
		{Name: "dir", Type: command.String, Description: "Output directory", Required: true},
		{Name: "skills-dir", Type: command.String, Description: "Skills source directory"},
	},
	RunCLI: func(ctx context.Context, args json.RawMessage) error {
		var p struct {
			Dir       string `json:"dir"`
			SkillsDir string `json:"skills-dir"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return fmt.Errorf("invalid arguments: %w", err)
		}

		tools.RegisterAll(app, nil)

		return app.GenerateAllWithSkills(p.Dir, p.SkillsDir)
	},
})
```

**Step 2: Build lux to verify it compiles**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go build ./packages/lux/cmd/lux/`
Expected: Builds without errors

**Step 3: Commit**

```bash
git add packages/lux/cmd/lux/app.go
git commit -m "feat(lux): add --skills-dir flag to _generate command"
```

---

### Task 4: Update grit and get-hubbed to use framework-based generation

Both grit and get-hubbed use ad-hoc `generate-plugin` arg parsing. Update them to:
1. Set `PluginDescription` and `PluginAuthor` on their `App` (optional, they have none today)
2. Keep the `generate-plugin` entry point calling `GenerateAllWithSkills`

**Files:**
- Modify: `packages/grit/cmd/grit/main.go:38-43`
- Modify: `packages/get-hubbed/cmd/get-hubbed/main.go` (similar section)

**Step 1: Update grit to call GenerateAllWithSkills**

In `packages/grit/cmd/grit/main.go`, change:

```go
if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
	if err := app.GenerateAll(flag.Arg(1)); err != nil {
		log.Fatalf("generating plugin: %v", err)
	}
	return
}
```

to:

```go
if flag.NArg() >= 2 && flag.Arg(0) == "generate-plugin" {
	skillsDir := ""
	if flag.NArg() >= 3 {
		skillsDir = flag.Arg(2)
	}
	if err := app.GenerateAllWithSkills(flag.Arg(1), skillsDir); err != nil {
		log.Fatalf("generating plugin: %v", err)
	}
	return
}
```

**Step 2: Update get-hubbed similarly**

In `packages/get-hubbed/cmd/get-hubbed/main.go`, change:

```go
if len(os.Args) >= 3 && os.Args[1] == "generate-plugin" {
	if err := app.GenerateAll(os.Args[2]); err != nil {
		log.Fatalf("generating plugin: %v", err)
	}
	return
}
```

to:

```go
if len(os.Args) >= 3 && os.Args[1] == "generate-plugin" {
	skillsDir := ""
	if len(os.Args) >= 4 {
		skillsDir = os.Args[3]
	}
	if err := app.GenerateAllWithSkills(os.Args[2], skillsDir); err != nil {
		log.Fatalf("generating plugin: %v", err)
	}
	return
}
```

**Step 3: Build both to verify they compile**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go build ./packages/grit/cmd/grit/ && nix develop --command go build ./packages/get-hubbed/cmd/get-hubbed/`
Expected: Both build without errors

**Step 4: Commit**

```bash
git add packages/grit/cmd/grit/main.go packages/get-hubbed/cmd/get-hubbed/main.go
git commit -m "feat(grit,get-hubbed): update generate-plugin to use GenerateAllWithSkills"
```

---

### Task 5: Create `package.toml` source files for non-Go packages

**Files:**
- Create: `packages/chix/package.toml`
- Create: `packages/batman/package.toml`
- Create: `packages/tap-dancer/package.toml`
- Create: `packages/sandcastle/package.toml`

**Step 1: Create package.toml for chix**

```toml
name = "chix"
description = "Nix MCP server and skills for Claude Code"

[author]
name = "friedenberg"

[mcp.chix]
command = "chix"

[[hooks.PostToolUse]]
matcher = "Edit|Write"
command = "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix"
timeout = 30
```

**Step 2: Create package.toml for batman (robin)**

```toml
name = "robin"
description = "Expert skill for setting up and writing BATS integration tests with bats-support libraries, justfile integration, and sandcastle environment isolation"

[author]
name = "friedenberg"
```

**Step 3: Create package.toml for tap-dancer**

```toml
name = "tap-dancer"
description = "TAP-14 writer libraries (Go + Rust) and skill for producing spec-compliant TAP output"

[author]
name = "friedenberg"
```

**Step 4: Create package.toml for sandcastle**

```toml
name = "sandcastle"
version = "0.1.0"
description = "Sandcastle CLI usage, Nix integration, and test isolation patterns"

[author]
name = "Sasha F"
repository = "https://github.com/amarbel-llc/sandcastle"
```

**Step 5: Commit**

```bash
git add packages/chix/package.toml packages/batman/package.toml packages/tap-dancer/package.toml packages/sandcastle/package.toml
git commit -m "feat: add package.toml source files for non-Go packages"
```

---

### Task 6: Implement `purse-first generate-plugin` command

This replaces the deprecated `generate-local-plugin` with a `package.toml`-based generator.

**Files:**
- Create: `internal/packagetoml/packagetoml.go`
- Create: `internal/packagetoml/packagetoml_test.go`
- Create: `internal/packagetoml/generate.go`
- Create: `internal/packagetoml/generate_test.go`
- Modify: `cmd/purse-first/main.go`

**Step 1: Write failing test for TOML parsing**

Create `internal/packagetoml/packagetoml_test.go`:

```go
package packagetoml

import (
	"testing"
)

func TestParsePackageToml(t *testing.T) {
	input := `
name = "chix"
description = "Nix MCP server and skills for Claude Code"

[author]
name = "friedenberg"

[mcp.chix]
command = "chix"

[[hooks.PostToolUse]]
matcher = "Edit|Write"
command = "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix"
timeout = 30
`

	pkg, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if pkg.Name != "chix" {
		t.Errorf("name = %q, want chix", pkg.Name)
	}
	if pkg.Description != "Nix MCP server and skills for Claude Code" {
		t.Errorf("description = %q", pkg.Description)
	}
	if pkg.Author.Name != "friedenberg" {
		t.Errorf("author.name = %q", pkg.Author.Name)
	}
	if len(pkg.MCP) != 1 {
		t.Fatalf("len(mcp) = %d, want 1", len(pkg.MCP))
	}
	if pkg.MCP["chix"].Command != "chix" {
		t.Errorf("mcp.chix.command = %q", pkg.MCP["chix"].Command)
	}
	if len(pkg.Hooks) != 1 {
		t.Fatalf("len(hooks) = %d, want 1", len(pkg.Hooks))
	}
	hooks := pkg.Hooks["PostToolUse"]
	if len(hooks) != 1 {
		t.Fatalf("len(hooks.PostToolUse) = %d, want 1", len(hooks))
	}
	if hooks[0].Matcher != "Edit|Write" {
		t.Errorf("hook matcher = %q", hooks[0].Matcher)
	}
}

func TestParseSkillOnlyPackage(t *testing.T) {
	input := `
name = "robin"
description = "Expert skill for BATS tests"

[author]
name = "friedenberg"
`

	pkg, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if pkg.Name != "robin" {
		t.Errorf("name = %q, want robin", pkg.Name)
	}
	if len(pkg.MCP) != 0 {
		t.Errorf("mcp should be empty for skill-only package")
	}
	if len(pkg.Hooks) != 0 {
		t.Errorf("hooks should be empty for skill-only package")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go test -v ./internal/packagetoml/`
Expected: FAIL — package doesn't exist

**Step 3: Implement packagetoml types and Parse**

Create `internal/packagetoml/packagetoml.go`:

```go
package packagetoml

import "github.com/BurntSushi/toml"

type Author struct {
	Name string `toml:"name"`
}

type MCPServer struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
}

type Hook struct {
	Matcher string `toml:"matcher"`
	Command string `toml:"command"`
	Timeout int    `toml:"timeout"`
}

type Package struct {
	Name        string                `toml:"name"`
	Description string                `toml:"description"`
	Version     string                `toml:"version"`
	Repository  string                `toml:"repository"`
	Author      Author                `toml:"author"`
	MCP         map[string]MCPServer  `toml:"mcp"`
	Hooks       map[string][]Hook     `toml:"hooks"`
}

func Parse(data []byte) (*Package, error) {
	var pkg Package
	if err := toml.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

func ParseFile(path string) (*Package, error) {
	var pkg Package
	if _, err := toml.DecodeFile(path, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go test -v ./internal/packagetoml/`
Expected: PASS

**Step 5: Write failing test for plugin.json generation from package.toml**

Create `internal/packagetoml/generate_test.go`:

```go
package packagetoml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratePluginJSON(t *testing.T) {
	pkg := &Package{
		Name:        "chix",
		Description: "Nix MCP server and skills for Claude Code",
		Author:      Author{Name: "friedenberg"},
		MCP: map[string]MCPServer{
			"chix": {Command: "chix"},
		},
		Hooks: map[string][]Hook{
			"PostToolUse": {
				{Matcher: "Edit|Write", Command: "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix", Timeout: 30},
			},
		},
	}

	dir := t.TempDir()
	skillsDir := t.TempDir()

	// Create fake skill
	os.MkdirAll(filepath.Join(skillsDir, "nix-codebase"), 0o755)
	os.WriteFile(filepath.Join(skillsDir, "nix-codebase", "SKILL.md"), []byte("# Nix"), 0o644)

	if err := GeneratePluginJSON(pkg, dir, skillsDir); err != nil {
		t.Fatalf("GeneratePluginJSON: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "share", "purse-first", "chix", "plugin.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var manifest map[string]any
	json.Unmarshal(data, &manifest)

	if manifest["name"] != "chix" {
		t.Errorf("name = %v", manifest["name"])
	}
	if manifest["description"] != "Nix MCP server and skills for Claude Code" {
		t.Errorf("description = %v", manifest["description"])
	}

	author := manifest["author"].(map[string]any)
	if author["name"] != "friedenberg" {
		t.Errorf("author.name = %v", author["name"])
	}

	servers := manifest["mcpServers"].(map[string]any)
	srv := servers["chix"].(map[string]any)
	if srv["command"] != "chix" {
		t.Errorf("command = %v", srv["command"])
	}

	skills := manifest["skills"].([]any)
	if len(skills) != 1 || skills[0] != "./skills/nix-codebase" {
		t.Errorf("skills = %v", skills)
	}

	hooks := manifest["hooks"].(map[string]any)
	postToolUse := hooks["PostToolUse"].([]any)
	if len(postToolUse) != 1 {
		t.Errorf("PostToolUse hooks = %v", postToolUse)
	}

	// Check skill was copied
	skillFile := filepath.Join(dir, "share", "purse-first", "chix", "skills", "nix-codebase", "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Errorf("skill not copied: %v", err)
	}
}

func TestGenerateSkillOnlyPluginJSON(t *testing.T) {
	pkg := &Package{
		Name:        "robin",
		Description: "Expert BATS skill",
		Author:      Author{Name: "friedenberg"},
	}

	dir := t.TempDir()
	skillsDir := t.TempDir()
	os.MkdirAll(filepath.Join(skillsDir, "bats-testing"), 0o755)
	os.WriteFile(filepath.Join(skillsDir, "bats-testing", "SKILL.md"), []byte("# BATS"), 0o644)

	if err := GeneratePluginJSON(pkg, dir, skillsDir); err != nil {
		t.Fatalf("GeneratePluginJSON: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "share", "purse-first", "robin", "plugin.json"))
	var manifest map[string]any
	json.Unmarshal(data, &manifest)

	if _, ok := manifest["mcpServers"]; ok {
		t.Errorf("mcpServers should be omitted for skill-only package")
	}
	if _, ok := manifest["hooks"]; ok {
		t.Errorf("hooks should be omitted for skill-only package")
	}

	skills := manifest["skills"].([]any)
	if len(skills) != 1 {
		t.Errorf("skills = %v", skills)
	}
}
```

**Step 6: Run test to verify it fails**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go test -v -run TestGenerate ./internal/packagetoml/`
Expected: FAIL — `GeneratePluginJSON` doesn't exist

**Step 7: Implement GeneratePluginJSON**

Create `internal/packagetoml/generate.go`:

```go
package packagetoml

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type pluginAuthor struct {
	Name string `json:"name"`
}

type pluginMCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type pluginHookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

type pluginHookMatcher struct {
	Matcher string            `json:"matcher"`
	Hooks   []pluginHookEntry `json:"hooks"`
}

type pluginManifest struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description,omitempty"`
	Author      *pluginAuthor                 `json:"author,omitempty"`
	McpServers  map[string]pluginMCPServer    `json:"mcpServers,omitempty"`
	Skills      []string                      `json:"skills,omitempty"`
	Hooks       map[string][]pluginHookMatcher `json:"hooks,omitempty"`
}

func GeneratePluginJSON(pkg *Package, outputDir, skillsDir string) error {
	manifest := pluginManifest{
		Name:        pkg.Name,
		Description: pkg.Description,
	}

	if pkg.Author.Name != "" {
		manifest.Author = &pluginAuthor{Name: pkg.Author.Name}
	}

	if len(pkg.MCP) > 0 {
		manifest.McpServers = make(map[string]pluginMCPServer)
		for name, srv := range pkg.MCP {
			manifest.McpServers[name] = pluginMCPServer{
				Type:    "stdio",
				Command: srv.Command,
				Args:    srv.Args,
			}
		}
	}

	if len(pkg.Hooks) > 0 {
		manifest.Hooks = make(map[string][]pluginHookMatcher)
		for event, hooks := range pkg.Hooks {
			var matchers []pluginHookMatcher
			for _, h := range hooks {
				matchers = append(matchers, pluginHookMatcher{
					Matcher: h.Matcher,
					Hooks: []pluginHookEntry{
						{Type: "command", Command: h.Command, Timeout: h.Timeout},
					},
				})
			}
			manifest.Hooks[event] = matchers
		}
	}

	// Discover and copy skills
	purseDir := filepath.Join(outputDir, "share", "purse-first")

	if skillsDir != "" {
		skills, err := discoverSkills(skillsDir)
		if err != nil {
			return fmt.Errorf("discovering skills: %w", err)
		}
		manifest.Skills = skills

		if len(skills) > 0 {
			destSkillsDir := filepath.Join(purseDir, pkg.Name, "skills")
			if err := copyDir(skillsDir, destSkillsDir); err != nil {
				return fmt.Errorf("copying skills: %w", err)
			}
		}
	}

	// Write plugin.json
	pluginDir := filepath.Join(purseDir, pkg.Name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)
}

func discoverSkills(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*", "SKILL.md"))
	if err != nil {
		return nil, err
	}

	var skills []string
	for _, path := range matches {
		name := filepath.Base(filepath.Dir(path))
		skills = append(skills, "./skills/"+name)
	}

	sort.Strings(skills)
	return skills, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, 0o644)
	})
}
```

**Step 8: Run tests to verify they pass**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go test -v ./internal/packagetoml/`
Expected: ALL PASS

**Step 9: Commit**

```bash
git add internal/packagetoml/
git commit -m "feat: add packagetoml package for parsing package.toml and generating plugin.json"
```

---

### Task 7: Wire up `purse-first generate-plugin` CLI command

**Files:**
- Modify: `cmd/purse-first/main.go`

**Step 1: Add `generate-plugin` command**

Add a new cobra command in `cmd/purse-first/main.go` (near the existing `generate-local-plugin` command):

```go
var (
	genPluginRoot      string
	genPluginOutput    string
	genPluginSkillsDir string
)

genPluginCmd := &cobra.Command{
	Use:   "generate-plugin",
	Short: "Generate plugin.json from package.toml",
	Long:  "Read package.toml, auto-discover skills, and produce share/purse-first/{name}/plugin.json.",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := genPluginRoot
		if root == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			root = cwd
		}

		tomlPath := filepath.Join(root, "package.toml")
		pkg, err := packagetoml.ParseFile(tomlPath)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", tomlPath, err)
		}

		output := genPluginOutput
		if output == "" {
			output = root
		}

		if err := packagetoml.GeneratePluginJSON(pkg, output, genPluginSkillsDir); err != nil {
			return fmt.Errorf("generating plugin.json: %w", err)
		}

		fmt.Fprintf(os.Stderr, "generated %s/share/purse-first/%s/plugin.json\n", output, pkg.Name)
		return nil
	},
}

genPluginCmd.Flags().StringVar(&genPluginRoot, "root", "", "package root containing package.toml (defaults to cwd)")
genPluginCmd.Flags().StringVar(&genPluginOutput, "output", "", "output directory (defaults to root)")
genPluginCmd.Flags().StringVar(&genPluginSkillsDir, "skills-dir", "", "directory containing skills to discover and copy")
```

Add `"code.linenisgreat.com/purse-first/internal/packagetoml"` to imports.

Register with `rootCmd.AddCommand(genPluginCmd)`.

**Step 2: Build to verify it compiles**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go build ./cmd/purse-first/`
Expected: Builds without errors

**Step 3: Smoke-test with chix's package.toml**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go run ./cmd/purse-first generate-plugin --root packages/chix --output /tmp/test-chix --skills-dir packages/chix/skills && cat /tmp/test-chix/share/purse-first/chix/plugin.json`
Expected: Valid JSON with name, description, author, mcpServers, skills, hooks

**Step 4: Commit**

```bash
git add cmd/purse-first/main.go
git commit -m "feat: add purse-first generate-plugin command for package.toml-based generation"
```

---

### Task 8: Update nix build expressions for non-Go packages

**Files:**
- Modify: `lib/packages/chix.nix:51-53`
- Modify: `lib/packages/batman.nix` (the robin derivation)
- Modify: `lib/packages/tap-dancer.nix` (the tap-dancer-skill derivation)

**Step 1: Update chix.nix**

Replace the static `cp` with `purse-first generate-plugin`:

```nix
mkdir -p $out/share/purse-first/chix/hooks
${purse-first-cli}/bin/purse-first generate-plugin \
  --root ${src} \
  --output $out \
  --skills-dir ${src}/skills
install -m 755 ${formatNixHook} $out/share/purse-first/chix/hooks/format-nix
```

Add `purse-first-cli` to the function parameters in `chix.nix`: `{ pkgs, src, craneLib, fhPkg, rustMcpSrc, purse-first-cli }:`

**Step 2: Update batman.nix robin derivation**

Replace the staging dance with:

```nix
robin = pkgs.stdenvNoCC.mkDerivation {
  pname = "robin";
  version = "0.1.0";
  inherit src;
  dontBuild = true;
  nativeBuildInputs = [ purse-first-cli ];
  installPhase = ''
    purse-first generate-plugin \
      --root $src \
      --output $out \
      --skills-dir $src/skills
  '';
};
```

**Step 3: Update tap-dancer.nix skill derivation**

Replace the static `cp` with:

```nix
tap-dancer-skill = pkgs.runCommand "tap-dancer-skill"
  { nativeBuildInputs = [ purse-first-cli ]; }
  ''
    ${purse-first-cli}/bin/purse-first generate-plugin \
      --root ${src} \
      --output $out \
      --skills-dir ${src}/skills
  '';
```

**Step 4: Build all affected packages**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix build .#chix && nix build .#batman && nix build .#tap-dancer`
Expected: All three build successfully

**Step 5: Verify generated plugin.json files**

Run: `cat ./result/share/purse-first/chix/plugin.json` (and similar for robin, tap-dancer)
Expected: Each contains correct name, description, author, mcpServers (where applicable), skills, hooks

**Step 6: Commit**

```bash
git add lib/packages/chix.nix lib/packages/batman.nix lib/packages/tap-dancer.nix
git commit -m "feat(nix): use purse-first generate-plugin for non-Go package manifests"
```

---

### Task 9: Delete all manual `.claude-plugin/plugin.json` files

**Files:**
- Delete: `packages/grit/.claude-plugin/plugin.json`
- Delete: `packages/get-hubbed/.claude-plugin/plugin.json`
- Delete: `packages/lux/.claude-plugin/plugin.json`
- Delete: `packages/chix/.claude-plugin/plugin.json`
- Delete: `packages/batman/.claude-plugin/plugin.json`
- Delete: `packages/tap-dancer/.claude-plugin/plugin.json`
- Delete: `packages/sandcastle/.claude-plugin/plugin.json`

**Step 1: Remove all manual plugin.json files**

```bash
git rm packages/grit/.claude-plugin/plugin.json
git rm packages/get-hubbed/.claude-plugin/plugin.json
git rm packages/lux/.claude-plugin/plugin.json
git rm packages/chix/.claude-plugin/plugin.json
git rm packages/batman/.claude-plugin/plugin.json
git rm packages/tap-dancer/.claude-plugin/plugin.json
git rm packages/sandcastle/.claude-plugin/plugin.json
```

Also remove the `.claude-plugin/` directories if they're now empty. Check if any have other files (hooks, etc.) that should remain.

**Step 2: Remove deprecated `generate-local-plugin` command**

In `cmd/purse-first/main.go`, remove the `genLocalPluginCmd` cobra command and its registration.

**Step 3: Update `localplugin.Generate` callers**

Check if `localplugin.Generate` is called from anywhere else. If only from the deleted command, remove the function. `localplugin.InstallLocal` may still be useful for development — update it to use `packagetoml` instead of reading `.claude-plugin/plugin.json`.

**Step 4: Build everything to verify nothing breaks**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix build .#default`
Expected: Full build succeeds

**Step 5: Commit**

```bash
git add -A
git commit -m "chore: remove manual .claude-plugin/plugin.json files and deprecated generate-local-plugin"
```

---

### Task 10: Update existing tests and fix `TestGeneratePluginWithArgs`

**Files:**
- Modify: `libs/go-mcp/command/generate_plugin_test.go`

**Step 1: Fix the existing test that uses old MCPArgs**

`TestGeneratePluginWithArgs` currently asserts `args = [mcp stdio]` which was the broken value. Update it to use a valid example:

```go
func TestGeneratePluginWithArgs(t *testing.T) {
	app := NewApp("lux", "LSP multiplexer")
	app.MCPArgs = []string{"mcp-stdio"}

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lux", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	servers := plugin["mcpServers"].(map[string]any)
	srv := servers["lux"].(map[string]any)
	args := srv["args"].([]any)
	if len(args) != 1 || args[0] != "mcp-stdio" {
		t.Errorf("args = %v, want [mcp-stdio]", args)
	}
}
```

**Step 2: Run all tests**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix develop --command go test ./...`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add libs/go-mcp/command/generate_plugin_test.go
git commit -m "test: fix TestGeneratePluginWithArgs to use correct mcp-stdio args"
```

---

### Task 11: Full nix build verification

**Step 1: Run full nix build**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix build`
Expected: Succeeds

**Step 2: Run nix flake check**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/mcp-inventory && nix flake check`
Expected: Succeeds

**Step 3: Verify each package's generated plugin.json**

Inspect `./result/share/purse-first/*/plugin.json` for each package and verify:
- grit: has name, mcpServers
- get-hubbed: has name, mcpServers
- lux: has name, mcpServers with args `["mcp-stdio"]`
- chix: has name, description, author, mcpServers, skills, hooks
- robin: has name, description, author, skills
- tap-dancer: has name, description, author, skills

**Step 4: Verify no manual plugin.json files remain**

Run: `find packages/ -name plugin.json`
Expected: No results
