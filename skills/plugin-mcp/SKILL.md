---
name: Plugin MCP Integration
description: This skill should be used when the user asks to "add purse-first support", "turn an MCP into a plugin", "create a plugin manifest", "add generate-plugin command", "integrate with purse-first", "make an MCP server a plugin", "add plugin.json to an MCP", "register MCP with purse-first marketplace", or mentions purse-first plugin integration, MCP-to-plugin conversion, plugin manifest generation, or purse-first marketplace registration.
version: 0.1.0
---

# Adding Purse-First Plugin Support to an MCP Server

This skill guides the conversion of an MCP server into a purse-first plugin. The result is an MCP server whose Nix build outputs a `share/purse-first/<name>/plugin.json` manifest, enabling purse-first to discover, aggregate, and install it as part of a Claude Code plugin marketplace.

## Overview

A purse-first plugin is an MCP server that:

1. Ships a `plugin.json` manifest at `$out/share/purse-first/<name>/plugin.json`
2. Declares itself via a `.claude-plugin/plugin.json` in the repo (for standalone validation)
3. Uses stdio transport to communicate with Claude Code

There are two integration patterns depending on the language:

| Pattern | Language | How plugin.json is produced |
|---------|----------|-----------------------------|
| **Generate** | Go | `generate-plugin` subcommand using the `purse` package |
| **Static** | Rust / other | Static `plugin.json` file copied in `flake.nix` |

## Pattern 1: Go MCP Servers (generate-plugin command)

### Step 1: Add the purse dependency

```bash
go get github.com/amarbel-llc/purse-first/purse
```

Then run `gomod2nix` to regenerate `gomod2nix.toml`.

### Step 2: Add the generate-plugin subcommand

Add a hidden `generate-plugin` subcommand that writes the plugin manifest. The subcommand takes a single argument: the output directory.

For **cobra-based** CLIs (like lux):

```go
var generatePluginCmd = &cobra.Command{
	Use:    "generate-plugin <output-dir>",
	Short:  "Generate purse-first plugin manifest",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := purse.NewPluginBuilder("my-mcp").
			Command("my-mcp").
			StdioTransport().
			Build()

		return purse.WritePlugin(args[0], p)
	},
}
```

Register with: `rootCmd.AddCommand(generatePluginCmd)`

For **flag-based** CLIs (like grit):

```go
import "github.com/amarbel-llc/purse-first/purse"

func main() {
	flag.Parse()

	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		p := purse.NewPluginBuilder("my-mcp").
			Command("my-mcp").
			StdioTransport().
			Build()

		if err := purse.WritePlugin(flag.Arg(1), p); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}
		return
	}

	// ... rest of main
}
```

Key details for the builder:
- **Name**: Short, kebab-case identifier (becomes the key in `mcpServers`)
- **Command**: The binary name as it appears in `$out/bin/`
- **Args**: Additional arguments if the MCP mode requires a subcommand (e.g., `"mcp", "stdio"` for lux)

### Step 3: Add postInstall to flake.nix

In the Nix derivation's `postInstall`, invoke the newly built binary:

```nix
postInstall = ''
  $out/bin/my-mcp generate-plugin $out/share/purse-first
'';
```

This writes `$out/share/purse-first/my-mcp/plugin.json` at build time.

## Pattern 2: Non-Go MCP Servers (static plugin.json)

For Rust or other languages that cannot easily import the `purse` Go package.

### Step 1: Create a static plugin.json at the repo root

```json
{
  "name": "my-mcp",
  "mcpServers": {
    "my-mcp": {
      "type": "stdio",
      "command": "my-mcp-binary"
    }
  }
}
```

### Step 2: Copy it during the Nix build

In `flake.nix`, copy the static file to the share directory:

```nix
# In the build script or postInstall:
mkdir -p $out/share/purse-first/my-mcp
cp ${./plugin.json} $out/share/purse-first/my-mcp/plugin.json
```

If the binary is wrapped (e.g., with `makeWrapper` for PATH additions), place this in the wrapping `runCommand`:

```nix
my-mcp = pkgs.runCommand "my-mcp"
  { nativeBuildInputs = [ pkgs.makeWrapper ]; }
  ''
    mkdir -p $out/bin
    makeWrapper ${my-mcp-unwrapped}/bin/my-mcp $out/bin/my-mcp \
      --prefix PATH : ${pkgs.lib.makeBinPath [ extra-dep ]}

    mkdir -p $out/share/purse-first/my-mcp
    cp ${./plugin.json} $out/share/purse-first/my-mcp/plugin.json
  '';
```

## Adding Tool Mappings (Bash Command Interception)

Mappings tell the purse-first PreToolUse hook to deny Bash commands that match configured prefixes and suggest specific MCP tools instead. This is how plugins redirect `git status` to `grit status`, `nix build` to `nix__build`, etc.

### How Matching Works

`FindMatch` iterates mappings in order and returns the **first** match. This means:
- **Specific mappings must come before general ones** -- `"git log"` before `"git "`
- A general catch-all at the end handles unrecognized subcommands

### Targeted Per-Subcommand Mappings

The recommended pattern is one mapping per subcommand, each suggesting only the relevant tool(s). This gives focused denial messages instead of listing every tool in the plugin.

For **flag-based** CLIs:

```go
reason := "Use the grit MCP tool instead of shelling out. When the command uses git -C <path>, pass that path as the repo_path parameter"

b := purse.NewPluginBuilder("my-mcp").
    Command("my-mcp").
    StdioTransport().
    // Specific mappings first (matched before the catch-all)
    Mapping("Bash").
    CommandPrefixes("git status").
    Tool("status", "checking repository status").
    Reason(reason).
    Done().
    Mapping("Bash").
    CommandPrefixes("git log").
    Tool("log", "viewing commit history").
    Reason(reason).
    Done().
    Mapping("Bash").
    CommandPrefixes("git branch").
    Tool("branch_list", "listing branches").
    Tool("branch_create", "creating a new branch").
    Reason(reason).
    Done().
    // General catch-all last (for unrecognized subcommands)
    Mapping("Bash").
    CommandPrefixes("git ", "git -C ").
    Tool("status", "checking repository status").
    Tool("log", "viewing commit history").
    Tool("branch_list", "listing branches").
    Tool("branch_create", "creating a new branch").
    Reason("Use grit MCP tools for git operations instead of shelling out").
    Done()
```

Key points:
- Each `Mapping("Bash")` creates a separate `MappingEntry` in `mappings.json`
- Use `CommandPrefixes` for Bash commands, `Extensions` for file-based tools (Read, Grep, etc.)
- Multiple prefixes per mapping are supported (e.g., `git checkout` and `git switch` both map to `checkout`)
- Multiple tools per mapping are supported (e.g., `git branch` suggests both `branch_list` and `branch_create`)
- The `Reason` string is shown in the denial message along with the tool suggestions

### Writing Mappings in postInstall

When using mappings, the `generate-plugin` command must also write `mappings.json`. Update the code to call `BuildMappings` and `WriteMappings`:

```go
if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
    b := purse.NewPluginBuilder("my-mcp").
        Command("my-mcp").
        StdioTransport().
        Mapping("Bash").
        // ... mappings ...
        Done()

    p := b.Build()
    dir := flag.Arg(1)

    if err := purse.WritePlugin(dir, p); err != nil {
        log.Fatalf("generating plugin: %v", err)
    }

    if mf := b.BuildMappings(); mf != nil {
        if err := purse.WriteMappings(dir, p.Name, mf); err != nil {
            log.Fatalf("generating mappings: %v", err)
        }
    }

    return
}
```

This produces both `$out/share/purse-first/<name>/plugin.json` and `$out/share/purse-first/<name>/mappings.json`. The `postInstall` in `flake.nix` stays the same -- the binary handles both files.

### MappingBuilder API Reference

| Method | Description |
|--------|-------------|
| `Mapping(replaces)` | Start a new mapping that replaces the named tool (`"Bash"`, `"Read"`, `"Grep"`, etc.) |
| `CommandPrefixes(p...)` | Match Bash commands starting with any of these prefixes |
| `Extensions(e...)` | Match file operations on files with these extensions |
| `Tool(name, useWhen)` | Suggest this MCP tool as a replacement |
| `Reason(reason)` | Set the denial message shown to the user |
| `Done()` | Finish this mapping and return to the PluginBuilder |
| `BuildMappings()` | Returns `*MappingFile` (nil if no mappings declared) |

## Both Patterns: Repo-Level .claude-plugin/plugin.json

Regardless of the integration pattern, also create `.claude-plugin/plugin.json` in the repo for standalone plugin validation and direct Claude Code discovery:

```json
{
  "name": "my-mcp",
  "mcpServers": {
    "my-mcp": { "type": "stdio", "command": "my-mcp-binary" }
  }
}
```

This file is checked into git and validated by purse-first's BATS integration tests.

## Registering with the Purse-First Marketplace

After the MCP server outputs its plugin manifest, register it in purse-first.

### Step 1: Add as a flake input in purse-first's flake.nix

```nix
my-mcp = {
  url = "github:owner/my-mcp";
  inputs.nixpkgs.follows = "nixpkgs";
  inputs.nixpkgs-master.follows = "nixpkgs-master";
};
```

Add `my-mcp` to the `outputs` function parameters.

### Step 2: Create the package binding

```nix
my-mcp-pkg = my-mcp.packages.${system}.default;
```

If the MCP needs runtime dependencies on PATH, wrap it:

```nix
my-mcp-pkg = pkgs.runCommand "my-mcp"
  { nativeBuildInputs = [ pkgs.makeWrapper ]; }
  ''
    mkdir -p $out/bin
    makeWrapper ${my-mcp-upstream}/bin/my-mcp $out/bin/my-mcp \
      --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.some-dep ]}

    if [ -d "${my-mcp-upstream}/share" ]; then
      cp -r ${my-mcp-upstream}/share $out/share
    fi
  '';
```

### Step 3: Add to the marketplace aggregation

Add the package to the `marketplace` symlinkJoin paths:

```nix
marketplace = pkgs.symlinkJoin {
  name = "claude-plugin-marketplace";
  paths = [
    # ... existing packages
    my-mcp-pkg
  ];
  # ... rest unchanged
};
```

### Step 4: Add metadata to marketplace-config.json

```json
{
  "plugins": {
    "my-mcp": {
      "description": "Short description of what the MCP does",
      "version": "0.1.0",
      "homepage": "https://github.com/owner/my-mcp",
      "repo": "owner/my-mcp",
      "category": "development",
      "tags": ["relevant", "tags", "mcp"]
    }
  }
}
```

### Step 5: Update flake inputs and verify

```bash
just update-plugins   # or: nix flake update my-mcp
just build-all        # verify marketplace builds
just test-validate-repos  # validate plugin manifests
```

## Plugin Manifest Format

The `plugin.json` manifest follows this schema:

```json
{
  "name": "plugin-name",
  "mcpServers": {
    "server-name": {
      "type": "stdio",
      "command": "binary-name",
      "args": ["optional", "args"]
    }
  }
}
```

- **name**: Plugin identifier, kebab-case, matches the directory name in `share/purse-first/`
- **mcpServers**: Map of server name to MCP server config
- **type**: Always `"stdio"` for purse-first plugins
- **command**: The binary name (bare, not a path -- purse-first resolves it from the aggregated bin/)
- **args**: Optional arguments to pass to the binary

## Checklist

When adding purse-first support to an MCP server:

1. Create `.claude-plugin/plugin.json` in the repo
2. Add plugin manifest generation (Go: `generate-plugin` command, other: static file)
3. Add tool mappings if the plugin replaces CLI commands (use targeted per-subcommand mappings with a catch-all)
4. Update `flake.nix` to output `$out/share/purse-first/<name>/plugin.json` (and `mappings.json` if applicable)
5. Build and verify: `nix build && ls ./result/share/purse-first/`
6. Add as flake input in purse-first
7. Add metadata to `marketplace-config.json`
8. Add package to `marketplace` symlinkJoin paths
9. Run `just build-all` and `just test-validate-repos` in purse-first

## Reference Files

For detailed examples from existing integrations, consult:
- **`references/existing-integrations.md`** -- Side-by-side comparison of grit, lux, get-hubbed, and nix-mcp-server implementations
- **`examples/plugin.json`** -- Minimal plugin manifest template
- **`examples/generate-plugin-cobra.go`** -- Cobra-based generate-plugin command
- **`examples/generate-plugin-flag.go`** -- Flag-based generate-plugin command
- **`examples/flake-go.nix`** -- Flake snippet for Go MCP with postInstall
- **`examples/flake-rust.nix`** -- Flake snippet for Rust MCP with static copy
