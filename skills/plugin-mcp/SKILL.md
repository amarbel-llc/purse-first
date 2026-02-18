---
name: Plugin MCP Integration
description: This skill should be used when the user asks to "add purse-first support", "turn an MCP into a package", "create a package manifest", "add generate-plugin command", "register with purse-first marketplace", "add skills to a package", "bundle skills in a package", or mentions purse-first package integration, package manifest generation, or skill bundling.
version: 0.1.0
---

# Adding Purse-First Package Support

A purse-first package ships MCP servers, skills, or both. The Nix build outputs a `share/purse-first/<name>/` directory containing a `plugin.json` manifest and optional skills, enabling purse-first to discover, aggregate, and install it as part of a Claude Code package marketplace.

## Overview

A purse-first package ships a `plugin.json` manifest at `$out/share/purse-first/<name>/plugin.json` and declares itself via a `.claude-plugin/plugin.json` in the repo (for standalone validation). Packages come in three flavors:

| Flavor | Contents | Example |
|--------|----------|---------|
| **MCP-only** | MCP server(s) + optional tool mappings | grit, get-hubbed, chix |
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

Then run `gomod2nix` to regenerate `gomod2nix.toml`.

### Step 2: Add the generate-plugin subcommand

Add a hidden `generate-plugin` subcommand that writes the package manifest. The subcommand takes a single argument: the output directory.

For **cobra-based** CLIs (like lux):

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

Use mappings to redirect Bash commands to MCP tools. The purse-first PreToolUse hook denies matching commands and suggests the corresponding MCP tool instead (e.g., `git status` redirects to `grit status`).

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
    Reason("Use the grit MCP tool instead of shelling out").
    Done()
```

For the full MappingBuilder API, detailed examples, and the `BuildMappings`/`WriteMappings` workflow, see **`references/mapping-api.md`**.

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
4. Update `flake.nix` to output `$out/share/purse-first/<name>/plugin.json` (and `mappings.json` if applicable)
5. Build and verify: `nix build && ls ./result/share/purse-first/`

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

For detailed examples from existing integrations, consult:
- **`references/existing-integrations.md`** -- Side-by-side comparison of grit, lux, get-hubbed, and chix implementations
- **`references/mapping-api.md`** -- Full MappingBuilder API reference with detailed examples
- **`examples/plugin.json`** -- Minimal MCP package manifest template
- **`examples/plugin-skill-only.json`** -- Skill-only package manifest template
- **`examples/plugin-mcp-with-skills.json`** -- MCP package with bundled skills manifest template
- **`examples/generate-plugin-cobra.go`** -- Cobra-based generate-plugin command
- **`examples/generate-plugin-flag.go`** -- Flag-based generate-plugin command
- **`examples/flake-go.nix`** -- Flake snippet for Go MCP with postInstall
- **`examples/flake-rust.nix`** -- Flake snippet for Rust MCP with static copy
