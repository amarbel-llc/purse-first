# Existing Purse-First Integrations

Side-by-side comparison of all four MCP servers currently integrated with purse-first.

## Summary Table

| Project | Language | Pattern | Command | Args | Extra wrapping |
|---------|----------|---------|---------|------|----------------|
| grit | Go (flag) | generate | `grit` | none | none |
| get-hubbed | Go (flag) | generate | `get-hubbed` | none | gh on PATH |
| lux | Go (cobra) | generate | `lux` | `mcp stdio` | none |
| chix | Rust | static | `chix` | none | fh, cachix, nil on PATH |

## grit (Go, flag-based, with targeted mappings)

**Repo:** `github:amarbel-llc/grit`

### plugin.json and mappings.json (both generated at build time)

```json
{
  "name": "grit",
  "mcpServers": {
    "grit": { "type": "stdio", "command": "grit" }
  }
}
```

### main.go integration (with per-subcommand mappings)

```go
import "github.com/amarbel-llc/purse-first/purse"

func main() {
	flag.Parse()

	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		reason := "Use the grit MCP tool instead of shelling out. When the command uses git -C <path>, pass that path as the repo_path parameter"

		b := purse.NewPluginBuilder("grit").
			Command("grit").
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
			Reason("Use grit MCP tools for git operations instead of shelling out. When the command uses git -C <path>, pass that path as the repo_path parameter").
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
grit = pkgs.buildGoApplication {
  pname = "grit";
  inherit version;
  src = ./.;
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/grit" ];

  postInstall = ''
    $out/bin/grit generate-plugin $out/share/purse-first
  '';
};
```

The `postInstall` stays the same -- the binary writes both `plugin.json` and `mappings.json`.

---

## get-hubbed (Go, flag-based)

**Repo:** `github:amarbel-llc/get-hubbed`

### plugin.json (generated at build time)

```json
{
  "name": "get-hubbed",
  "mcpServers": {
    "get-hubbed": { "type": "stdio", "command": "get-hubbed" }
  }
}
```

### flake.nix (in MCP server repo)

Same pattern as grit:

```nix
get_hubbed = pkgs.buildGoApplication {
  pname = "get-hubbed";
  inherit version;
  src = ./.;
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/get-hubbed" ];

  postInstall = ''
    $out/bin/get-hubbed generate-plugin $out/share/purse-first
  '';
};
```

### flake.nix (in purse-first -- wrapping)

get-hubbed requires `gh` on PATH at runtime, so purse-first wraps it:

```nix
get-hubbed-upstream = get-hubbed.packages.${system}.default;
get-hubbed-pkg =
  pkgs.runCommand "get-hubbed"
    { nativeBuildInputs = [ pkgs.makeWrapper ]; }
    ''
      mkdir -p $out/bin
      makeWrapper ${get-hubbed-upstream}/bin/get-hubbed $out/bin/get-hubbed \
        --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.gh ]}

      # Propagate share directory (plugin manifest, etc.)
      if [ -d "${get-hubbed-upstream}/share" ]; then
        cp -r ${get-hubbed-upstream}/share $out/share
      fi
    '';
```

Key lesson: when wrapping, always propagate the `share/` directory so the plugin manifest survives.

---

## lux (Go, cobra-based)

**Repo:** `github:friedenberg/lux`

### plugin.json (generated at build time)

```json
{
  "name": "lux",
  "mcpServers": {
    "lux": { "type": "stdio", "command": "lux", "args": ["mcp", "stdio"] }
  }
}
```

Note the `args` field: lux's MCP mode is a subcommand (`lux mcp stdio`), not the default behavior.

### main.go integration (cobra)

```go
var generatePluginCmd = &cobra.Command{
	Use:    "generate-plugin <output-dir>",
	Short:  "Generate purse-first plugin manifest",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := purse.NewPluginBuilder("lux").
			Command("lux", "mcp", "stdio").
			Build()

		return purse.WritePlugin(args[0], p)
	},
}
```

Register in init: `rootCmd.AddCommand(generatePluginCmd)`

### flake.nix

```nix
lux = pkgs.buildGoApplication {
  pname = "lux";
  inherit version;
  src = ./.;
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/lux" ];

  postInstall = ''
    # man pages
    mkdir -p $out/share/man/man1
    $out/bin/lux genman $out/share/man/man1

    # purse-first plugin manifest
    $out/bin/lux generate-plugin $out/share/purse-first
  '';
};
```

Shows coexistence with other postInstall tasks (man page generation).

---

## chix (Rust, static)

**Repo:** `github:amarbel-llc/chix`

### plugin.json (static file in .claude-plugin/)

```json
{
  "name": "chix",
  "mcpServers": {
    "chix": { "type": "stdio", "command": "chix" }
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

Note: chix is a combined plugin with MCP server, hooks, and skills.

### flake.nix

Uses `runCommand` wrapping pattern because the binary needs fh, cachix, and nil on PATH:

```nix
chix =
  pkgs.runCommand "chix"
    { nativeBuildInputs = [ pkgs.makeWrapper ]; }
    ''
      mkdir -p $out/bin
      makeWrapper ${chix-unwrapped}/bin/chix $out/bin/chix \
        --prefix PATH : ${
          pkgs.lib.makeBinPath [
            fhPkg
            pkgs.cachix
            pkgs.nil
          ]
        }

      mkdir -p $out/share/purse-first/chix/hooks
      cp ${./.claude-plugin/plugin.json} $out/share/purse-first/chix/plugin.json
      install -m 755 ${formatNixHook} $out/share/purse-first/chix/hooks/format-nix
    '';
```

Key details:
- Static `plugin.json` in `.claude-plugin/`, copied during build
- Plugin directory name matches the `name` field and binary name
- Hooks are installed alongside the plugin manifest
- No Go dependency needed

---

## purse-first Marketplace Aggregation

All four packages are aggregated in purse-first's `flake.nix`:

```nix
marketplace = pkgs.symlinkJoin {
  name = "claude-plugin-marketplace";
  paths = [
    grit-pkg
    get-hubbed-pkg
    lux-pkg
    chix-pkg
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

3. **Plugin name vs binary name**: The plugin `name` field and the directory under `share/purse-first/` must match, but the `command` field uses the actual binary name. These can differ (e.g., plugin name `lux`, binary invoked with `lux mcp stdio`).

4. **Input follows**: When adding to purse-first's flake.nix, use `inputs.nixpkgs.follows` and `inputs.nixpkgs-master.follows` to avoid duplicate nixpkgs evaluations.

5. **Hidden subcommand**: Always mark `generate-plugin` as `Hidden: true` (cobra) or omit it from usage text (flag) -- it's a build-time utility, not user-facing.

6. **Mapping order matters**: `FindMatch` returns the first matching mapping. Specific subcommand mappings (e.g., `"git log"`) must be declared before general catch-alls (e.g., `"git "`), otherwise the catch-all matches first and the targeted suggestion is never reached.

7. **Multiple prefixes per mapping**: Use multiple `CommandPrefixes` when different commands map to the same tool (e.g., `"git checkout"` and `"git switch"` both map to the `checkout` tool).

8. **Multiple tools per mapping**: Use multiple `Tool` calls when a subcommand maps to more than one MCP tool (e.g., `"git branch"` suggests both `branch_list` and `branch_create`).
