# Unified Command Framework for purse-first

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace flag/cobra/clap and the separate `purse` builder package with a unified `command` package in go-lib-mcp and rust-lib-mcp. A single `Command` struct declaration produces: CLI parser, MCP tool registration, plugin.json, mappings.json, manpages, and shell completions.

**Motivation:** Today each MCP server maintains tool metadata in three independent places — `ToolRegistry.Register()` calls, `PluginBuilder` chains, and hand-written help text. Adding a tool means touching all three, and they drift. This framework makes one declaration the single source of truth.

---

## Core Data Model (Go)

### Command

```go
// libs/go-mcp/command/command.go

type Command struct {
    Name        string
    Aliases     []string
    Description Description
    Hidden      bool

    Params      []Param

    Subcommands map[string]*Command

    // Unified handler — works for both CLI and MCP.
    // CLI mode prints text content to stdout.
    Run         func(Request) (*protocol.ToolCallResult, error)

    // Optional overrides when CLI and MCP need different behavior.
    // TODO: Ergonomics of unified vs split handlers needs refinement
    // during implementation. The return type and CLI printing
    // convention may evolve.
    RunCLI      func(Request)
    RunMCP      ToolHandler

    MapsBash    []BashMapping
}

type Description struct {
    Short string   // one-line: manpage NAME, completion tab text, MCP tool description
    Long  string   // paragraph: manpage DESCRIPTION, --help output
}

type Param struct {
    Name        string
    Type        ParamType       // String, Int, Bool, Float
    Description string
    Required    bool
    Default     any
    Completer   func() map[string]string
}

type ParamType int

const (
    String ParamType = iota
    Int
    Bool
    Float
)

type BashMapping struct {
    Prefixes []string   // e.g., "git status"
    UseWhen  string     // shown to Claude in mapping denial
}
```

### App

```go
// libs/go-mcp/command/app.go

type App struct {
    Name        string
    Description Description
    Version     string
    Params      []Param               // global flags (e.g., --sse, --port)
    commands    map[string]*Command   // flat registry
}

func NewApp(name, short string) *App

func (a *App) AddCommand(cmd *Command)

// Dodder-style prefix merging for composing utilities
func (a *App) MergeWithPrefix(other *App, prefix string)
```

### Request

```go
// libs/go-mcp/command/request.go

type Request struct {
    Context context.Context
    Params  map[string]any    // normalized from CLI flags or MCP JSON args
    Args    []string          // positional args (CLI only)
}

func (r Request) String(name string) string
func (r Request) Int(name string) int
func (r Request) Bool(name string) bool
func (r Request) StringOr(name, fallback string) string
```

Both CLI flags and MCP JSON arguments are normalized into the same `Params` map, so most handlers work identically in both modes.

### Registration Pattern

Commands are registered as plain structs at init time (dodder pattern):

```go
func init() {
    app.AddCommand(&command.Command{
        Name: "status",
        Description: command.Description{
            Short: "Show working tree status",
            Long:  "Show working tree status with machine-readable output",
        },
        Params: []command.Param{
            {Name: "repo_path", Type: command.String,
             Description: "Path to the git repository", Required: true},
        },
        Run:      handleStatus,
        MapsBash: []command.BashMapping{
            {Prefixes: []string{"git status"}, UseWhen: "checking repository status"},
        },
    })
}
```

---

## Generation Outputs

All generators walk the same command tree:

```go
// All-in-one for Nix postInstall
func (a *App) GenerateAll(dir string) error

// Individual generators
func (a *App) GenerateManpages(dir string) error
func (a *App) GenerateCompletions(dir string) error
func (a *App) GeneratePlugin(dir string) error
func (a *App) GenerateMappings(dir string) error

// Runtime
func (a *App) RegisterMCPTools(registry *server.ToolRegistry)
func (a *App) RunCLI(args []string)
func (a *App) ServeMCP(ctx context.Context, transport transport.Transport) error
func (a *App) Main()
```

### What Each Generator Derives

| Field | Manpage | Completion | plugin.json | mappings.json | MCP schema | CLI parser |
|-------|---------|------------|-------------|---------------|------------|------------|
| `Name` | section header | subcommand match | mcpServers key | — | tool name | subcommand dispatch |
| `Description.Short` | NAME section | tab description | — | — | tool description | help text |
| `Description.Long` | DESCRIPTION | — | — | — | — | `--help` output |
| `Params` | OPTIONS/SYNOPSIS | flag completions | — | — | inputSchema properties | flag definitions |
| `Param.Required` | bold in SYNOPSIS | — | — | — | schema required array | parse validation |
| `Param.Default` | shown in OPTIONS | — | — | — | schema default | flag default |
| `MapsBash` | — | — | — | commandPrefixes + tools | — | — |
| `Aliases` | SEE ALSO | alias completion | — | — | — | alias dispatch |
| `Hidden` | skip | skip | skip | skip | skip | skip in help |

### ParamType Mapping

| ParamType | JSON Schema type | roff format |
|-----------|-----------------|-------------|
| `String` | `"string"` | `=STRING` |
| `Int` | `"integer"` | `=N` |
| `Bool` | `"boolean"` | (flag-style) |
| `Float` | `"number"` | `=NUM` |

### Manpage Output

Per-app page (`grit.1`) lists all commands. Per-command pages (`grit-status.1`) document params:

```
GRIT-STATUS(1)

NAME
    grit-status — Show working tree status

SYNOPSIS
    grit status --repo_path=PATH

DESCRIPTION
    Show working tree status with machine-readable output.

OPTIONS
    --repo_path=PATH (required)
        Path to the git repository
```

### Completion Output

Emitted to standard paths:
- `share/bash-completion/completions/<app>`
- `share/zsh/site-functions/_<app>`
- `share/fish/vendor_completions.d/<app>.fish`

### Plugin + Mappings Output

Emitted to `share/purse-first/<app>/plugin.json` and `share/purse-first/<app>/mappings.json`, identical format to today.

---

## Runtime Execution

```go
func (a *App) Main() {
    // 1. Parse global flags from Params
    // 2. If subcommand present → RunCLI dispatch
    //    (includes built-in: help, complete, generate-all)
    // 3. Otherwise → ServeMCP on stdio
}
```

Built-in subcommands added automatically:
- **`help`** — generated from Description + Params
- **`complete`** — dodder-style runtime introspection of commands and flags
- **`generate-all`** — calls GenerateAll (hidden, for Nix postInstall)

---

## Rust API (rust-lib-mcp)

Idiomatic Rust using derive macros:

```rust
use purse::Command;

#[derive(Command)]
#[command(name = "status", short = "Show working tree status")]
#[command(long = "Show working tree status with machine-readable output")]
#[command(bash = "git status", use_when = "checking repository status")]
struct Status {
    /// Path to the git repository
    #[param(required)]
    repo_path: String,
}

#[derive(Command)]
#[command(name = "diff", short = "Show changes in the working tree")]
#[command(bash = "git diff", use_when = "viewing changes")]
struct Diff {
    /// Path to the git repository
    #[param(required)]
    repo_path: String,

    /// Show staged changes (--cached)
    #[param(default = false)]
    staged: bool,

    /// Show only diffstat summary
    #[param(default = false)]
    stat_only: bool,
}
```

Key Rust idioms:
- **Derive macros** — `#[derive(Command)]` generates param introspection, JSON schema, flag parsing, and manpage metadata at compile time
- **Doc comments as descriptions** — `///` becomes both manpage text and JSON schema description
- **Type system for params** — `String` → JSON `"string"`, `bool` → `"boolean"`, etc. No `ParamType` enum; derive macro reads the Rust type
- **`Option<T>` for optional params** — `repo_path: String` is required, `staged: Option<bool>` is optional

App assembly:

```rust
let app = App::new("chix", "Nix CLI operations MCP server")
    .version("0.1.0")
    .command::<Status>()
    .command::<Diff>()
    .command::<Build>();

app.generate_all(dir)?;
app.serve_mcp(stdio_transport).await?;
```

**Phasing:** Initial Rust implementation uses struct-based API matching Go. Derive macro follows once the data model stabilizes.

---

## Nix Integration

`postInstall` calls the built-in `generate-all` subcommand:

```nix
postInstall = ''
  $out/bin/grit generate-all $out
'';
```

This outputs to standard paths:
```
$out/share/purse-first/grit/plugin.json
$out/share/purse-first/grit/mappings.json
$out/share/man/man1/grit.1
$out/share/man/man1/grit-status.1
$out/share/man/man1/grit-diff.1
$out/share/bash-completion/completions/grit
$out/share/zsh/site-functions/_grit
$out/share/fish/vendor_completions.d/grit.fish
```

---

## Migration Plan

### Phase 1: Go command package (libs/go-mcp)

Build the `command` package in libs/go-mcp with:
- `App`, `Command`, `Param`, `Request` types
- CLI parser (replaces flag/cobra)
- MCP tool registration (replaces ToolRegistry.Register boilerplate)
- `GenerateAll` (plugin.json, mappings.json, manpages, completions)
- Built-in `help`, `complete`, `generate-all` subcommands

### Phase 2: Migrate grit

Grit is the simplest consumer (flag-based, ~15 tools). Migration removes:
- Hand-written `json.RawMessage` schemas from 7 tool registration files
- 140-line `PluginBuilder` chain in main.go
- Manual `flag.Usage` help text
- Direct `flag` and `purse` imports

### Phase 3: Migrate lux

Lux uses cobra. Migration removes cobra dependency entirely. Same pattern as grit but more tools and longer descriptions.

### Phase 4: Migrate get-hubbed

Same pattern. Removes cobra or flag dependency.

### Phase 5: Deprecate purse package

Once all Go consumers migrate, the `purse` package (PluginBuilder, MappingBuilder, Writer) is removed. The `plugin-mcp` skill is updated to reference `command.NewApp` patterns.

### Phase 6: Rust command package (libs/rust-mcp)

Build the struct-based API in rust-lib-mcp. Migrate chix.

### Phase 7: Rust derive macro

Add `#[derive(Command)]` proc macro for ergonomic Rust declarations.

---

## Open Design Questions

1. **Handler ergonomics** — The unified `Run` vs split `RunCLI`/`RunMCP` interface needs refinement. The return type (`*ToolCallResult`) may not be natural for CLI-only commands. Consider a simpler return type with adapter functions, or a trait/interface pattern.

2. **Positional args** — MCP tools don't have positional arguments (everything is named in JSON). CLI subcommands sometimes do (e.g., `grit generate-all <dir>`). The `Request.Args` field handles this but the boundary needs testing.

3. **Streaming output** — Some CLI commands stream output line by line. MCP tools return a single result. The framework may need a streaming adapter.

4. **Language-agnostic schema** — A future direction could define the command tree in a JSON/TOML schema and generate Go/Rust code from it, rather than having separate Go and Rust declarations. Flagged for future exploration.

---

## Decisions

- **Unified framework** — one `Command` declaration produces all artifacts, replacing flag/cobra/clap and the `purse` builder package
- **Go first, Rust follows** — Go API stabilizes with grit/lux/get-hubbed before Rust implementation
- **Derive macros for Rust** — idiomatic Rust with `#[derive(Command)]`, doc comments as descriptions, `Option<T>` for optional params
- **Flat registry** — dodder-style flat command map with prefix merging, not nested cobra-style trees
- **Built-in subcommands** — help, complete, generate-all added automatically
- **Skills out of scope** — SKILL.md files remain hand-written
- **`generate-all` in postInstall** — single command in Nix build outputs everything to standard `$out/share/` paths
