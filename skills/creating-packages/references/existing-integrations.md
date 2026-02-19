# Existing Purse-First Packages

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

Side-by-side comparison of all four MCP packages currently integrated with purse-first. Package names below are archetypes describing what each package does — use them as pattern references when building your own package.

## Summary Table

| Package | Language | Pattern | Command | Args | Extra wrapping |
|---------|----------|---------|---------|------|----------------|
| git-mcp | Go (flag) | generate | `git-mcp` | none | none |
| github-mcp | Go (flag) | generate | `github-mcp` | none | gh on PATH |
| lsp-mcp | Go (cobra) | generate | `lsp-mcp` | `mcp stdio` | none |
| nix-mcp | Rust | static | `nix-mcp` | none | fh, cachix, nil on PATH |

## git-mcp (Go, flag-based, with targeted mappings)

### plugin.json and mappings.json (both generated at build time)

```json
{
  "name": "git-mcp",
  "mcpServers": {
    "git-mcp": { "type": "stdio", "command": "git-mcp" }
  }
}
```

### main.go integration (with per-subcommand mappings)

```go
import "github.com/amarbel-llc/purse-first/purse"

func main() {
	flag.Parse()

	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		reason := "Use the git-mcp MCP tool instead of shelling out. When the command uses git -C <path>, pass that path as the repo_path parameter"

		b := purse.NewPluginBuilder("git-mcp").
			Command("git-mcp").
			StdioTransport().
			// Targeted mappings (specific subcommands first)
			Mapping("Bash").
			CommandPrefixes("git status").
			Tool("status", "checking repository status").
			Reason(reason).
			Done().
			Mapping("Bash").
			CommandPrefixes("git diff").
			Tool("diff", "viewing changes").
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
			Mapping("Bash").
			CommandPrefixes("git checkout", "git switch").
			Tool("checkout", "switching branches").
			Reason(reason).
			Done().
			// ... other subcommands ...
			// General catch-all last
			Mapping("Bash").
			CommandPrefixes("git ", "git -C ").
			Tool("status", "checking repository status").
			Tool("diff", "viewing changes").
			Tool("log", "viewing commit history").
			// ... all tools listed ...
			Reason("Use git-mcp MCP tools for git operations instead of shelling out. When the command uses git -C <path>, pass that path as the repo_path parameter").
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

	// ... MCP server setup
}
```

Key design: targeted per-subcommand mappings come first so `FindMatch` returns focused suggestions (e.g., `git log` only suggests the `log` tool). The general `"git "` / `"git -C "` catch-all at the end handles any unrecognized git subcommands with the full tool list.

### flake.nix

```nix
git-mcp = pkgs.buildGoApplication {
  pname = "git-mcp";
  inherit version;
  src = ./.;
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/git-mcp" ];

  postInstall = ''
    $out/bin/git-mcp generate-plugin $out/share/purse-first
  '';
};
```

The `postInstall` stays the same -- the binary writes both `plugin.json` and `mappings.json`.

---

## github-mcp (Go, flag-based)

### plugin.json (generated at build time)

```json
{
  "name": "github-mcp",
  "mcpServers": {
    "github-mcp": { "type": "stdio", "command": "github-mcp" }
  }
}
```

### flake.nix (in MCP server repo)

Same pattern as git-mcp:

```nix
github-mcp = pkgs.buildGoApplication {
  pname = "github-mcp";
  inherit version;
  src = ./.;
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/github-mcp" ];

  postInstall = ''
    $out/bin/github-mcp generate-plugin $out/share/purse-first
  '';
};
```

### flake.nix (in purse-first -- wrapping)

github-mcp requires `gh` on PATH at runtime, so purse-first wraps it:

```nix
github-mcp-upstream = github-mcp.packages.${system}.default;
github-mcp-pkg =
  pkgs.runCommand "github-mcp"
    { nativeBuildInputs = [ pkgs.makeWrapper ]; }
    ''
      mkdir -p $out/bin
      makeWrapper ${github-mcp-upstream}/bin/github-mcp $out/bin/github-mcp \
        --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.gh ]}

      # Propagate share directory (plugin manifest, etc.)
      if [ -d "${github-mcp-upstream}/share" ]; then
        cp -r ${github-mcp-upstream}/share $out/share
      fi
    '';
```

Key lesson: when wrapping, always propagate the `share/` directory so the package manifest survives.

---

## lsp-mcp (Go, cobra-based)

### plugin.json (generated at build time)

```json
{
  "name": "lsp-mcp",
  "mcpServers": {
    "lsp-mcp": { "type": "stdio", "command": "lsp-mcp", "args": ["mcp", "stdio"] }
  }
}
```

Note the `args` field: lsp-mcp's MCP mode is a subcommand (`lsp-mcp mcp stdio`), not the default behavior.

### main.go integration (cobra)

```go
var generatePluginCmd = &cobra.Command{
	Use:    "generate-plugin <output-dir>",
	Short:  "Generate purse-first plugin manifest",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := purse.NewPluginBuilder("lsp-mcp").
			Command("lsp-mcp", "mcp", "stdio").
			Build()

		return purse.WritePlugin(args[0], p)
	},
}
```

Register in init: `rootCmd.AddCommand(generatePluginCmd)`

### flake.nix

```nix
lsp-mcp = pkgs.buildGoApplication {
  pname = "lsp-mcp";
  inherit version;
  src = ./.;
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/lsp-mcp" ];

  postInstall = ''
    # man pages
    mkdir -p $out/share/man/man1
    $out/bin/lsp-mcp genman $out/share/man/man1

    # purse-first plugin manifest
    $out/bin/lsp-mcp generate-plugin $out/share/purse-first
  '';
};
```

Shows coexistence with other postInstall tasks (man page generation).

---

## nix-mcp (Rust, static)

### plugin.json (static file in .claude-plugin/)

```json
{
  "name": "nix-mcp",
  "mcpServers": {
    "nix-mcp": { "type": "stdio", "command": "nix-mcp" }
  },
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

Note: nix-mcp is a combined package with MCP server, hooks, and skills.

### flake.nix

Uses `runCommand` wrapping pattern because the binary needs fh, cachix, and nil on PATH:

```nix
nix-mcp =
  pkgs.runCommand "nix-mcp"
    { nativeBuildInputs = [ pkgs.makeWrapper ]; }
    ''
      mkdir -p $out/bin
      makeWrapper ${nix-mcp-unwrapped}/bin/nix-mcp $out/bin/nix-mcp \
        --prefix PATH : ${
          pkgs.lib.makeBinPath [
            fhPkg
            pkgs.cachix
            pkgs.nil
          ]
        }

      mkdir -p $out/share/purse-first/nix-mcp/hooks
      cp ${./.claude-plugin/plugin.json} $out/share/purse-first/nix-mcp/plugin.json
      install -m 755 ${formatNixHook} $out/share/purse-first/nix-mcp/hooks/format-nix
    '';
```

Key details:
- Static `plugin.json` in `.claude-plugin/`, copied during build
- Package directory name matches the `name` field and binary name
- Hooks are installed alongside the package manifest
- No Go dependency needed

---

## purse-first Marketplace Aggregation

All four packages are aggregated in purse-first's `flake.nix`:

```nix
marketplace = pkgs.symlinkJoin {
  name = "claude-plugin-marketplace";
  paths = [
    git-mcp-pkg
    github-mcp-pkg
    lsp-mcp-pkg
    nix-mcp-pkg
  ];
  nativeBuildInputs = [ pkgs.makeWrapper ];
  postBuild = ''
    makeWrapper ${purse-first-pkg}/bin/purse-first $out/bin/purse-first \
      --set PURSE_FIRST_PLUGINS_DIR "$out/share/purse-first"

    $out/bin/purse-first generate-marketplace \
      --plugins-dir "$out/share/purse-first" \
      --config ${./marketplace-config.json} \
      --output "$out/.claude-plugin/marketplace.json"
  '';
};
```

The `symlinkJoin` merges all `share/purse-first/<name>/plugin.json` files into a single tree, then `generate-marketplace` reads them to produce the final `marketplace.json`.

### marketplace-config.json entry format

```json
{
  "plugins": {
    "plugin-name": {
      "description": "Short description of what the MCP server does",
      "version": "0.1.0",
      "homepage": "https://github.com/owner/repo",
      "repo": "owner/repo",
      "category": "development",
      "tags": ["relevant", "tags", "mcp"]
    }
  }
}
```

---

## Common Gotchas

1. **gomod2nix**: After adding the `purse` dependency, always run `gomod2nix` to regenerate `gomod2nix.toml`. The Nix build uses this file, not `go.sum`.

2. **Share directory propagation**: When wrapping a binary with `makeWrapper`, the original `share/` directory is NOT automatically included. Copy or symlink it explicitly.

3. **Package name vs binary name**: The package `name` field and the directory under `share/purse-first/` must match, but the `command` field uses the actual binary name. These can differ (e.g., package name `lsp-mcp`, binary invoked with `lsp-mcp mcp stdio`).

4. **Input follows**: When adding to purse-first's flake.nix, use `inputs.nixpkgs.follows` and `inputs.nixpkgs-master.follows` to avoid duplicate nixpkgs evaluations.

5. **Hidden subcommand**: Always mark `generate-plugin` as `Hidden: true` (cobra) or omit it from usage text (flag) -- it's a build-time utility, not user-facing.

6. **Mapping order matters**: `FindMatch` returns the first matching mapping. Specific subcommand mappings (e.g., `"git log"`) must be declared before general catch-alls (e.g., `"git "`), otherwise the catch-all matches first and the targeted suggestion is never reached.

7. **Multiple prefixes per mapping**: Use multiple `CommandPrefixes` when different commands map to the same tool (e.g., `"git checkout"` and `"git switch"` both map to the `checkout` tool).

8. **Multiple tools per mapping**: Use multiple `Tool` calls when a subcommand maps to more than one MCP tool (e.g., `"git branch"` suggests both `branch_list` and `branch_create`).
