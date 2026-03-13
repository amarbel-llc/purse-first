---
status: exploring
date: 2026-03-13
promotion-criteria: go-mcp exposes InstallMCP and at least two downstream packages
  ship install-mcp commands using it
---

# Self-Install MCP

## Problem Statement

MCP servers built with go-mcp can be distributed through the purse-first
marketplace, but marketplace registration requires coordination: adding flake
inputs, rebuilding the marketplace, and restarting Claude Code. Users who build
or install an MCP server directly (via `nix build`, `go install`, or a release
binary) have no way to register it with Claude Code without manually editing
`~/.claude/mcp.json`.

A standalone `install-mcp` command on each MCP binary would let users go from
build to working MCP in one step, without marketplace involvement.

## Interface

go-mcp's `command` package provides an `InstallMCP` function that downstream
packages wire into a hidden `install-mcp` CLI command. The function:

1. Resolves the running binary's absolute path via `os.Executable` +
   `filepath.EvalSymlinks`.
2. Reads the existing `~/.claude/mcp.json` (if any) and unmarshals it.
3. Merges an entry for the current app into the `mcpServers` map, using the
   app's name as the key, the resolved binary path as `command`, and
   `app.MCPArgs` as `args`.
4. Writes the updated file back.

The downstream wiring is a single command registration:

```go
app.AddCommand(&command.Command{
    Name: "install-mcp",
    Description: command.Description{
        Short: "Install as a Claude Code MCP server",
    },
    RunCLI: func(ctx context.Context, args json.RawMessage) error {
        return app.InstallMCP()
    },
})
```

The library owns the MCP config format, path resolution, and merge logic.
Downstream packages only call `app.InstallMCP()`.

## Examples

Build and install in one step:

    nix build ./go && ./go/result/bin/chrest install-mcp
    # installed chrest MCP server to /home/user/.claude/mcp.json

Resulting `~/.claude/mcp.json`:

```json
{
  "mcpServers": {
    "chrest": {
      "type": "stdio",
      "command": "/nix/store/...-chrest-0.0.1/bin/chrest",
      "args": ["mcp"]
    }
  }
}
```

Running again after rebuilding updates the binary path in place without
disturbing other MCP entries in the file.

## Limitations

- **Nix store paths are volatile.** If the store path is garbage-collected,
  Claude Code will fail to start the MCP. Users must re-run `install-mcp` after
  `nix-store --gc`. A future enhancement could create a stable symlink
  (similar to chrest's `init` command) instead of pointing at the store path
  directly.
- **User scope only.** Writes to `~/.claude/mcp.json` (user-global). Does not
  support project-scoped `.mcp.json` installation. A `--scope project` flag
  could be added later.
- **No uninstall.** There is no `uninstall-mcp` counterpart. Removal requires
  manual editing of `mcp.json`.

## More Information

- Prototype implementation: `chrest` `go/cmd/chrest/install_mcp.go`
- Target library location: `libs/go-mcp/command/install_mcp.go`
- Claude Code MCP config format: `~/.claude/mcp.json` with `mcpServers` map
