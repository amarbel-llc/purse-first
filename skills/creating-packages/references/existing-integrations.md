# Reference Package Patterns

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

> **Note.** `grit` and `get-hubbed` are no longer in this repo — they ship as moxins in the `moxy` repo. `lux` remains here but is dormant. The patterns below are retained as illustrative shapes for building Go MCP packages against the purse-first framework; do not expect to find these as live derivations in this flake.

Side-by-side patterns for MCP packages. Use them as references when building your own.

## Summary Table

| Package | Language | Pattern | Command | Hooks | Extra wrapping |
|---------|----------|---------|---------|-------|----------------|
| grit | Go (flag) | generate (workspace) | `grit` | PreToolUse (per-package) | none |
| get-hubbed | Go (flag) | generate (workspace) | `get-hubbed` | PreToolUse (per-package) | gh on PATH |
| lux | Go (command.App) | generate (workspace) | `lux` | PreToolUse (per-package) | none |

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

Each Go binary becomes an attribute of `gomod.nix`'s `packages` set, built via
the `mkGoModule` factory (a thin wrapper over `pkgs.buildGoApplication` from
the gomod2nix overlay):

```nix
# in gomod.nix's packages attrset
grit = mkGoModule {
  pname = "grit";
  version = "0.1.0";
  subPackages = [ "packages/grit/cmd/grit" ];
  postInstall = ''
    $out/bin/grit generate-plugin $out
  '';
};
```

`mkGoModule` pins `src`, `pwd`, and `modules` to the workspace's RFC 0001
`go-pkgs-test` source and the shared `gomod2nix.toml` lockfile at the workspace
root. The lockfile pins external module versions — local code changes never
invalidate it; only `go.mod` / `go.sum` / `go.work` changes do.

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

Same `mkGoModule` pattern as grit:

```nix
get-hubbed-unwrapped = mkGoModule {
  pname = "get-hubbed";
  version = "0.1.0";
  subPackages = [ "packages/get-hubbed/cmd/get-hubbed" ];
  postInstall = ''
    $out/bin/get-hubbed generate-plugin $out
  '';
};
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
lux = mkGoModule {
  pname = "lux";
  version = "0.1.0";
  subPackages = [ "packages/lux/cmd/lux" ];
  nativeBuildInputs = [ pkgs.scdoc ];
  postInstall = ''
    $out/bin/lux _generate $out
    mkdir -p $out/share/man/man5
    scdoc < $src/packages/lux/doc/lux-config.5.scd > $out/share/man/man5/lux-config.5
  '';
};
```

Shows coexistence with other postInstall tasks (man page generation via scdoc).
`$src` resolves to the workspace source `mkGoModule` already passed to
`buildGoApplication` — no need to re-reference the source binding by name.

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

1. **Workspace lockfile**: All Go packages in the monorepo share a single `gomod2nix.toml` lockfile at the workspace root. It pins external module versions — local code changes never invalidate it. Run `just deps` (alias for `just build-nix-gomod2nix`) after adding/changing external Go dependencies; CI fails on lockfile drift.

2. **Share directory propagation**: When wrapping a binary with `makeWrapper`, the original `share/` directory is NOT automatically included. Copy or symlink it explicitly — this includes hooks/ and skills/, not just plugin.json.

3. **Package name vs binary name**: The package `name` field and the directory under `share/purse-first/` must match, but the `command` field uses the actual binary name.

4. **Hidden subcommand**: Always mark `generate-plugin` and `hook` as `Hidden: true` — they're build-time/runtime utilities, not user-facing.

5. **Mapping order matters**: `FindMatch` returns the first matching mapping. Specific subcommand mappings (e.g., `"git log"`) must be declared before general catch-alls (e.g., `"git "`), otherwise the catch-all matches first and the targeted suggestion is never reached.

6. **Multiple prefixes per mapping**: Use multiple `CommandPrefixes` when different commands map to the same tool (e.g., `"git checkout"` and `"git switch"` both map to the `checkout` tool).

7. **Multiple tools per mapping**: Use multiple `Tool` calls when a subcommand maps to more than one MCP tool (e.g., `"git branch"` suggests both `branch_list` and `branch_create`).

8. **Hook subcommand required**: If you declare `MapsTools` on any command, you MUST also wire a `hook` subcommand (calls `app.HandleHook`). The generated `pre-tool-use` wrapper script calls `<binary> hook` — without it, tool routing silently fails.
