# Purse-First Migration Design

Migrate lux from custom MCP infrastructure + go-lib-mcp to purse-first's
go-mcp library and command framework.

## Goals

- Replace `go-lib-mcp` dependency with `purse-first/libs/go-mcp`
- Define all 10 LSP tools as `command.Command` structs
- Use purse-first's `server.Server` and registries instead of custom ones
- Generate purse-first protocol artifacts (plugin.json, mappings, manpages,
  completions) at build time
- Keep Cobra for CLI dispatch (serve, add, list, status, start, stop)

## Architecture

### Before

```
cmd/lux/main.go (Cobra)
  -> internal/mcp/server.go (custom Server, message loop)
  -> internal/mcp/handler.go (custom Handler, method dispatch)
  -> internal/mcp/tools.go (custom ToolRegistry, 10 inline tool defs)
  -> internal/mcp/resources.go (custom ResourceRegistry)
  -> internal/mcp/prompts.go (custom PromptRegistry)
  -> internal/mcp/bridge.go (MCP-to-LSP adapter)
```

Dependencies: `go-lib-mcp/transport`, `go-lib-mcp/jsonrpc`, `go-lib-mcp/protocol`

### After

```
cmd/lux/main.go (Cobra + generate-plugin subcommand)
  -> internal/tools/registry.go (command.App with 10 Command defs)
  -> internal/tools/handlers.go (RunMCP handlers calling Bridge)
  -> internal/tools/bridge.go (MCP-to-LSP adapter, moved from mcp/)
  -> internal/mcp/server.go (thin wrapper around purse-first server.Server)
  -> internal/mcp/resources.go (uses purse-first ResourceRegistry)
  -> internal/mcp/prompts.go (uses purse-first PromptRegistry)
```

Dependencies: `purse-first/libs/go-mcp/command`, `purse-first/libs/go-mcp/server`,
`purse-first/libs/go-mcp/protocol`, `purse-first/libs/go-mcp/transport`

### Deleted

- `internal/mcp/handler.go` — purse-first server handles dispatch
- `internal/mcp/tools.go` — replaced by `internal/tools/`

## Tool Definitions

All 10 tools become `command.Command` structs with typed `Param` declarations
instead of inline JSON schemas. Example:

```go
app.AddCommand(&command.Command{
    Name: "hover",
    Description: command.Description{
        Short: "Get type information, documentation, and signatures for a symbol.",
        Long:  "...",
    },
    Params: []command.Param{
        {Name: "uri", Type: command.String, Required: true,
         Description: "File URI (e.g., file:///path/to/file.go)"},
        {Name: "line", Type: command.Int, Required: true,
         Description: "0-indexed line number"},
        {Name: "character", Type: command.Int, Required: true,
         Description: "0-indexed character offset"},
    },
    RunMCP: func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
        // Same logic as current handleHover
    },
})
```

## MCP Server Construction

The current `mcp.New()` creates a custom server with manual message routing.
The new version uses purse-first's server:

```go
func New(cfg *config.Config, t transport.Transport) (*Server, error) {
    pool := subprocess.NewPool(cfg)
    router := server.NewRouter(cfg)
    bridge := tools.NewBridge(pool, router)

    app := tools.RegisterAll(bridge)

    toolRegistry := mcpserver.NewToolRegistry()
    app.RegisterMCPTools(toolRegistry)

    resourceRegistry := mcpserver.NewResourceRegistry()
    registerResources(resourceRegistry, cfg, pool, router)

    promptRegistry := mcpserver.NewPromptRegistry()
    registerPrompts(promptRegistry)

    srv, err := mcpserver.New(t, mcpserver.Options{
        ServerName:    "lux",
        ServerVersion: "0.1.0",
        Tools:         toolRegistry,
        Resources:     resourceRegistry,
        Prompts:       promptRegistry,
    })

    return &Server{inner: srv, pool: pool}, err
}
```

## Custom Transports

Lux's SSE and HTTP transports (`internal/transport/`) stay unchanged. They
implement `transport.Transport` which is the same interface in both libraries.

## Generate-Plugin Support

Add hidden Cobra subcommand:

```go
var generatePluginCmd = &cobra.Command{
    Use:    "generate-plugin <output-dir>",
    Hidden: true,
    Args:   cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        app := tools.RegisterAll(nil)
        return app.GenerateAll(args[0])
    },
}
```

Update `flake.nix` postInstall:

```nix
postInstall = ''
  $out/bin/lux generate-plugin $out
'';
```

Generated artifacts:

- `share/purse-first/lux/plugin.json`
- `share/purse-first/lux/mappings.json`
- `share/man/man1/lux*.1`
- `share/bash-completion/completions/lux`
- `share/zsh/site-functions/_lux`
- `share/fish/vendor_completions.d/lux.fish`

## File Changes

| File | Action |
|------|--------|
| `go.mod` | Replace `go-lib-mcp` with `purse-first/libs/go-mcp` |
| `internal/tools/registry.go` | New: command.App with 10 tool definitions |
| `internal/tools/handlers.go` | New: RunMCP handlers calling Bridge |
| `internal/tools/bridge.go` | Move from `internal/mcp/bridge.go` |
| `internal/tools/result.go` | New: helper for JSON result formatting |
| `internal/mcp/server.go` | Rewrite: thin wrapper around purse-first server |
| `internal/mcp/resources.go` | Rewrite: use purse-first ResourceRegistry |
| `internal/mcp/prompts.go` | Rewrite: use purse-first PromptRegistry |
| `internal/mcp/handler.go` | Delete |
| `internal/mcp/tools.go` | Delete |
| `cmd/lux/main.go` | Add generate-plugin command, update MCP construction |
| `flake.nix` | Add postInstall for generate-plugin |

## Unchanged

- `internal/config/` — config loading
- `internal/server/` — LSP router and handler
- `internal/subprocess/` — LSP process pool
- `internal/control/` — Unix socket control
- `internal/transport/` — custom SSE/HTTP transports
- `internal/capabilities/` — LSP capability bootstrapping
- Cobra CLI commands (serve, add, list, status, start, stop, mcp *)
