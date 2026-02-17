# Unified Command Framework Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a `command` package in libs/go-mcp that replaces flag/cobra and the `purse` builder, generating CLI parser, MCP tool registration, plugin.json, mappings.json, manpages, and shell completions from a single `Command` declaration.

**Architecture:** New `command` package in `libs/go-mcp/command/` with types (`App`, `Command`, `Param`, `Request`), a CLI parser, an MCP tool bridge, and generators for manpages, completions, plugin manifest, and mappings. Each generator walks the same command tree. Built-in subcommands (help, complete, generate-all) are added automatically.

**Tech Stack:** Go (stdlib + go-lib-mcp protocol/server packages), roff for manpages, bash/zsh/fish completion formats

---

### Task 1: Core types — Command, Param, Description

**Files:**
- Create: `libs/go-mcp/command/command.go`
- Create: `libs/go-mcp/command/command_test.go`

**Step 1: Write the failing test**

```go
// libs/go-mcp/command/command_test.go
package command

import "testing"

func TestParamTypeString(t *testing.T) {
	tests := []struct {
		pt   ParamType
		want string
	}{
		{String, "string"},
		{Int, "integer"},
		{Bool, "boolean"},
		{Float, "number"},
	}
	for _, tt := range tests {
		if got := tt.pt.JSONSchemaType(); got != tt.want {
			t.Errorf("ParamType(%d).JSONSchemaType() = %q, want %q", tt.pt, got, tt.want)
		}
	}
}

func TestCommandParamsRequired(t *testing.T) {
	cmd := Command{
		Name: "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
	}

	required := cmd.RequiredParams()
	if len(required) != 1 {
		t.Fatalf("RequiredParams() len = %d, want 1", len(required))
	}
	if required[0].Name != "repo_path" {
		t.Errorf("RequiredParams()[0].Name = %q, want %q", required[0].Name, "repo_path")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd libs/go-mcp && go test ./command/ -v`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
// libs/go-mcp/command/command.go
package command

// ParamType identifies the data type of a command parameter.
type ParamType int

const (
	String ParamType = iota
	Int
	Bool
	Float
)

// JSONSchemaType returns the JSON Schema type name for this ParamType.
func (pt ParamType) JSONSchemaType() string {
	switch pt {
	case Int:
		return "integer"
	case Bool:
		return "boolean"
	case Float:
		return "number"
	default:
		return "string"
	}
}

// Description holds short and long descriptions for a command.
type Description struct {
	Short string // one-line: manpage NAME, completion tab text, MCP tool description
	Long  string // paragraph: manpage DESCRIPTION, --help output
}

// BashMapping declares a bash command prefix that should be intercepted
// and redirected to this command's MCP tool.
type BashMapping struct {
	Prefixes []string // e.g., "git status"
	UseWhen  string   // shown to Claude in mapping denial
}

// Param declares a single command parameter, used for CLI flags,
// MCP JSON schema properties, manpage OPTIONS, and completions.
type Param struct {
	Name        string
	Type        ParamType
	Description string
	Required    bool
	Default     any
	Completer   func() map[string]string
}

// Command declares a single subcommand with all metadata needed
// to generate CLI parsing, MCP tool registration, manpages,
// completions, and plugin manifests.
type Command struct {
	Name        string
	Aliases     []string
	Description Description
	Hidden      bool

	Params      []Param
	MapsBash    []BashMapping
}

// RequiredParams returns only the params marked as required.
func (c *Command) RequiredParams() []Param {
	var out []Param
	for _, p := range c.Params {
		if p.Required {
			out = append(out, p)
		}
	}
	return out
}

// OptionalParams returns only the params not marked as required.
func (c *Command) OptionalParams() []Param {
	var out []Param
	for _, p := range c.Params {
		if !p.Required {
			out = append(out, p)
		}
	}
	return out
}
```

**Step 4: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./command/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/command.go libs/go-mcp/command/command_test.go
git commit -m "Add command package core types: Command, Param, Description"
```

---

### Task 2: App type — registry and AddCommand

**Files:**
- Create: `libs/go-mcp/command/app.go`
- Create: `libs/go-mcp/command/app_test.go`

**Step 1: Write the failing test**

```go
// libs/go-mcp/command/app_test.go
package command

import "testing"

func TestAppAddCommand(t *testing.T) {
	app := NewApp("grit", "Git operations MCP server")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
	})

	cmd, ok := app.GetCommand("status")
	if !ok {
		t.Fatal("GetCommand(status) not found")
	}
	if cmd.Name != "status" {
		t.Errorf("cmd.Name = %q, want %q", cmd.Name, "status")
	}
}

func TestAppAddCommandWithAliases(t *testing.T) {
	app := NewApp("dodder", "Zettelkasten CLI")

	app.AddCommand(&Command{
		Name:    "checkin",
		Aliases: []string{"add", "save"},
	})

	for _, name := range []string{"checkin", "add", "save"} {
		if _, ok := app.GetCommand(name); !ok {
			t.Errorf("GetCommand(%q) not found", name)
		}
	}
}

func TestAppAddCommandPanicsOnDuplicate(t *testing.T) {
	app := NewApp("test", "test")
	app.AddCommand(&Command{Name: "foo"})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate command")
		}
	}()

	app.AddCommand(&Command{Name: "foo"})
}

func TestAppMergeWithPrefix(t *testing.T) {
	parent := NewApp("dodder", "main")
	child := NewApp("madder", "blob store")

	child.AddCommand(&Command{Name: "cat"})
	child.AddCommand(&Command{Name: "ls"})

	parent.MergeWithPrefix(child, "blob_store")

	if _, ok := parent.GetCommand("blob_store-cat"); !ok {
		t.Error("GetCommand(blob_store-cat) not found")
	}
	if _, ok := parent.GetCommand("blob_store-ls"); !ok {
		t.Error("GetCommand(blob_store-ls) not found")
	}
}

func TestAppAllCommands(t *testing.T) {
	app := NewApp("test", "test")
	app.AddCommand(&Command{Name: "a"})
	app.AddCommand(&Command{Name: "b"})
	app.AddCommand(&Command{Name: "c", Hidden: true})

	count := 0
	for range app.AllCommands() {
		count++
	}
	if count != 3 {
		t.Errorf("AllCommands count = %d, want 3", count)
	}

	visible := 0
	for range app.VisibleCommands() {
		visible++
	}
	if visible != 2 {
		t.Errorf("VisibleCommands count = %d, want 2", visible)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestApp`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// libs/go-mcp/command/app.go
package command

import "fmt"

// App holds the command registry and top-level metadata for a CLI/MCP application.
type App struct {
	Name        string
	Description Description
	Version     string
	Params      []Param // global flags
	commands    map[string]*Command
}

// NewApp creates a new App with the given name and short description.
func NewApp(name, short string) *App {
	return &App{
		Name:        name,
		Description: Description{Short: short},
		commands:    make(map[string]*Command),
	}
}

// AddCommand registers a command and its aliases. Panics on duplicate names.
func (a *App) AddCommand(cmd *Command) {
	a.addName(cmd.Name, cmd)
	for _, alias := range cmd.Aliases {
		a.addName(alias, cmd)
	}
}

func (a *App) addName(name string, cmd *Command) {
	if _, ok := a.commands[name]; ok {
		panic(fmt.Sprintf("command added more than once: %s", name))
	}
	a.commands[name] = cmd
}

// GetCommand looks up a command by name or alias.
func (a *App) GetCommand(name string) (*Command, bool) {
	cmd, ok := a.commands[name]
	return cmd, ok
}

// AllCommands iterates over all registered commands (including hidden).
// Each unique command is yielded once even if it has aliases.
func (a *App) AllCommands() func(yield func(string, *Command) bool) {
	return func(yield func(string, *Command) bool) {
		seen := make(map[*Command]bool)
		for name, cmd := range a.commands {
			if seen[cmd] {
				continue
			}
			seen[cmd] = true
			if !yield(name, cmd) {
				return
			}
		}
	}
}

// VisibleCommands iterates over non-hidden commands.
func (a *App) VisibleCommands() func(yield func(string, *Command) bool) {
	return func(yield func(string, *Command) bool) {
		for name, cmd := range a.AllCommands() {
			if cmd.Hidden {
				continue
			}
			if !yield(name, cmd) {
				return
			}
		}
	}
}

// MergeWithPrefix adds all commands from another App, prefixed with the given string.
func (a *App) MergeWithPrefix(other *App, prefix string) {
	for name, cmd := range other.AllCommands() {
		key := name
		if prefix != "" {
			key = prefix + "-" + name
		}
		a.addName(key, cmd)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestApp`
Expected: PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/app.go libs/go-mcp/command/app_test.go
git commit -m "Add App type with command registry and prefix merging"
```

---

### Task 3: JSON Schema generation from Params

**Files:**
- Create: `libs/go-mcp/command/schema.go`
- Create: `libs/go-mcp/command/schema_test.go`

**Step 1: Write the failing test**

```go
// libs/go-mcp/command/schema_test.go
package command

import (
	"encoding/json"
	"testing"
)

func TestCommandInputSchema(t *testing.T) {
	cmd := Command{
		Name: "status",
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]interface{}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	if parsed["type"] != "object" {
		t.Errorf("type = %v, want object", parsed["type"])
	}

	props := parsed["properties"].(map[string]interface{})
	repoProp := props["repo_path"].(map[string]interface{})
	if repoProp["type"] != "string" {
		t.Errorf("repo_path.type = %v, want string", repoProp["type"])
	}
	if repoProp["description"] != "Path to repo" {
		t.Errorf("repo_path.description = %v", repoProp["description"])
	}

	verboseProp := props["verbose"].(map[string]interface{})
	if verboseProp["type"] != "boolean" {
		t.Errorf("verbose.type = %v, want boolean", verboseProp["type"])
	}

	required := parsed["required"].([]interface{})
	if len(required) != 1 || required[0] != "repo_path" {
		t.Errorf("required = %v, want [repo_path]", required)
	}
}

func TestCommandInputSchemaNoRequired(t *testing.T) {
	cmd := Command{
		Name: "ping",
		Params: []Param{
			{Name: "message", Type: String, Description: "Optional message"},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]interface{}
	json.Unmarshal(schema, &parsed)

	if _, ok := parsed["required"]; ok {
		t.Error("required should be omitted when no params are required")
	}
}

func TestCommandInputSchemaWithDefault(t *testing.T) {
	cmd := Command{
		Name: "serve",
		Params: []Param{
			{Name: "port", Type: Int, Description: "Port number", Default: 8080},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]interface{}
	json.Unmarshal(schema, &parsed)

	props := parsed["properties"].(map[string]interface{})
	portProp := props["port"].(map[string]interface{})
	if portProp["default"] != float64(8080) {
		t.Errorf("port.default = %v, want 8080", portProp["default"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestCommandInput`
Expected: FAIL — InputSchema method does not exist

**Step 3: Write minimal implementation**

```go
// libs/go-mcp/command/schema.go
package command

import "encoding/json"

type schemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
}

type inputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]schemaProperty `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
}

// InputSchema returns a JSON Schema describing this command's parameters,
// suitable for use as an MCP tool's inputSchema.
func (c *Command) InputSchema() json.RawMessage {
	schema := inputSchema{
		Type:       "object",
		Properties: make(map[string]schemaProperty),
	}

	for _, p := range c.Params {
		prop := schemaProperty{
			Type:        p.Type.JSONSchemaType(),
			Description: p.Description,
			Default:     p.Default,
		}
		schema.Properties[p.Name] = prop

		if p.Required {
			schema.Required = append(schema.Required, p.Name)
		}
	}

	if len(schema.Required) == 0 {
		schema.Required = nil
	}

	data, _ := json.Marshal(schema)
	return data
}
```

**Step 4: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestCommandInput`
Expected: PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/schema.go libs/go-mcp/command/schema_test.go
git commit -m "Add JSON Schema generation from Command params"
```

---

### Task 4: MCP tool registration bridge

**Files:**
- Create: `libs/go-mcp/command/mcp.go`
- Create: `libs/go-mcp/command/mcp_test.go`

**Step 1: Write the failing test**

```go
// libs/go-mcp/command/mcp_test.go
package command

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
	"github.com/amarbel-llc/go-lib-mcp/server"
)

func TestAppRegisterMCPTools(t *testing.T) {
	app := NewApp("grit", "Git MCP server")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
		},
		RunMCP: func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
			return &protocol.ToolCallResult{
				Content: []protocol.ContentBlock{protocol.TextContent("ok")},
			}, nil
		},
	})

	app.AddCommand(&Command{
		Name:   "internal",
		Hidden: true,
	})

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)

	tools, err := registry.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("tools len = %d, want 1 (hidden commands excluded)", len(tools))
	}

	if tools[0].Name != "status" {
		t.Errorf("tools[0].Name = %q, want %q", tools[0].Name, "status")
	}

	if tools[0].Description != "Show status" {
		t.Errorf("tools[0].Description = %q, want %q", tools[0].Description, "Show status")
	}

	// Verify the schema has the right structure
	var schema map[string]interface{}
	json.Unmarshal(tools[0].InputSchema, &schema)
	props := schema["properties"].(map[string]interface{})
	if _, ok := props["repo_path"]; !ok {
		t.Error("schema missing repo_path property")
	}
}

func TestAppMCPToolCall(t *testing.T) {
	app := NewApp("test", "test")

	app.AddCommand(&Command{
		Name: "echo",
		Params: []Param{
			{Name: "message", Type: String, Description: "Message to echo"},
		},
		RunMCP: func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
			var params struct {
				Message string `json:"message"`
			}
			json.Unmarshal(args, &params)
			return &protocol.ToolCallResult{
				Content: []protocol.ContentBlock{protocol.TextContent(params.Message)},
			}, nil
		},
	})

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)

	result, err := registry.CallTool(
		context.Background(),
		"echo",
		json.RawMessage(`{"message":"hello"}`),
	)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.Content[0].Text != "hello" {
		t.Errorf("result = %q, want %q", result.Content[0].Text, "hello")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestAppRegisterMCP`
Expected: FAIL — RunMCP field and RegisterMCPTools method do not exist

**Step 3: Write minimal implementation**

Add handler fields to `Command` in `command.go`:

```go
// Add to command.go, inside the Command struct:
import (
	"context"
	"encoding/json"
	"github.com/amarbel-llc/go-lib-mcp/protocol"
)

// In Command struct, add:
	// RunMCP handles MCP tool invocations.
	RunMCP func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error)
```

Create the bridge:

```go
// libs/go-mcp/command/mcp.go
package command

import (
	"github.com/amarbel-llc/go-lib-mcp/server"
)

// RegisterMCPTools registers all non-hidden commands as MCP tools
// in the given ToolRegistry, using each command's description and
// auto-generated JSON schema.
func (a *App) RegisterMCPTools(registry *server.ToolRegistry) {
	for _, cmd := range a.AllCommands() {
		if cmd.Hidden {
			continue
		}
		if cmd.RunMCP == nil {
			continue
		}

		registry.Register(
			cmd.Name,
			cmd.Description.Short,
			cmd.InputSchema(),
			cmd.RunMCP,
		)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestApp`
Expected: PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/command.go libs/go-mcp/command/mcp.go libs/go-mcp/command/mcp_test.go
git commit -m "Add MCP tool registration bridge from App commands"
```

---

### Task 5: Plugin manifest generation (GeneratePlugin)

**Files:**
- Create: `libs/go-mcp/command/generate_plugin.go`
- Create: `libs/go-mcp/command/generate_plugin_test.go`

**Step 1: Write the failing test**

```go
// libs/go-mcp/command/generate_plugin_test.go
package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratePlugin(t *testing.T) {
	app := NewApp("grit", "Git operations")
	app.Version = "0.1.0"

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
	})

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	path := filepath.Join(dir, "grit", "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}

	var plugin map[string]interface{}
	if err := json.Unmarshal(data, &plugin); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if plugin["name"] != "grit" {
		t.Errorf("name = %v, want grit", plugin["name"])
	}

	servers := plugin["mcpServers"].(map[string]interface{})
	srv := servers["grit"].(map[string]interface{})
	if srv["type"] != "stdio" {
		t.Errorf("type = %v, want stdio", srv["type"])
	}
	if srv["command"] != "grit" {
		t.Errorf("command = %v, want grit", srv["command"])
	}
}

func TestGeneratePluginWithArgs(t *testing.T) {
	app := NewApp("lux", "LSP multiplexer")
	app.MCPArgs = []string{"mcp", "stdio"}

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lux", "plugin.json"))
	var plugin map[string]interface{}
	json.Unmarshal(data, &plugin)

	servers := plugin["mcpServers"].(map[string]interface{})
	srv := servers["lux"].(map[string]interface{})
	args := srv["args"].([]interface{})
	if len(args) != 2 || args[0] != "mcp" || args[1] != "stdio" {
		t.Errorf("args = %v, want [mcp stdio]", args)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestGeneratePlugin`
Expected: FAIL

**Step 3: Write minimal implementation**

Add `MCPArgs` to App in `app.go`:

```go
// Add to App struct:
	MCPArgs []string // extra args for MCP server command (e.g., "mcp", "stdio")
```

```go
// libs/go-mcp/command/generate_plugin.go
package command

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type pluginMcpServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type pluginManifest struct {
	Name       string                     `json:"name"`
	McpServers map[string]pluginMcpServer `json:"mcpServers"`
}

// GeneratePlugin writes a plugin.json manifest to {dir}/{app.Name}/plugin.json.
func (a *App) GeneratePlugin(dir string) error {
	manifest := pluginManifest{
		Name: a.Name,
		McpServers: map[string]pluginMcpServer{
			a.Name: {
				Type:    "stdio",
				Command: a.Name,
				Args:    a.MCPArgs,
			},
		},
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

**Step 4: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestGeneratePlugin`
Expected: PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/app.go libs/go-mcp/command/generate_plugin.go libs/go-mcp/command/generate_plugin_test.go
git commit -m "Add plugin.json generation from App"
```

---

### Task 6: Mappings generation (GenerateMappings)

**Files:**
- Create: `libs/go-mcp/command/generate_mappings.go`
- Create: `libs/go-mcp/command/generate_mappings_test.go`

**Step 1: Write the failing test**

```go
// libs/go-mcp/command/generate_mappings_test.go
package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateMappings(t *testing.T) {
	app := NewApp("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		MapsBash: []BashMapping{
			{Prefixes: []string{"git status"}, UseWhen: "checking repository status"},
		},
	})

	app.AddCommand(&Command{
		Name:        "diff",
		Description: Description{Short: "Show changes"},
		MapsBash: []BashMapping{
			{Prefixes: []string{"git diff"}, UseWhen: "viewing changes"},
		},
	})

	app.AddCommand(&Command{
		Name:        "internal",
		Description: Description{Short: "Internal only"},
		Hidden:      true,
	})

	dir := t.TempDir()
	if err := app.GenerateMappings(dir); err != nil {
		t.Fatalf("GenerateMappings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "grit", "mappings.json"))
	if err != nil {
		t.Fatalf("read mappings.json: %v", err)
	}

	var mf struct {
		Server   string `json:"server"`
		Mappings []struct {
			Replaces        string   `json:"replaces"`
			CommandPrefixes []string `json:"command_prefixes"`
			Tools           []struct {
				Name    string `json:"name"`
				UseWhen string `json:"use_when"`
			} `json:"tools"`
			Reason string `json:"reason"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(data, &mf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if mf.Server != "grit" {
		t.Errorf("server = %q, want grit", mf.Server)
	}
	if len(mf.Mappings) != 2 {
		t.Fatalf("mappings len = %d, want 2", len(mf.Mappings))
	}
	// All bash mappings replace "Bash"
	for _, m := range mf.Mappings {
		if m.Replaces != "Bash" {
			t.Errorf("replaces = %q, want Bash", m.Replaces)
		}
	}
}

func TestGenerateMappingsNoMappings(t *testing.T) {
	app := NewApp("test", "test")
	app.AddCommand(&Command{Name: "foo"})

	dir := t.TempDir()
	if err := app.GenerateMappings(dir); err != nil {
		t.Fatalf("GenerateMappings: %v", err)
	}

	// No mappings.json should be written when no commands have MapsBash
	path := filepath.Join(dir, "test", "mappings.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("mappings.json should not exist when no commands have bash mappings")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestGenerateMapping`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// libs/go-mcp/command/generate_mappings.go
package command

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type mappingToolSuggestion struct {
	Name    string `json:"name"`
	UseWhen string `json:"use_when"`
}

type mappingEntry struct {
	Replaces        string                  `json:"replaces"`
	CommandPrefixes []string                `json:"command_prefixes,omitempty"`
	Tools           []mappingToolSuggestion `json:"tools"`
	Reason          string                  `json:"reason"`
}

type mappingFile struct {
	Server   string         `json:"server"`
	Mappings []mappingEntry `json:"mappings"`
}

// GenerateMappings writes a mappings.json file to {dir}/{app.Name}/mappings.json.
// Only commands with MapsBash declarations are included. Each BashMapping on a
// command produces a separate mapping entry. If no commands have bash mappings,
// no file is written.
func (a *App) GenerateMappings(dir string) error {
	var entries []mappingEntry

	for _, cmd := range a.AllCommands() {
		if cmd.Hidden {
			continue
		}
		for _, bm := range cmd.MapsBash {
			entries = append(entries, mappingEntry{
				Replaces:        "Bash",
				CommandPrefixes: bm.Prefixes,
				Tools: []mappingToolSuggestion{
					{Name: cmd.Name, UseWhen: bm.UseWhen},
				},
				Reason: "Use the " + a.Name + " MCP tool instead of shelling out",
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

**Step 4: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestGenerateMapping`
Expected: PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate_mappings.go libs/go-mcp/command/generate_mappings_test.go
git commit -m "Add mappings.json generation from Command.MapsBash"
```

---

### Task 7: Manpage generation (GenerateManpages)

**Files:**
- Create: `libs/go-mcp/command/generate_manpages.go`
- Create: `libs/go-mcp/command/generate_manpages_test.go`

**Step 1: Write the failing test**

```go
// libs/go-mcp/command/generate_manpages_test.go
package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateManpageApp(t *testing.T) {
	app := NewApp("grit", "Git operations MCP server")
	app.Version = "0.1.0"
	app.Description.Long = "An MCP server exposing git operations."

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show working tree status"},
	})
	app.AddCommand(&Command{
		Name:   "generate-all",
		Hidden: true,
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	// App manpage should exist
	appPage, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "grit.1"))
	if err != nil {
		t.Fatalf("read grit.1: %v", err)
	}

	content := string(appPage)
	if !strings.Contains(content, ".TH GRIT 1") {
		t.Error("missing .TH header")
	}
	if !strings.Contains(content, "Git operations MCP server") {
		t.Error("missing short description in NAME")
	}
	if !strings.Contains(content, "An MCP server exposing git operations.") {
		t.Error("missing long description in DESCRIPTION")
	}
	if !strings.Contains(content, "status") {
		t.Error("missing status in COMMANDS")
	}
	// Hidden commands should not appear
	if strings.Contains(content, "generate-all") {
		t.Error("hidden command should not appear in manpage")
	}
}

func TestGenerateManpageCommand(t *testing.T) {
	app := NewApp("grit", "Git operations")

	app.AddCommand(&Command{
		Name: "status",
		Description: Description{
			Short: "Show working tree status",
			Long:  "Show working tree status with machine-readable output.",
		},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to the git repository", Required: true},
			{Name: "verbose", Type: Bool, Description: "Show verbose output", Default: false},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	cmdPage, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "grit-status.1"))
	if err != nil {
		t.Fatalf("read grit-status.1: %v", err)
	}

	content := string(cmdPage)
	if !strings.Contains(content, ".TH GRIT-STATUS 1") {
		t.Error("missing .TH header")
	}
	if !strings.Contains(content, "repo_path") {
		t.Error("missing repo_path in OPTIONS")
	}
	if !strings.Contains(content, "(required)") {
		t.Error("missing required marker")
	}
	if !strings.Contains(content, "Path to the git repository") {
		t.Error("missing param description")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestGenerateManpage`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// libs/go-mcp/command/generate_manpages.go
package command

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GenerateManpages writes roff-formatted manpages to {dir}/share/man/man1/.
// One page per app ({name}.1) and one per non-hidden command ({name}-{cmd}.1).
func (a *App) GenerateManpages(dir string) error {
	manDir := filepath.Join(dir, "share", "man", "man1")
	if err := os.MkdirAll(manDir, 0o755); err != nil {
		return err
	}

	if err := a.writeAppManpage(manDir); err != nil {
		return err
	}

	for _, cmd := range a.AllCommands() {
		if cmd.Hidden {
			continue
		}
		if err := a.writeCommandManpage(manDir, cmd); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) writeAppManpage(dir string) error {
	var b strings.Builder
	date := time.Now().Format("2006-01-02")
	name := strings.ToUpper(a.Name)

	fmt.Fprintf(&b, ".TH %s 1 %q %q\n", name, date, a.Name+" "+a.Version)
	fmt.Fprintf(&b, ".SH NAME\n")
	fmt.Fprintf(&b, "%s \\- %s\n", a.Name, a.Description.Short)

	if a.Description.Long != "" {
		fmt.Fprintf(&b, ".SH DESCRIPTION\n")
		fmt.Fprintf(&b, "%s\n", a.Description.Long)
	}

	// Collect and sort visible commands
	type namedCmd struct {
		name string
		cmd  *Command
	}
	var cmds []namedCmd
	for cmdName, cmd := range a.VisibleCommands() {
		cmds = append(cmds, namedCmd{cmdName, cmd})
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].name < cmds[j].name
	})

	if len(cmds) > 0 {
		fmt.Fprintf(&b, ".SH COMMANDS\n")
		for _, nc := range cmds {
			fmt.Fprintf(&b, ".TP\n")
			fmt.Fprintf(&b, ".B %s\n", nc.name)
			fmt.Fprintf(&b, "%s\n", nc.cmd.Description.Short)
		}
	}

	path := filepath.Join(dir, a.Name+".1")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func (a *App) writeCommandManpage(dir string, cmd *Command) error {
	var b strings.Builder
	date := time.Now().Format("2006-01-02")
	fullName := a.Name + "-" + cmd.Name
	upperName := strings.ToUpper(fullName)

	fmt.Fprintf(&b, ".TH %s 1 %q %q\n", upperName, date, a.Name+" "+a.Version)
	fmt.Fprintf(&b, ".SH NAME\n")
	fmt.Fprintf(&b, "%s \\- %s\n", fullName, cmd.Description.Short)

	// SYNOPSIS
	fmt.Fprintf(&b, ".SH SYNOPSIS\n")
	fmt.Fprintf(&b, ".B %s %s\n", a.Name, cmd.Name)
	for _, p := range cmd.Params {
		if p.Required {
			fmt.Fprintf(&b, ".RI --%s = %s\n", p.Name, strings.ToUpper(p.Type.JSONSchemaType()))
		} else {
			fmt.Fprintf(&b, ".RI [ --%s = %s ]\n", p.Name, strings.ToUpper(p.Type.JSONSchemaType()))
		}
	}

	// DESCRIPTION
	desc := cmd.Description.Long
	if desc == "" {
		desc = cmd.Description.Short
	}
	fmt.Fprintf(&b, ".SH DESCRIPTION\n")
	fmt.Fprintf(&b, "%s\n", desc)

	// OPTIONS
	if len(cmd.Params) > 0 {
		fmt.Fprintf(&b, ".SH OPTIONS\n")
		for _, p := range cmd.Params {
			fmt.Fprintf(&b, ".TP\n")
			label := fmt.Sprintf("--%s", p.Name)
			if p.Required {
				label += " (required)"
			}
			fmt.Fprintf(&b, ".B %s\n", label)
			fmt.Fprintf(&b, "%s\n", p.Description)
			if p.Default != nil {
				fmt.Fprintf(&b, "Default: %v\n", p.Default)
			}
		}
	}

	// ALIASES
	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(&b, ".SH ALIASES\n")
		fmt.Fprintf(&b, "%s\n", strings.Join(cmd.Aliases, ", "))
	}

	path := filepath.Join(dir, fullName+".1")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
```

**Step 4: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestGenerateManpage`
Expected: PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate_manpages.go libs/go-mcp/command/generate_manpages_test.go
git commit -m "Add manpage generation from App command tree"
```

---

### Task 8: Shell completion generation (GenerateCompletions)

**Files:**
- Create: `libs/go-mcp/command/generate_completions.go`
- Create: `libs/go-mcp/command/generate_completions_test.go`

**Step 1: Write the failing test**

```go
// libs/go-mcp/command/generate_completions_test.go
package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCompletionsBash(t *testing.T) {
	app := NewApp("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
		},
	})
	app.AddCommand(&Command{
		Name:        "diff",
		Description: Description{Short: "Show changes"},
	})
	app.AddCommand(&Command{Name: "hidden", Hidden: true})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	bashPath := filepath.Join(dir, "share", "bash-completion", "completions", "grit")
	data, err := os.ReadFile(bashPath)
	if err != nil {
		t.Fatalf("read bash completion: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "status") {
		t.Error("bash completion missing status command")
	}
	if !strings.Contains(content, "diff") {
		t.Error("bash completion missing diff command")
	}
	if strings.Contains(content, "hidden") {
		t.Error("bash completion should not contain hidden commands")
	}
	if !strings.Contains(content, "repo_path") {
		t.Error("bash completion missing repo_path flag")
	}
}

func TestGenerateCompletionsZsh(t *testing.T) {
	app := NewApp("grit", "Git operations")
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	zshPath := filepath.Join(dir, "share", "zsh", "site-functions", "_grit")
	data, err := os.ReadFile(zshPath)
	if err != nil {
		t.Fatalf("read zsh completion: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "#compdef grit") {
		t.Error("zsh completion missing #compdef header")
	}
	if !strings.Contains(content, "status") {
		t.Error("zsh completion missing status command")
	}
}

func TestGenerateCompletionsFish(t *testing.T) {
	app := NewApp("grit", "Git operations")
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "grit.fish")
	data, err := os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish completion: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "complete -c grit") {
		t.Error("fish completion missing complete -c header")
	}
	if !strings.Contains(content, "status") {
		t.Error("fish completion missing status command")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestGenerateCompletion`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// libs/go-mcp/command/generate_completions.go
package command

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GenerateCompletions writes shell completion scripts for bash, zsh, and fish
// to standard paths under {dir}/share/.
func (a *App) GenerateCompletions(dir string) error {
	if err := a.generateBashCompletion(dir); err != nil {
		return err
	}
	if err := a.generateZshCompletion(dir); err != nil {
		return err
	}
	return a.generateFishCompletion(dir)
}

func (a *App) sortedVisibleCommands() []struct {
	name string
	cmd  *Command
} {
	type entry struct {
		name string
		cmd  *Command
	}
	var cmds []entry
	for name, cmd := range a.VisibleCommands() {
		cmds = append(cmds, entry{name, cmd})
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].name < cmds[j].name
	})
	return cmds
}

func (a *App) generateBashCompletion(dir string) error {
	bashDir := filepath.Join(dir, "share", "bash-completion", "completions")
	if err := os.MkdirAll(bashDir, 0o755); err != nil {
		return err
	}

	cmds := a.sortedVisibleCommands()

	var b strings.Builder
	fmt.Fprintf(&b, "# bash completion for %s\n\n", a.Name)
	fmt.Fprintf(&b, "_%s() {\n", a.Name)
	fmt.Fprintf(&b, "    local cur prev commands\n")
	fmt.Fprintf(&b, "    COMPREPLY=()\n")
	fmt.Fprintf(&b, "    cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	fmt.Fprintf(&b, "    prev=\"${COMP_WORDS[COMP_CWORD-1]}\"\n\n")

	// Subcommand names
	var names []string
	for _, c := range cmds {
		names = append(names, c.name)
	}
	fmt.Fprintf(&b, "    commands=%q\n\n", strings.Join(names, " "))

	// If completing first arg, complete subcommand names
	fmt.Fprintf(&b, "    if [[ ${COMP_CWORD} -eq 1 ]]; then\n")
	fmt.Fprintf(&b, "        COMPREPLY=( $(compgen -W \"${commands}\" -- \"${cur}\") )\n")
	fmt.Fprintf(&b, "        return 0\n")
	fmt.Fprintf(&b, "    fi\n\n")

	// Per-subcommand flag completion
	fmt.Fprintf(&b, "    local subcmd=\"${COMP_WORDS[1]}\"\n")
	fmt.Fprintf(&b, "    case \"${subcmd}\" in\n")
	for _, c := range cmds {
		var flags []string
		for _, p := range c.cmd.Params {
			flags = append(flags, "--"+p.Name)
		}
		if len(flags) > 0 {
			fmt.Fprintf(&b, "        %s)\n", c.name)
			fmt.Fprintf(&b, "            COMPREPLY=( $(compgen -W %q -- \"${cur}\") )\n", strings.Join(flags, " "))
			fmt.Fprintf(&b, "            ;;\n")
		}
	}
	fmt.Fprintf(&b, "    esac\n")
	fmt.Fprintf(&b, "}\n\n")
	fmt.Fprintf(&b, "complete -F _%s %s\n", a.Name, a.Name)

	return os.WriteFile(filepath.Join(bashDir, a.Name), []byte(b.String()), 0o644)
}

func (a *App) generateZshCompletion(dir string) error {
	zshDir := filepath.Join(dir, "share", "zsh", "site-functions")
	if err := os.MkdirAll(zshDir, 0o755); err != nil {
		return err
	}

	cmds := a.sortedVisibleCommands()

	var b strings.Builder
	fmt.Fprintf(&b, "#compdef %s\n\n", a.Name)
	fmt.Fprintf(&b, "_%s() {\n", a.Name)
	fmt.Fprintf(&b, "    local -a commands\n")
	fmt.Fprintf(&b, "    commands=(\n")
	for _, c := range cmds {
		desc := strings.ReplaceAll(c.cmd.Description.Short, "'", "'\\''")
		fmt.Fprintf(&b, "        '%s:%s'\n", c.name, desc)
	}
	fmt.Fprintf(&b, "    )\n\n")
	fmt.Fprintf(&b, "    _describe 'command' commands\n")
	fmt.Fprintf(&b, "}\n\n")
	fmt.Fprintf(&b, "_%s\n", a.Name)

	return os.WriteFile(filepath.Join(zshDir, "_"+a.Name), []byte(b.String()), 0o644)
}

func (a *App) generateFishCompletion(dir string) error {
	fishDir := filepath.Join(dir, "share", "fish", "vendor_completions.d")
	if err := os.MkdirAll(fishDir, 0o755); err != nil {
		return err
	}

	cmds := a.sortedVisibleCommands()

	var b strings.Builder
	fmt.Fprintf(&b, "# fish completion for %s\n\n", a.Name)

	// Disable file completion by default
	fmt.Fprintf(&b, "complete -c %s -f\n\n", a.Name)

	// Subcommand completions
	for _, c := range cmds {
		desc := strings.ReplaceAll(c.cmd.Description.Short, "'", "\\'")
		fmt.Fprintf(&b, "complete -c %s -n '__fish_use_subcommand' -a %s -d '%s'\n",
			a.Name, c.name, desc)
	}

	// Per-subcommand flag completions
	for _, c := range cmds {
		for _, p := range c.cmd.Params {
			desc := strings.ReplaceAll(p.Description, "'", "\\'")
			fmt.Fprintf(&b, "complete -c %s -n '__fish_seen_subcommand_from %s' -l %s -d '%s'\n",
				a.Name, c.name, p.Name, desc)
		}
	}

	return os.WriteFile(filepath.Join(fishDir, a.Name+".fish"), []byte(b.String()), 0o644)
}
```

**Step 4: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestGenerateCompletion`
Expected: PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate_completions.go libs/go-mcp/command/generate_completions_test.go
git commit -m "Add shell completion generation for bash, zsh, and fish"
```

---

### Task 9: GenerateAll and CLI entry point

**Files:**
- Create: `libs/go-mcp/command/generate.go`
- Create: `libs/go-mcp/command/cli.go`
- Create: `libs/go-mcp/command/cli_test.go`

**Step 1: Write the failing test**

```go
// libs/go-mcp/command/cli_test.go
package command

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAll(t *testing.T) {
	app := NewApp("grit", "Git operations")
	app.Version = "0.1.0"

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
		},
		MapsBash: []BashMapping{
			{Prefixes: []string{"git status"}, UseWhen: "checking status"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateAll(dir); err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}

	// Verify all expected files exist
	expected := []string{
		filepath.Join("share", "purse-first", "grit", "plugin.json"),
		filepath.Join("share", "purse-first", "grit", "mappings.json"),
		filepath.Join("share", "man", "man1", "grit.1"),
		filepath.Join("share", "man", "man1", "grit-status.1"),
		filepath.Join("share", "bash-completion", "completions", "grit"),
		filepath.Join("share", "zsh", "site-functions", "_grit"),
		filepath.Join("share", "fish", "vendor_completions.d", "grit.fish"),
	}

	for _, rel := range expected {
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file missing: %s", rel)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestGenerateAll`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// libs/go-mcp/command/generate.go
package command

import "path/filepath"

// GenerateAll writes all artifacts (plugin manifest, mappings, manpages,
// and shell completions) to standard paths under dir.
//
// Output layout:
//
//   {dir}/share/purse-first/{name}/plugin.json
//   {dir}/share/purse-first/{name}/mappings.json (if any commands have MapsBash)
//   {dir}/share/man/man1/{name}.1
//   {dir}/share/man/man1/{name}-{cmd}.1 (per visible command)
//   {dir}/share/bash-completion/completions/{name}
//   {dir}/share/zsh/site-functions/_{name}
//   {dir}/share/fish/vendor_completions.d/{name}.fish
func (a *App) GenerateAll(dir string) error {
	purseDir := filepath.Join(dir, "share", "purse-first")

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

**Step 4: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./command/ -v -run TestGenerateAll`
Expected: PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate.go libs/go-mcp/command/cli_test.go
git commit -m "Add GenerateAll to emit all artifacts from a single call"
```

---

### Task 10: Run all tests and verify clean build

**Files:** None (verification only)

**Step 1: Run go-mcp tests**

Run: `cd libs/go-mcp && go test ./... -v`
Expected: all tests pass

**Step 2: Run purse-first tests**

Run: `just test`
Expected: all tests pass (Go workspace includes new package)

**Step 3: Verify go vet is clean**

Run: `cd libs/go-mcp && go vet ./...`
Expected: no issues

**Step 4: Format code**

Run: `cd libs/go-mcp && gofmt -l ./command/`
Expected: no files listed (already formatted)

**Step 5: Commit any formatting fixes**

If any formatting fixes were needed:
```bash
git add -A && git commit -m "Format command package"
```

---

### Task 11: Add justfile targets for the command package

**Files:**
- Modify: `justfile`

**Step 1: Add test target**

Append to `justfile`:

```just
# Test command package specifically
test-command:
    cd libs/go-mcp && go test -v ./command/
```

**Step 2: Verify target works**

Run: `just test-command`
Expected: all command tests pass

**Step 3: Commit**

```bash
git add justfile
git commit -m "Add just test-command target for command package"
```

---

## Layout After Implementation

```
libs/go-mcp/
├── command/
│   ├── command.go              # Command, Param, Description, BashMapping types
│   ├── command_test.go         # Type tests
│   ├── app.go                  # App registry, AddCommand, MergeWithPrefix
│   ├── app_test.go             # Registry tests
│   ├── schema.go               # InputSchema() — JSON Schema from Params
│   ├── schema_test.go          # Schema generation tests
│   ├── mcp.go                  # RegisterMCPTools bridge
│   ├── mcp_test.go             # MCP integration tests
│   ├── generate.go             # GenerateAll orchestrator
│   ├── generate_plugin.go      # plugin.json writer
│   ├── generate_plugin_test.go
│   ├── generate_mappings.go    # mappings.json writer
│   ├── generate_mappings_test.go
│   ├── generate_manpages.go    # roff manpage writer
│   ├── generate_manpages_test.go
│   ├── generate_completions.go # bash/zsh/fish completion writer
│   └── generate_completions_test.go
├── protocol/
├── server/
├── transport/
├── ...
```

## Future Tasks (not in this plan)

These are follow-up work after the core package is stable:

- **CLI parser** — `App.RunCLI(args)` to dispatch subcommands and parse flags from `Param` declarations
- **`App.Main()`** — unified entry point (CLI vs MCP mode dispatch)
- **Built-in help/complete subcommands** — auto-registered in App
- **`App.ServeMCP()`** — convenience wrapper combining RegisterMCPTools + server.New + Run
- **Migrate grit** — replace flag + ToolRegistry.Register + PluginBuilder with App.AddCommand
- **Migrate lux** — remove cobra dependency
- **Migrate get-hubbed** — same pattern
- **Rust command package** — struct-based API in libs/rust-mcp
- **Rust derive macro** — `#[derive(Command)]`
