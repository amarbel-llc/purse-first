---
name: Creating Packages
description: This skill should be used when the user asks to "add purse-first support", "turn an MCP into a package", "create a package manifest", "add generate-plugin command", "register with purse-first marketplace", "add skills to a package", "bundle skills in a package", "make this available as a Claude Code tool", "distribute this MCP server", "share this CLI with Claude", "set up plugin.json", "ship this as a package", or wants to package an MCP server, CLI, or skill set for distribution via purse-first. Also applies when working in a repo that has a `.claude-plugin/` directory, editing a `plugin.json` or `mappings.json`, or modifying a `flake.nix` that uses `mkMarketplace` or references purse-first as a flake input.
version: 0.1.0
---

# Creating Purse-First Packages

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

If you're building an MCP server, CLI tool, or skill set and want Claude Code users to discover and use it without manual configuration, package it with purse-first. The framework handles discovery, installation, and tool routing automatically.

For a high-level understanding of the framework, see the **bob:overview** skill.
For understanding how installed packages behave at runtime, see the **bob:using-packages** skill.
For adding output-limiting to MCP tools, see the **bob:context-saving** skill.
For building Go MCP servers and CLIs, see **go-mcp-command(7)**.

## Overview

A purse-first package ships a `plugin.json` manifest at `$out/share/purse-first/<name>/plugin.json` and declares itself via a `.claude-plugin/plugin.json` in the repo (for standalone validation). Packages come in three flavors:

| Flavor | Contents | Example |
|--------|----------|---------|
| **MCP-only** | MCP server(s) + optional tool mappings | git-mcp, github-mcp, nix-mcp |
| **Skill-only** | Skills only (no MCP server) | bob |
| **MCP + Skills** | MCP server(s) + bundled skills | (future) |

For MCP-containing packages, there are two patterns depending on language:

| Pattern | Language | How plugin.json is produced |
|---------|----------|-----------------------------|
| **Generate** | Go | `generate-plugin` subcommand using the `purse` package |
| **Static** | Rust / other | Static `plugin.json` file copied in `flake.nix` |

## Pattern 1: Go MCP Servers (generate-plugin command)

### Step 1: Add the purse dependency

```bash
go get github.com/amarbel-llc/purse-first/purse
```

### Step 2: Add the generate-plugin subcommand

Add a hidden `generate-plugin` subcommand that writes the package manifest. The subcommand takes a single argument: the output directory.

For **cobra-based** CLIs (like lsp-mcp):

```go
var generatePluginCmd = &cobra.Command{
	Use:    "generate-plugin <output-dir>",
	Short:  "Generate purse-first package manifest",
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

For **flag-based** CLIs (like git-mcp):

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
			log.Fatalf("generating package: %v", err)
		}
		return
	}

	// ... rest of main
}
```

Key details for the builder:
- **Name**: Short, kebab-case identifier (becomes the key in `mcpServers`)
- **Command**: The binary name as it appears in `$out/bin/`
- **Args**: Additional arguments if the MCP mode requires a subcommand (e.g., `"mcp", "stdio"` for lsp-mcp)

### Step 3: Add postInstall to flake.nix

In the Nix derivation's `postInstall`, invoke the newly built binary:

```nix
postInstall = ''
  $out/bin/my-mcp generate-plugin $out/share/purse-first
'';
```

This writes `$out/share/purse-first/my-mcp/plugin.json` at build time.

### Step 4: Add install-mcp subcommand (optional)

For standalone dev testing outside the marketplace, add an `install-mcp` subcommand that registers the binary as an MCP server in `~/.claude.json`. The `command.App` abstraction provides this out of the box:

```go
app.AddCommand(&command.Command{
    Name:        "install-mcp",
    Hidden:      true,
    Description: command.Description{Short: "Register as MCP server in ~/.claude.json"},
    RunCLI: func(ctx context.Context, args json.RawMessage) error {
        return app.InstallMCP()
    },
})
```

Then run `my-mcp install-mcp` to register the dev binary without a full marketplace install. This resolves the running binary's path, so it works with both `go run` and Nix-built binaries.

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

Use mappings to redirect Bash commands to MCP tools. Per-package PreToolUse hooks deny matching commands and suggest the corresponding MCP tool instead (e.g., `git status` redirects to the `grit status` tool).

Key rules:
- Declare one mapping per subcommand for focused denial messages
- Order specific mappings before general catch-alls (`"git log"` before `"git "`)
- Use `CommandPrefixes` for Bash commands, `Extensions` for file-based tools (Read, Grep, etc.)

Minimal example:

```go
b := purse.NewPluginBuilder("my-mcp").
    Command("my-mcp").
    StdioTransport().
    Mapping("Bash").
    CommandPrefixes("git status").
    Tool("status", "checking repository status").
    Reason("Use the git-mcp MCP tool instead of shelling out").
    Done()
```

For the full MappingBuilder API, detailed examples, and the `BuildMappings`/`WriteMappings` workflow, see **`references/mapping-api.md`**.

## Per-Package Hooks (Tool Routing)

Packages that declare tool mappings automatically get per-package PreToolUse hooks via `GenerateAll` / `GenerateHooks`. There is no central hook infrastructure — each package handles its own tool routing independently.

### How It Works

When a package has `MapsTools` declarations on any command, `GenerateAll` produces:

```
$out/share/purse-first/<name>/hooks/
├── hooks.json      # PreToolUse matcher pointing to the wrapper script
└── pre-tool-use    # Shell script that calls `<binary> hook`
```

At runtime, Claude Code fires the PreToolUse hook, which calls the package binary's `hook` subcommand. The binary reads the hook input from stdin, matches against its registered `ToolMapping` declarations, and denies the built-in tool when an MCP tool should be used instead.

### Wiring the Hook Subcommand

Each package binary needs a `hook` subcommand. For Go packages using `command.App`, add:

**Flag-based CLI** (grit, get-hubbed pattern):

```go
if flag.NArg() >= 1 && flag.Arg(0) == "hook" {
    if err := app.HandleHook(os.Stdin, os.Stdout); err != nil {
        log.Fatalf("handling hook: %v", err)
    }
    return
}
```

**command.App CLI** (lux pattern):

```go
app.AddCommand(&command.Command{
    Name:   "hook",
    Hidden: true,
    Description: command.Description{Short: "Handle PreToolUse hook"},
    RunCLI: func(ctx context.Context, args json.RawMessage) error {
        tools.RegisterAll(app, nil)
        return app.HandleHook(os.Stdin, os.Stdout)
    },
})
```

The hook subcommand should be hidden — it's called by the generated wrapper script, not by users.

For full API details on `HandleHook` and `GenerateHooks`, see **go-mcp-hooks(7)**.

## Both Patterns: Repo-Level .claude-plugin/plugin.json

Regardless of the integration pattern, also create `.claude-plugin/plugin.json` in the repo for standalone package validation and direct Claude Code discovery:

```json
{
  "name": "my-mcp",
  "mcpServers": {
    "my-mcp": { "type": "stdio", "command": "my-mcp-binary" }
  }
}
```

Check this file into git for standalone validation. Purse-first's BATS integration tests validate it.

## Registering with the Purse-First Marketplace

After the MCP server outputs its package manifest, register it in purse-first.

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
just test-validate-repos  # validate package manifests
purse-first validate-mcp ./result/bin/<binary>  # verify MCP server responds correctly
```

## Package Manifest Format

The `plugin.json` manifest follows this schema:

```json
{
  "name": "my-mcp",
  "mcpServers": {
    "server-name": {
      "type": "stdio",
      "command": "binary-name",
      "args": ["optional", "args"]
    }
  }
}
```

- **name**: Package identifier, kebab-case, matches the directory name in `share/purse-first/`
- **mcpServers**: Map of server name to MCP server config
- **type**: Always `"stdio"` for purse-first packages
- **command**: The binary name (bare, not a path -- purse-first resolves it from the aggregated bin/)
- **args**: Optional arguments to pass to the binary

## Adding Skills to a Package

Skills are SKILL.md files (with YAML frontmatter) that teach Claude Code domain-specific workflows. Any purse-first package — MCP-containing or not — can ship skills.

### Directory Layout

```
$out/share/purse-first/<package-name>/
├── plugin.json
└── skills/
    └── <skill-name>/
        ├── SKILL.md           # required per skill
        ├── references/        # optional detailed docs
        └── examples/          # optional working examples
```

### Skill-Only Packages

For packages that ship only skills (no MCP server), the `plugin.json` declares skills but omits `mcpServers`:

```json
{
  "name": "my-package",
  "skills": [
    "./skills/my-skill"
  ]
}
```

The `skills` array uses relative paths from the package root directory to each skill directory.

### MCP Packages with Bundled Skills

An MCP server can also bundle skills alongside its server declaration:

```json
{
  "name": "my-mcp",
  "mcpServers": {
    "my-mcp": { "type": "stdio", "command": "my-mcp" }
  },
  "skills": [
    "./skills/usage-patterns"
  ]
}
```

### Shipping Skills in Nix Builds

Copy the skills directory into the package's share output and use `generate-local-plugin` to discover them and update the manifest.

For **skill-only packages** (bob pattern):

```nix
postInstall = ''
  mkdir -p $out/share/purse-first/my-package/skills
  cp -r ${./skills}/* $out/share/purse-first/my-package/skills/

  staging=$(mktemp -d)
  ln -s $out/share/purse-first/my-package/skills $staging/skills
  mkdir -p $staging/.claude-plugin
  cp ${./.claude-plugin/plugin.json} $staging/.claude-plugin/plugin.json
  chmod u+w $staging/.claude-plugin/plugin.json
  $out/bin/purse-first generate-local-plugin --root $staging
  cp $staging/.claude-plugin/plugin.json $out/share/purse-first/my-package/plugin.json
'';
```

For **MCP servers adding skills** (extend existing postInstall):

```nix
postInstall = ''
  # Generate package manifest (existing)
  $out/bin/my-mcp generate-plugin $out/share/purse-first

  # Copy skills into the package directory
  mkdir -p $out/share/purse-first/my-mcp/skills
  cp -r ${./skills}/* $out/share/purse-first/my-mcp/skills/
'';
```

When the MCP already uses `generate-plugin`, add the skills array to the static `.claude-plugin/plugin.json` or update the Go code to emit it. The marketplace discovery step (`generate-marketplace`) will find `skills/*/SKILL.md` automatically and include them in the aggregated output.

### Skill Discovery

Purse-first discovers skills by globbing `skills/*/SKILL.md` under each package directory. Skills are included in the marketplace manifest automatically — no explicit registration is needed beyond placing them in the correct directory.

### Writing Effective SKILL.md Files

Each `SKILL.md` requires YAML frontmatter with `name` and `description`:

```yaml
---
name: My Skill Name
description: This skill should be used when the user asks to "do X", "configure Y", or mentions Z.
---
```

Key guidelines:
- Use third-person description with specific trigger phrases
- Write the body in imperative form (verb-first instructions)
- Keep SKILL.md lean (1,500-2,000 words) — move detailed content to `references/`
- Reference supporting files explicitly so Claude knows they exist

## Checklist

When adding purse-first support:

### MCP Package
1. Create `.claude-plugin/plugin.json` in the repo
2. Add package manifest generation (Go: `generate-plugin` command, other: static file)
3. Add tool mappings if the package replaces CLI commands (use targeted per-subcommand mappings with a catch-all)
4. Wire the `hook` subcommand if using tool mappings (calls `app.HandleHook`)
5. Update `flake.nix` to output `$out/share/purse-first/<name>/plugin.json` (and `mappings.json`, `hooks/` if applicable)
6. Build and verify: `nix build && ls ./result/share/purse-first/`
7. Validate MCP server: `purse-first validate-mcp ./result/bin/<binary>` (checks initialize handshake, tools/list, resources/list, annotations)

### Skills (if applicable)
6. Create `skills/<skill-name>/SKILL.md` with YAML frontmatter
7. Add optional `references/` and `examples/` directories for supporting content
8. Update `flake.nix` postInstall to copy skills to `$out/share/purse-first/<name>/skills/`
9. Add `"skills"` array to `.claude-plugin/plugin.json` (or use `generate-local-plugin` to discover them)
10. Verify skills are included: `nix build && ls ./result/share/purse-first/<name>/skills/`

### Marketplace Registration
11. Add as flake input in purse-first
12. Add metadata to `marketplace-config.json`
13. Add package to `marketplace` symlinkJoin paths
14. Run `just build-all` and `just test-validate-repos` in purse-first

## Reference Files

Consult these when you need detailed implementation examples:

- **`references/existing-integrations.md`** — Read this when comparing approaches across languages. Shows side-by-side git-mcp (Go/flag), lsp-mcp (Go/cobra), github-mcp (Go/flag), and nix-mcp (Rust) implementations.
- **`references/mapping-api.md`** — Read this when adding tool mappings. Full MappingBuilder API reference with per-subcommand and catch-all mapping examples.
- **`examples/plugin.json`** — Minimal MCP package manifest template
- **`examples/plugin-skill-only.json`** — Skill-only package manifest template
- **`examples/plugin-mcp-with-skills.json`** — MCP package with bundled skills manifest template
- **`examples/generate-plugin-cobra.go`** — Cobra-based generate-plugin command
- **`examples/generate-plugin-flag.go`** — Flag-based generate-plugin command
- **`examples/flake-go.nix`** — Flake snippet for Go MCP with postInstall
- **`examples/flake-rust.nix`** — Flake snippet for Rust MCP with static copy

## Related Skills

- **bob:overview** — Framework orientation, terminology, and workflow overview
- **bob:using-packages** — How installed packages work at runtime, troubleshooting
- **bob:context-saving** — Adding output-limiting to MCP tools (pagination, truncation)
- **go-mcp(7)** — Building Go CLIs and MCP servers with go-mcp
