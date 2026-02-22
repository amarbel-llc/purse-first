# Self-Contained Marketplace MCP Resolution

**Date:** 2026-02-22

## Problem

`marketplace.json` uses `{source: "github", repo: "amarbel-llc/purse-first"}`
for each plugin entry. When Claude Code installs a plugin (e.g.,
`grit@purse-first`), it clones the monorepo and finds the root
`.claude-plugin/plugin.json` which is `bob` (skills only, no MCPs). Every
package loses its individual MCP identity and becomes a copy of `bob`.

## Root Cause

The `Generate()` function in `internal/marketplace/generate.go` resolves plugin
`source` from the `Repo` field in `marketplace-config.json`. Since every plugin
points to the same GitHub repo, Claude Code clones the same monorepo root for
all of them.

## Solution

Replace GitHub source references with relative directory paths pointing to each
package's directory within the marketplace output. This matches the pattern used
by the official Claude marketplace (`claude-plugins-official`), where local
plugins use paths like `"./plugins/typescript-lsp"`.

### Before

```json
{
  "name": "grit",
  "source": { "source": "github", "repo": "amarbel-llc/purse-first" },
  "mcpServers": { "grit": { "command": "grit", "type": "stdio" } }
}
```

### After

```json
{
  "name": "grit",
  "source": "./share/purse-first/grit",
  "mcpServers": { "grit": { "command": "grit", "type": "stdio" } }
}
```

## Changes

### `internal/marketplace/generate.go`

In `Generate()`, replace the GitHub/repo source resolution (lines 158-166) with
a relative directory path computed from the package name:

```go
source = "./share/purse-first/" + dp.Name
```

The `pluginsDir` relationship already encodes this: packages are discovered from
`<root>/share/purse-first/*/plugin.json`, so the relative path from the
marketplace root (where `.claude-plugin/marketplace.json` lives) is always
`./share/purse-first/<name>`.

To keep this generic (not hardcoded to `purse-first`), derive it from the
discovery path or pass the plugins directory prefix through the config.

### `marketplace-config.json`

Remove per-plugin `repo` fields since they no longer drive `source`. The
top-level `repo` remains for homepage/metadata purposes.

### No changes needed

- `mkMarketplace.nix` — already produces the correct directory layout
- Per-package Nix expressions — already output to `share/purse-first/<name>/`
- `purse-first install` — still calls `claude plugin marketplace add/install`
- MCP `command` fields — remain bare names, resolved via PATH
- `strict: true` — stays, so marketplace entry is authoritative

## Future Considerations

The relative directory source pattern is builder-agnostic. Any build system that
produces the standard layout will work:

```
$out/
├── bin/<commands>
├── share/purse-first/<package>/plugin.json
└── .claude-plugin/marketplace.json
```

Non-Nix builders are a mid-term goal. This design does not introduce any
Nix-specific coupling — the Go code computes relative paths from the directory
structure, not from Nix concepts.
