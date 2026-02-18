# Dev MCP: Unified Local Plugin Testing

**Status:** Approved
**Authors:** friedenberg

## Problem

When developing a purse-first MCP plugin (lux, grit, chix), testing local
changes against a live Claude Code session requires manually managing three
separate concerns:

1. **MCP server config** — `.mcp.json` pointing to the nix store path from the
   local build.
2. **Tool interception mappings** — `.purse-first/*.json` files that tell hooks
   which Claude Code tools to redirect to MCP alternatives.
3. **Hook registration** — `.claude/settings.json` entries that fire
   purse-first's PreToolUse/PostToolUse handlers.

Each `nix build` produces a new store path, requiring all three to be updated.
There is no automation and no way for plugins to declare non-Bash interception
rules (e.g. redirecting `Read` on `.go` files to `lux hover`).

## Design

### 1. `MapsTools` replaces `MapsBash`

The `Command` struct gets a unified `MapsTools` field that generalizes
`MapsBash` to support all Claude Code tool types:

```go
type ToolMapping struct {
    Replaces        string   // "Read", "Grep", "Glob", "Bash"
    Extensions      []string // file extensions, e.g. [".go", ".py"]
    CommandPrefixes []string // bash command prefixes, e.g. ["git status"]
    UseWhen         string   // shown to Claude in denial reason
}

type Command struct {
    // ...existing fields...
    MapsTools []ToolMapping  // replaces MapsBash
}
```

`MapsBash` is removed. Migration for grit: each `BashMapping{Prefixes, UseWhen}`
becomes `ToolMapping{Replaces: "Bash", CommandPrefixes, UseWhen}`.

For lux, tools declare interception rules like:

```go
MapsTools: []command.ToolMapping{
    {Replaces: "Read", Extensions: []string{".go", ".py"},
     UseWhen: "getting type info"},
}
```

### 2. `GenerateMappings()` extended

The `mappingEntry` struct gains an `Extensions` field:

```go
type mappingEntry struct {
    Replaces        string                  `json:"replaces"`
    Extensions      []string                `json:"extensions,omitempty"`
    CommandPrefixes []string                `json:"command_prefixes,omitempty"`
    Tools           []mappingToolSuggestion `json:"tools"`
    Reason          string                  `json:"reason"`
}
```

`GenerateMappings()` iterates `cmd.MapsTools` instead of `cmd.MapsBash`,
populating both `Extensions` and `CommandPrefixes` as appropriate. Multiple
tools on the same command that share the same `(Replaces, Extensions,
CommandPrefixes)` key are consolidated into a single mapping entry with multiple
tool suggestions.

Example output for lux:

```json
{
  "server": "lux",
  "mappings": [
    {
      "replaces": "Read",
      "extensions": [".go", ".py"],
      "tools": [
        {"name": "hover", "use_when": "getting type info"},
        {"name": "document_symbols", "use_when": "understanding file structure"}
      ],
      "reason": "Use the lux MCP tool instead"
    }
  ]
}
```

### 3. `tool_prefix` in mapping files

The `MappingFile` type gains an optional `tool_prefix` field:

```go
type MappingFile struct {
    Server     string    `json:"server"`
    ToolPrefix string    `json:"tool_prefix,omitempty"`
    Mappings   []Mapping `json:"mappings"`
}
```

The hook handler's `formatDenyReason` uses `tool_prefix` when present, falling
back to the marketplace convention `mcp__plugin_{server}_{server}`:

```go
prefix := match.ToolPrefix
if prefix == "" {
    prefix = fmt.Sprintf("mcp__plugin_%s_%s", match.Server, match.Server)
}
// tool name: {prefix}__{tool_name}
```

Marketplace mappings omit `tool_prefix` and use the default. Dev mappings set it
to `mcp__{name}-{suffix}` to match the `.mcp.json` server key.

### 4. `dev-mcp` as a library-provided command

`command.App` auto-registers a hidden `dev-mcp` command. Every plugin gets it
for free.

```
lux dev-mcp [--suffix=dev] [--clean]
grit dev-mcp [--suffix=local] [--clean]
```

The command:

1. **Resolves the build output** — walks up from the running binary's path to
   find the nix store output root. Since `./result/bin/lux dev-mcp` runs a
   binary inside a nix store symlink, the output root is two levels up from
   the binary.

2. **Reads plugin artifacts** from the resolved output:
   - `share/purse-first/<name>/plugin.json` — name, command path, args
   - `share/purse-first/<name>/mappings.json` — tool interception rules

3. **Generates three project-local files:**

   **`.mcp.json`** — MCP server with dev-suffixed name:
   ```json
   {
     "mcpServers": {
       "lux-dev": {
         "type": "stdio",
         "command": "/nix/store/...-lux-0.1.0/bin/lux",
         "args": ["mcp", "stdio"]
       }
     }
   }
   ```

   **`.purse-first/<name>.json`** — mappings with `tool_prefix` set:
   ```json
   {
     "server": "lux",
     "tool_prefix": "mcp__lux-dev",
     "mappings": [...]
   }
   ```

   **`.claude/settings.json`** — project-scoped hooks pointing to the binary:
   ```json
   {
     "hooks": {
       "PreToolUse": [{
         "matcher": "Read|Edit|Write|Grep|Glob|Bash",
         "hooks": [{
           "type": "command",
           "command": "/nix/store/...-lux-0.1.0/bin/purse-first hook",
           "timeout": 5,
           "blocking": true
         }]
       }],
       "PostToolUse": [{
         "matcher": "Read|Edit|Write|Grep|Glob|Bash",
         "hooks": [{
           "type": "command",
           "command": "/nix/store/...-lux-0.1.0/bin/purse-first post-hook",
           "timeout": 5,
           "blocking": false
         }]
       }]
     }
   }
   ```

   Note: the hooks point to the purse-first binary. Since the plugin binary
   doesn't include purse-first CLI, `dev-mcp` resolves the purse-first binary
   from the same mechanism `generate-plugin` uses — the purse-first library
   knows where its own binary is at build time and can embed the path, or
   `dev-mcp` can accept a `--purse-first-binary` flag with a sensible default.

4. **`--clean`** removes `.mcp.json`, `.purse-first/`, and project-scoped
   `.claude/settings.json` hooks.

### 5. Workflow

```sh
# Make code changes
vim internal/tools/something.go

# Build and generate dev config
nix build && ./result/bin/lux dev-mcp

# Restart Claude Code to pick up changes

# Iterate: change → nix build → dev-mcp → restart

# Clean up when done
./result/bin/lux dev-mcp --clean
```

### 6. `.gitignore`

All generated files contain machine-specific nix store paths:

```gitignore
.mcp.json
.purse-first/
.claude/settings.json
```

## Migration

- `MapsBash` removed from `command.Command`.
- Grit: each `BashMapping` becomes a `ToolMapping` with `Replaces: "Bash"`.
- Lux: add `MapsTools` declarations to existing tool registrations.
- `generate_mappings.go`: iterate `MapsTools` instead of `MapsBash`, populate
  `Extensions` field, consolidate entries by key.
- `mapping/types.go`: add `ToolPrefix` field to `MappingFile`.
- `hook/handler.go`: `formatDenyReason` uses `ToolPrefix` with fallback.
- `mapping/loader.go`: `Match` struct gains `ToolPrefix` from its `MappingFile`.
- Tests: update existing `MapsBash` references, add tests for `Extensions`
  matching and `ToolPrefix` formatting.

## Considerations

- The purse-first binary is not in the plugin's nix closure. `dev-mcp` needs
  a strategy for locating it — either embedded at library compile time, a
  `--purse-first-binary` flag, or requiring purse-first to be available on
  `$PATH`.
- Skills bundled with the plugin are not loaded via `.mcp.json`. Skill testing
  requires a separate mechanism.
- The `./result` symlink is stable across rebuilds in the same directory,
  but `.mcp.json` uses the resolved nix store path for reliability.
