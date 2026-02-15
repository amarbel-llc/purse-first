# Existing Purse-First Integrations

Side-by-side comparison of all four MCP servers currently integrated with purse-first.

## Summary Table

| Project | Language | Pattern | Command | Args | Extra wrapping |
|---------|----------|---------|---------|------|----------------|
| grit | Go (flag) | generate | `grit` | none | none |
| get-hubbed | Go (flag) | generate | `get-hubbed` | none | gh on PATH |
| lux | Go (cobra) | generate | `lux` | `mcp stdio` | none |
| nix-mcp-server | Rust | static | `nix-mcp-server` | none | fh, cachix, nil on PATH |

## grit (Go, flag-based)

**Repo:** `github:amarbel-llc/grit`

### plugin.json (generated at build time)

```json
{
  "name": "grit",
  "mcpServers": {
    "grit": { "type": "stdio", "command": "grit" }
  }
}
```

### main.go integration

```go
import "github.com/amarbel-llc/purse-first/purse"

func main() {
	flag.Parse()

	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		p := purse.NewPluginBuilder("grit").
			Command("grit").
			StdioTransport().
			Build()

		if err := purse.WritePlugin(flag.Arg(1), p); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}
		return
	}

	// ... MCP server setup
}
```

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

Simplest integration: no wrapping, no extra args, no runtime dependencies.

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

## nix-mcp-server (Rust, static)

**Repo:** `github:friedenberg/nix-mcp-server`

### plugin.json (static file at repo root)

```json
{
  "name": "nix",
  "mcpServers": {
    "nix": { "type": "stdio", "command": "nix-mcp-server" }
  }
}
```

Note: plugin name is `nix` (short), but binary is `nix-mcp-server`.

### flake.nix

Uses `runCommand` wrapping pattern because the binary needs fh, cachix, and nil on PATH:

```nix
nix-mcp-server =
  pkgs.runCommand "nix-mcp-server"
    { nativeBuildInputs = [ pkgs.makeWrapper ]; }
    ''
      mkdir -p $out/bin
      makeWrapper ${nix-mcp-server-unwrapped}/bin/nix-mcp-server $out/bin/nix-mcp-server \
        --prefix PATH : ${
          pkgs.lib.makeBinPath [
            fhPkg
            pkgs.cachix
            pkgs.nil
          ]
        }

      mkdir -p $out/share/purse-first/nix
      cp ${./plugin.json} $out/share/purse-first/nix/plugin.json
    '';
```

Key details:
- Static `plugin.json` at repo root, copied during build
- Plugin directory name (`nix`) matches the `name` field, not the binary name
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
    nix-mcp-server-pkg
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

3. **Plugin name vs binary name**: The plugin `name` field and the directory under `share/purse-first/` must match, but the `command` field uses the actual binary name. These can differ (e.g., plugin name `nix`, binary `nix-mcp-server`).

4. **Input follows**: When adding to purse-first's flake.nix, use `inputs.nixpkgs.follows` and `inputs.nixpkgs-master.follows` to avoid duplicate nixpkgs evaluations.

5. **Hidden subcommand**: Always mark `generate-plugin` as `Hidden: true` (cobra) or omit it from usage text (flag) -- it's a build-time utility, not user-facing.
