# Existing Purse-First Packages

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

Side-by-side comparison of all MCP packages currently in purse-first. Use them as pattern references when building your own package.

## Summary Table

| Package | Language | Pattern | Command | Hooks | Extra wrapping |
|---------|----------|---------|---------|-------|----------------|
| grit | Go (flag) | generate (workspace) | `grit` | PreToolUse (per-package) | none |
| get-hubbed | Go (flag) | generate (workspace) | `get-hubbed` | PreToolUse (per-package) | gh on PATH |
| lux | Go (command.App) | generate (workspace) | `lux` | PreToolUse (per-package) | none |
| chix | Rust | static | `chix` | PostToolUse (nix fmt) | fh, cachix, nil on PATH |

## grit (Go, flag-based, with targeted mappings + per-package hooks)

### plugin.json, mappings.json, and hooks (all generated at build time)

```json
{
  "name": "grit",
  "mcpServers": {
    "grit": { "type": "stdio", "command": "grit" }
  }
}
```

### main.go integration (with per-subcommand mappings + hook subcommand)

```go
func main() {
	flag.Parse()

	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		// ... build app with tool mappings ...
		app.GenerateAll(flag.Arg(1))
		return
	}

	// Per-package hook handler
	if flag.NArg() >= 1 && flag.Arg(0) == "hook" {
		if err := app.HandleHook(os.Stdin, os.Stdout); err != nil {
			log.Fatalf("handling hook: %v", err)
		}
		return
	}

	// ... MCP server setup
}
```

Key design: `GenerateAll` writes plugin.json, mappings.json, hooks/hooks.json, and hooks/pre-tool-use. The `hook` subcommand handles PreToolUse invocations at runtime.

### flake.nix (workspace build)

```nix
# Workspace build: uses the full Go monorepo source with `go work vendor`.
{ pkgs, goWorkspaceSrc, goVendorHash }:

pkgs.buildGoModule {
  pname = "grit";
  version = "0.1.0";
  src = goWorkspaceSrc;
  vendorHash = goVendorHash;
  GOWORK = "";
  overrideModAttrs = _: _: {
    GOWORK = "";
    buildPhase = ''
      runHook preBuild
      go work vendor -e
      runHook postBuild
    '';
  };
  subPackages = [ "packages/grit/cmd/grit" ];
  postInstall = ''
    $out/bin/grit generate-plugin $out
  '';
}
```

All Go packages share the same `goWorkspaceSrc` and `goVendorHash`. The vendor hash only covers external dependencies — local code changes never invalidate it.

---

## get-hubbed (Go, flag-based, with wrapping)

### plugin.json (generated at build time)

```json
{
  "name": "get-hubbed",
  "mcpServers": {
    "get-hubbed": { "type": "stdio", "command": "get-hubbed" }
  }
}
```

### flake.nix (workspace build)

Same workspace pattern as grit:

```nix
{ pkgs, goWorkspaceSrc, goVendorHash }:

pkgs.buildGoModule {
  pname = "get-hubbed";
  version = "0.1.0";
  src = goWorkspaceSrc;
  vendorHash = goVendorHash;
  GOWORK = "";
  overrideModAttrs = _: _: {
    GOWORK = "";
    buildPhase = ''
      runHook preBuild
      go work vendor -e
      runHook postBuild
    '';
  };
  subPackages = [ "packages/get-hubbed/cmd/get-hubbed" ];
  postInstall = ''
    $out/bin/get-hubbed generate-plugin $out
  '';
}
```

### flake.nix (in purse-first -- wrapping)

get-hubbed requires `gh` on PATH at runtime, so purse-first wraps it:

```nix
get-hubbed-wrapped =
  pkgs.runCommand "get-hubbed"
    { nativeBuildInputs = [ pkgs.makeWrapper ]; }
    ''
      mkdir -p $out/bin
      makeWrapper ${get-hubbed-unwrapped}/bin/get-hubbed $out/bin/get-hubbed \
        --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.gh ]}

      # Propagate share directory (plugin manifest, hooks, etc.)
      if [ -d "${get-hubbed-unwrapped}/share" ]; then
        cp -r ${get-hubbed-unwrapped}/share $out/share
      fi
    '';
```

Key lesson: when wrapping, always propagate the `share/` directory so the package manifest and hooks survive.

---

## lux (Go, command.App-based, with hooks)

### plugin.json (generated at build time)

```json
{
  "name": "lux",
  "mcpServers": {
    "lux": { "type": "stdio", "command": "lux" }
  }
}
```

### Hook integration (command.App pattern)

lux uses `command.App` which registers the hook subcommand as a hidden command:

```go
app.AddCommand(&command.Command{
    Name:   "hook",
    Hidden: true,
    Description: command.Description{Short: "Handle PreToolUse hook"},
    RunCLI: func(ctx context.Context, args json.RawMessage) error {
        tools.RegisterAll(app, nil) // ensure tool mappings are loaded
        return app.HandleHook(os.Stdin, os.Stdout)
    },
})
```

### flake.nix (workspace build)

```nix
{ pkgs, goWorkspaceSrc, goVendorHash }:

pkgs.buildGoModule {
  pname = "lux";
  version = "0.1.0";
  src = goWorkspaceSrc;
  vendorHash = goVendorHash;
  GOWORK = "";
  overrideModAttrs = _: _: {
    GOWORK = "";
    buildPhase = ''
      runHook preBuild
      go work vendor -e
      runHook postBuild
    '';
  };
  subPackages = [ "packages/lux/cmd/lux" ];
  nativeBuildInputs = [ pkgs.scdoc ];
  postInstall = ''
    $out/bin/lux _generate $out
    mkdir -p $out/share/man/man5
    scdoc < ${goWorkspaceSrc}/packages/lux/doc/lux-config.5.scd > $out/share/man/man5/lux-config.5
  '';
}
```

Shows coexistence with other postInstall tasks (man page generation via scdoc).

---

## chix (Rust, static, with PostToolUse hooks and skills)

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

Note: chix is a combined package with MCP server, PostToolUse hooks (for auto-formatting .nix files), and skills. Unlike Go packages which use per-package PreToolUse hooks for tool routing, chix uses a static PostToolUse hook for a different purpose (formatting).

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
- Package directory name matches the `name` field and binary name
- PostToolUse hooks are installed alongside the package manifest
- No Go dependency needed — Rust packages use static manifests

---

## purse-first Marketplace Aggregation

All packages are aggregated in purse-first's `flake.nix`:

```nix
marketplace = pkgs.symlinkJoin {
  name = "claude-plugin-marketplace";
  paths = [
    gritPkg
    get-hubbed-wrapped
    luxPkg
    chixPkg
    # ... other packages
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

The `symlinkJoin` merges all `share/purse-first/<name>/` directories (including plugin.json, mappings.json, hooks/, and skills/) into a single tree, then `generate-marketplace` reads them to produce the final `marketplace.json`. Per-package hooks survive the join and are discovered by Claude Code at runtime.

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

1. **Workspace vendor hash**: All Go packages in the monorepo share a single `goVendorHash`. It only covers external dependencies — local code changes never invalidate it. Only update when adding/changing external Go dependencies.

2. **Share directory propagation**: When wrapping a binary with `makeWrapper`, the original `share/` directory is NOT automatically included. Copy or symlink it explicitly — this includes hooks/ and skills/, not just plugin.json.

3. **Package name vs binary name**: The package `name` field and the directory under `share/purse-first/` must match, but the `command` field uses the actual binary name.

4. **Hidden subcommand**: Always mark `generate-plugin` and `hook` as `Hidden: true` — they're build-time/runtime utilities, not user-facing.

5. **Mapping order matters**: `FindMatch` returns the first matching mapping. Specific subcommand mappings (e.g., `"git log"`) must be declared before general catch-alls (e.g., `"git "`), otherwise the catch-all matches first and the targeted suggestion is never reached.

6. **Multiple prefixes per mapping**: Use multiple `CommandPrefixes` when different commands map to the same tool (e.g., `"git checkout"` and `"git switch"` both map to the `checkout` tool).

7. **Multiple tools per mapping**: Use multiple `Tool` calls when a subcommand maps to more than one MCP tool (e.g., `"git branch"` suggests both `branch_list` and `branch_create`).

8. **Hook subcommand required**: If you declare `MapsTools` on any command, you MUST also wire a `hook` subcommand (calls `app.HandleHook`). The generated `pre-tool-use` wrapper script calls `<binary> hook` — without it, tool routing silently fails.
