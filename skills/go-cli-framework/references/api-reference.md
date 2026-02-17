# go-lib-mcp API Reference

Complete type and method reference for `github.com/amarbel-llc/go-lib-mcp`.

## command

Unified command abstraction that generates CLI parsing, MCP tool registration, plugin manifests, bash mappings, manpages, and shell completions from a single definition.

### ParamType

```go
type ParamType int

const (
    String ParamType = iota
    Int
    Bool
    Float
    Array
)
```

`JSONSchemaType() string` — Returns the JSON Schema type name (`"string"`, `"integer"`, `"boolean"`, `"number"`, `"array"`).

### Description

```go
type Description struct {
    Short string // one-line: manpage NAME, completion tab text, MCP tool description
    Long  string // paragraph: manpage DESCRIPTION, --help output
}
```

### BashMapping

Declares a bash command prefix that should be intercepted and redirected to this command's MCP tool.

```go
type BashMapping struct {
    Prefixes []string // e.g., "git status"
    UseWhen  string   // shown to Claude in mapping denial
}
```

### Param

```go
type Param struct {
    Name        string
    Type        ParamType
    Description string
    Required    bool
    Default     any
    Completer   func() map[string]string
}
```

### Command

```go
type Command struct {
    Name        string
    Aliases     []string
    Description Description
    Hidden      bool
    Params      []Param
    MapsBash    []BashMapping
    RunMCP      func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error)
}
```

Methods:
- `RequiredParams() []Param` — Returns only params marked as required.
- `OptionalParams() []Param` — Returns only params not marked as required.
- `InputSchema() json.RawMessage` — Returns a JSON Schema describing this command's parameters, suitable for MCP tool inputSchema.

### App

```go
type App struct {
    Name        string
    Description Description
    Version     string
    MCPArgs     []string // extra args passed to the binary in plugin manifests
    Params      []Param  // global flags
}
```

Constructor:
- `NewApp(name, short string) *App` — Creates a new App with the given name and short description.

Command management:
- `AddCommand(cmd *Command)` — Registers a command and its aliases. **Panics on duplicate names.**
- `GetCommand(name string) (*Command, bool)` — Looks up a command by name or alias.
- `AllCommands() func(yield func(string, *Command) bool)` — Iterates over all registered commands (including hidden). Each unique command is yielded once even if it has aliases.
- `VisibleCommands() func(yield func(string, *Command) bool)` — Iterates over non-hidden commands.
- `MergeWithPrefix(other *App, prefix string)` — Adds all commands from another App, prefixed with the given string (e.g., `"store"` → `"store-ls"`, `"store-cat"`).

MCP integration:
- `RegisterMCPTools(registry *server.ToolRegistry)` — Registers all non-hidden commands that have a `RunMCP` handler as MCP tools, using each command's description and auto-generated JSON schema.

Artifact generation:
- `GenerateAll(dir string) error` — Writes all artifacts to standard paths under `dir`:
  - `{dir}/share/purse-first/{name}/plugin.json`
  - `{dir}/share/purse-first/{name}/mappings.json` (if any commands have MapsBash)
  - `{dir}/share/man/man1/{name}.1`
  - `{dir}/share/man/man1/{name}-{cmd}.1` (per visible command)
  - `{dir}/share/bash-completion/completions/{name}`
  - `{dir}/share/zsh/site-functions/_{name}`
  - `{dir}/share/fish/vendor_completions.d/{name}.fish`
- `GeneratePlugin(dir string) error` — Writes only `{dir}/{name}/plugin.json`.
- `GenerateMappings(dir string) error` — Writes only `{dir}/{name}/mappings.json`. Skips if no commands have bash mappings.
- `GenerateManpages(dir string) error` — Writes roff manpages to `{dir}/share/man/man1/`.
- `GenerateCompletions(dir string) error` — Writes bash, zsh, and fish completion scripts.

---

## server

MCP server scaffolding with lifecycle management and provider registries.

### Server

```go
type Server struct { /* unexported */ }
```

- `New(t transport.Transport, opts Options) (*Server, error)` — Creates a new MCP server. Requires `opts.ServerName` to be set.
- `Run(ctx context.Context) error` — Starts the server and processes messages until the context is canceled or the transport is closed. Handles concurrent request processing and graceful shutdown.
- `Close()` — Signals the server to shut down gracefully.

### Options

```go
type Options struct {
    ServerName    string           // required
    ServerVersion string           // optional
    Tools         ToolProvider     // optional; enables tools capability
    Resources     ResourceProvider // optional; enables resources capability
    Prompts       PromptProvider   // optional; enables prompts capability
}
```

Capabilities are automatically advertised based on which providers are set.

### Provider Interfaces

```go
type ToolProvider interface {
    ListTools(ctx context.Context) ([]protocol.Tool, error)
    CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error)
}

type ResourceProvider interface {
    ListResources(ctx context.Context) ([]protocol.Resource, error)
    ReadResource(ctx context.Context, uri string) (*protocol.ResourceReadResult, error)
    ListResourceTemplates(ctx context.Context) ([]protocol.ResourceTemplate, error)
}

type PromptProvider interface {
    ListPrompts(ctx context.Context) ([]protocol.Prompt, error)
    GetPrompt(ctx context.Context, name string, args map[string]string) (*protocol.PromptGetResult, error)
}
```

Implement these directly for custom dispatch logic, or use the registry helpers below.

### ToolRegistry

```go
func NewToolRegistry() *ToolRegistry

func (r *ToolRegistry) Register(name, description string, schema json.RawMessage, handler ToolHandler)
func (r *ToolRegistry) ListTools(ctx context.Context) ([]protocol.Tool, error)
func (r *ToolRegistry) CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error)
```

`ToolHandler` is `func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error)`.

### ResourceRegistry

```go
func NewResourceRegistry() *ResourceRegistry

func (r *ResourceRegistry) RegisterResource(resource protocol.Resource, reader ResourceReader)
func (r *ResourceRegistry) RegisterTemplate(template protocol.ResourceTemplate, reader ResourceReader)
func (r *ResourceRegistry) ListResources(ctx context.Context) ([]protocol.Resource, error)
func (r *ResourceRegistry) ReadResource(ctx context.Context, uri string) (*protocol.ResourceReadResult, error)
func (r *ResourceRegistry) ListResourceTemplates(ctx context.Context) ([]protocol.ResourceTemplate, error)
```

`ResourceReader` is `func(ctx context.Context, uri string) (*protocol.ResourceReadResult, error)`.

### PromptRegistry

```go
func NewPromptRegistry() *PromptRegistry

func (r *PromptRegistry) Register(prompt protocol.Prompt, renderer PromptRenderer)
func (r *PromptRegistry) ListPrompts(ctx context.Context) ([]protocol.Prompt, error)
func (r *PromptRegistry) GetPrompt(ctx context.Context, name string, args map[string]string) (*protocol.PromptGetResult, error)
```

`PromptRenderer` is `func(ctx context.Context, args map[string]string) (*protocol.PromptGetResult, error)`.

---

## protocol

MCP protocol types and constants. Implements MCP protocol version `2024-11-05`.

### Tool

```go
type Tool struct {
    Name        string          `json:"name"`
    Description string          `json:"description,omitempty"`
    InputSchema json.RawMessage `json:"inputSchema"`
}
```

### ToolCallParams

```go
type ToolCallParams struct {
    Name      string          `json:"name"`
    Arguments json.RawMessage `json:"arguments,omitempty"`
}
```

### ToolCallResult

```go
type ToolCallResult struct {
    Content []ContentBlock `json:"content"`
    IsError bool           `json:"isError,omitempty"`
}
```

### ContentBlock

```go
type ContentBlock struct {
    Type     string `json:"type"`
    Text     string `json:"text"`
    MimeType string `json:"mimeType,omitempty"`
    Data     string `json:"data,omitempty"`
}
```

### Helper Functions

- `TextContent(text string) ContentBlock` — Creates a ContentBlock containing plain text.
- `ErrorResult(msg string) *ToolCallResult` — Creates a ToolCallResult representing an error.

### Resource

```go
type Resource struct {
    URI         string `json:"uri"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    MimeType    string `json:"mimeType,omitempty"`
}
```

### ResourceTemplate

```go
type ResourceTemplate struct {
    URITemplate string `json:"uriTemplate"` // RFC 6570
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    MimeType    string `json:"mimeType,omitempty"`
}
```

### ResourceContent

```go
type ResourceContent struct {
    URI      string `json:"uri"`
    MimeType string `json:"mimeType,omitempty"`
    Text     string `json:"text,omitempty"`     // text content
    Blob     string `json:"blob,omitempty"`     // base64-encoded binary
}
```

### ResourceReadResult

```go
type ResourceReadResult struct {
    Contents []ResourceContent `json:"contents"`
}
```

### Prompt

```go
type Prompt struct {
    Name        string           `json:"name"`
    Description string           `json:"description,omitempty"`
    Arguments   []PromptArgument `json:"arguments,omitempty"`
}

type PromptArgument struct {
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
    Required    bool   `json:"required,omitempty"`
}
```

### PromptGetResult

```go
type PromptGetResult struct {
    Description string          `json:"description,omitempty"`
    Messages    []PromptMessage `json:"messages"`
}

type PromptMessage struct {
    Role    string       `json:"role"`    // "user" or "assistant"
    Content ContentBlock `json:"content"`
}
```

### Implementation

```go
type Implementation struct {
    Name    string `json:"name"`
    Version string `json:"version,omitempty"`
}
```

### Capabilities

```go
type ServerCapabilities struct {
    Tools     *ToolsCapability     `json:"tools,omitempty"`
    Resources *ResourcesCapability `json:"resources,omitempty"`
    Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

type ToolsCapability struct {
    ListChanged bool `json:"listChanged,omitempty"`
}

type ResourcesCapability struct {
    Subscribe   bool `json:"subscribe,omitempty"`
    ListChanged bool `json:"listChanged,omitempty"`
}

type PromptsCapability struct {
    ListChanged bool `json:"listChanged,omitempty"`
}
```

### Protocol Methods

```go
const (
    MethodInitialize       = "initialize"
    MethodInitialized      = "notifications/initialized"
    MethodPing             = "ping"
    MethodToolsList        = "tools/list"
    MethodToolsCall        = "tools/call"
    MethodResourcesList    = "resources/list"
    MethodResourcesRead    = "resources/read"
    MethodResourcesTemplates = "resources/templates/list"
    MethodPromptsList      = "prompts/list"
    MethodPromptsGet       = "prompts/get"
)
```

---

## transport

Transport layer for sending and receiving JSON-RPC messages.

### Transport Interface

```go
type Transport interface {
    Read() (*jsonrpc.Message, error)   // returns io.EOF on graceful close
    Write(*jsonrpc.Message) error
    Close() error
}
```

### Stdio

MCP stdio transport using newline-delimited JSON (not Content-Length headers like LSP).

```go
func NewStdio(r io.Reader, w io.Writer) *Stdio
func NewStdioWithCloser(r io.Reader, w io.Writer, c io.Closer) *Stdio
```

Buffer: 64KB initial, 1MB max per message.

---

## output

Context-saving utilities for limiting tool output size. See the **bob:context-saving** skill for the full pattern.

### Array Pagination

```go
type ArrayLimits struct {
    Limit  int `json:"limit,omitempty"`   // max items to return; 0 = unlimited
    Offset int `json:"offset,omitempty"`  // skip first N items; 0 = no offset
}

type PaginationInfo struct {
    Offset  int  `json:"offset"`
    Limit   int  `json:"limit"`
    Total   int  `json:"total"`
    HasMore bool `json:"has_more"`
}

type LimitedArray[T any] struct {
    Items      []T            `json:"items"`
    Truncated  bool           `json:"truncated"`
    TotalCount int            `json:"total_count"`
    Pagination PaginationInfo `json:"pagination"`
}

func LimitArray[T any](items []T, limits ArrayLimits) LimitedArray[T]
```

`LimitArray` returns a subslice (no copy). Offset is clamped to the slice length.

### Text Truncation

```go
type TextLimits struct {
    Head     int `json:"head,omitempty"`      // first N lines
    Tail     int `json:"tail,omitempty"`      // last N lines
    MaxLines int `json:"max_lines,omitempty"` // max total lines
    MaxBytes int `json:"max_bytes,omitempty"` // max total bytes
}

type TruncationInfo struct {
    OriginalBytes int    `json:"original_bytes"`
    OriginalLines int    `json:"original_lines"`
    KeptBytes     int    `json:"kept_bytes"`
    KeptLines     int    `json:"kept_lines"`
    Position      string `json:"position"` // "head", "tail", or ""
}

type LimitedText struct {
    Content        string          `json:"content"`
    Truncated      bool            `json:"truncated"`
    TruncationInfo *TruncationInfo `json:"truncation_info,omitempty"`
}

func LimitText(input string, limits TextLimits) LimitedText
```

Processing order: Head/Tail, then MaxLines, then MaxBytes. Head and Tail are mutually exclusive; Head takes priority. Truncation respects UTF-8 rune boundaries and prefers line boundaries.

### Defaults

```go
type Defaults struct {
    MaxBytes int `json:"max_bytes"` // 100,000
    MaxLines int `json:"max_lines"` // 2,000
    MaxItems int `json:"max_items"` // 100
}

func StandardDefaults() Defaults

func (d Defaults) MergeTextLimits(user TextLimits) TextLimits   // fills zero MaxBytes/MaxLines
func (d Defaults) MergeArrayLimits(user ArrayLimits) ArrayLimits // fills zero Limit
```

---

## purse

Plugin manifest and mapping builders for purse-first integration.

### PluginBuilder

```go
func NewPluginBuilder(name string) *PluginBuilder

func (b *PluginBuilder) Command(cmd string, args ...string) *PluginBuilder
func (b *PluginBuilder) StdioTransport() *PluginBuilder
func (b *PluginBuilder) OnPostToolUse(action HTTPPostAction, when *NotifyCondition) *PluginBuilder
func (b *PluginBuilder) OnStop(action HTTPPostAction) *PluginBuilder
func (b *PluginBuilder) Mappings() *MappingBuilder
func (b *PluginBuilder) Build() Plugin
```

### MappingBuilder

```go
func NewMappingBuilder(server string) *MappingBuilder

func (b *MappingBuilder) Replaces(builtinTool string) *MappingEntryBuilder
func (b *MappingBuilder) Build() MappingFile
```

Built-in tool constants: `BuiltinRead`, `BuiltinEdit`, `BuiltinWrite`, `BuiltinGrep`, `BuiltinGlob`, `BuiltinBash`.

### MappingEntryBuilder

```go
func (eb *MappingEntryBuilder) ForExtensions(exts ...string) *MappingEntryBuilder
func (eb *MappingEntryBuilder) WithTool(name, useWhen string) *MappingEntryBuilder
func (eb *MappingEntryBuilder) Because(reason string) *MappingEntryBuilder
func (eb *MappingEntryBuilder) Replaces(builtinTool string) *MappingEntryBuilder // chains to parent
```

### Writers

```go
func WritePlugin(dir string, p Plugin) error      // writes {dir}/{p.Name}/plugin.json
func WriteGlobal(mf MappingFile) error             // writes $XDG_STATE_HOME/purse-first/{server}.json
func WriteProject(projectDir string, mf MappingFile) error // writes {projectDir}/.purse-first/{server}.json
```

### Plugin

```go
type Plugin struct {
    Name          string         `json:"name"`
    Type          string         `json:"type"`          // "stdio"
    Command       string         `json:"command"`
    Args          []string       `json:"args"`
    Notifications []Notification `json:"notifications,omitempty"`
    Mappings      []Mapping      `json:"mappings,omitempty"`
}
```

---

## executor

Process management abstraction for MCP servers that need to manage subprocesses.

### Executor Interface

```go
type Executor interface {
    Build(ctx context.Context, spec string) (string, error)      // resolve spec to executable path
    Execute(ctx context.Context, path string, args []string) (*Process, error)
}
```

### Process

```go
type Process struct {
    Stdin  io.WriteCloser
    Stdout io.ReadCloser
    Stderr io.ReadCloser
    Wait   func() error
    Kill   func() error
}
```

A Nix implementation is available at `executor/nix` for building Nix flake references.
