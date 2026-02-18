# Unified Terminology Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace "plugin" with "package" in all user-facing text across docs, skills, CLI strings, and error messages. Add TODO markers for deferred code-level and protocol-level breaking changes. Create root README.md and CLAUDE.md.

**Architecture:** Pure text changes — no code logic changes, no API changes, no test behavior changes. The protocol doc, skills, marketplace-config, CLI help/error strings, and description fields all get rewritten. Go identifiers, filenames (`plugin.json`), and library names stay but receive TODO comments.

**Tech Stack:** Markdown, JSON, Go (string literals only), git

**Design doc:** `docs/plans/2026-02-18-unified-terminology-design.md`

---

### Task 1: Create root README.md

**Files:**
- Create: `README.md`

**Step 1: Write README.md**

Use the canonical one-liner and three-layer description from the design doc. Include:
- One-liner: "purse-first is a package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages for humans and agents like Claude Code."
- Three layers table (Protocol, CLI, Libraries)
- Package flavors table (MCP package, Skill package, MCP + Skill package) with examples
- Current packages section listing grit, get-hubbed, lux, chix, robin, tap-dancer, bob
- Brief getting started / installation pointer
- Link to `docs/purse-first-protocol.md`

**Step 2: Commit**

```
git add README.md
git commit -m "Add root README with unified terminology"
```

---

### Task 2: Create root CLAUDE.md

**Files:**
- Create: `CLAUDE.md`

**Step 1: Write CLAUDE.md**

Include:
- One-liner description
- Build/test commands (`just build-all`, `just test`, `nix flake check`)
- Repository layout table (cmd/, internal/, libs/, purse/, skills/, docs/, zz-tests_bats/)
- Terminology section: "package" (not "plugin") for adopters, three flavors, "marketplace" for aggregated output, "bob" is purse-first's own skill package
- Key conventions: stable-first nixpkgs, GPG signing required, TAP-14 test output, bats for CLI tests
- Pointer to `docs/purse-first-protocol.md` for the full protocol spec

**Step 2: Commit**

```
git add CLAUDE.md
git commit -m "Add root CLAUDE.md with project instructions"
```

---

### Task 3: Update marketplace-config.json descriptions

**Files:**
- Modify: `marketplace-config.json`

**Step 1: Update descriptions**

| Field | Current | New |
|-------|---------|-----|
| top-level `description` | "MCP servers and tool routing for Claude Code, built with Nix" | "Package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages" |
| `plugins.purse-first.description` | "Plugin integration skills — guides conversion of MCP servers into purse-first plugins" | "Package integration skills — guides conversion of MCP servers into purse-first packages" |
| `plugins.chix.description` | "Nix MCP server and skills for Claude Code — build, evaluate, search, and manage flakes" | No change (doesn't use "plugin") |
| `plugins.robin.description` | Keep as-is | No change |
| `plugins.tap-dancer.description` | Keep as-is | No change |

All other per-package descriptions are fine — they don't use "plugin."

**Step 2: Commit**

```
git add marketplace-config.json
git commit -m "Update marketplace-config descriptions to use package terminology"
```

---

### Task 4: Update .claude-plugin/plugin.json description

**Files:**
- Modify: `.claude-plugin/plugin.json`

**Step 1: Update description field**

Change from:
```
"MCP-first tool routing for Claude Code — aggregates MCP servers into an installable marketplace with plugin integration skills"
```
To:
```
"Package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages for humans and agents like Claude Code"
```

**Step 2: Commit**

```
git add .claude-plugin/plugin.json
git commit -m "Update bob plugin.json description to use package terminology"
```

---

### Task 5: Update CLI help strings and error messages

**Files:**
- Modify: `cmd/purse-first/main.go`

**Step 1: Update each string**

Line 21 — root `Short`:
```go
Short: "Package framework for Claude Code",
```

Line 37 — post-hook `Short`:
```go
Short: "PostToolUse hook handler (fires package notifications)",
```

Line 47 — session-end `Short`:
```go
Short: "SessionEnd hook handler (fires package stop notifications)",
```

Line 67 — install `Short`:
```go
Short: "Install purse-first marketplace and packages into Claude Code",
```

Line 84 — generate-marketplace `Short`:
```go
Short: "Generate .claude-plugin/marketplace.json from discovered packages",
```

Line 97 — error message:
```go
return fmt.Errorf("discovering packages: %w", err)
```

Line 108 — stderr output:
```go
fmt.Fprintf(os.Stderr, "wrote %s (%d packages)\n", outputPath, len(m.Plugins))
```

Line 113 — flag help:
```go
genMarketplaceCmd.Flags().StringVar(&pluginsDir, "plugins-dir", "", "directory containing package manifest files")
```

Line 135 — error message:
```go
return fmt.Errorf("generating local package manifest: %w", err)
```

Line 152 — validate `Short`:
```go
Short: "Validate package, mapping, or marketplace documents",
```

Lines 153-157 — validate `Long`:
```go
Long: `Validate purse-first package documents.

Accepts a file path, directory, or "-" for stdin.
Auto-detects document type from filename or content.
Use --type to override detection. Use --strict to promote warnings to errors.`,
```

Line 164 — error message:
```go
return fmt.Errorf("unknown type %q; use plugin, mapping, or marketplace", validateType)
```
Note: Keep "plugin" here — it's the `--type plugin` flag value that matches the filename `plugin.json`.

**Step 2: Run existing tests to verify nothing breaks**

```bash
just test
```

**Step 3: Commit**

```
git add cmd/purse-first/main.go
git commit -m "Update CLI help strings and error messages to use package terminology"
```

---

### Task 6: Update install.go user-facing strings

**Files:**
- Modify: `internal/install/install.go`

**Step 1: Update TAP output and error messages**

Line 116 — TAP output:
```go
fmt.Fprintf(w, "ok %d - read marketplace.json (%d packages)\n", n, len(m.Plugins))
```

Lines 139-142 — install loop output (keep `claude plugin install` as-is since that's the Claude Code CLI command):
```go
fmt.Fprintf(w, "not ok %d - install package %s\n", n, styleCode.Render(plugin.Name))
return fmt.Errorf("installing package %q: %w", plugin.Name, err)
```
```go
fmt.Fprintf(w, "ok %d - install package %s\n", n, styleCode.Render(plugin.Name))
```

Line 101 — comment:
```go
// TAP header: 4 fixed steps + one per package + optional hook uninstall
```

Line 134 — comment:
```go
// 5..N Install each package
```

**Step 2: Run tests**

```bash
just test
```

**Step 3: Commit**

```
git add internal/install/install.go
git commit -m "Update install output strings to use package terminology"
```

---

### Task 7: Rewrite protocol doc (plugin → package)

**Files:**
- Modify: `docs/purse-first-protocol.md`

**Step 1: Full rewrite**

This is the most extensive change. Replace "plugin" with "package" throughout, except:
- Keep `plugin.json` as-is (it's the filename)
- Keep `.claude-plugin/` as-is (Claude Code's directory)
- Keep `claude plugin validate` as-is (Claude Code's CLI)

Key changes:
- Title stays "Purse-First Plugin Protocol" → "Purse-First Package Protocol"
- Terminology table: "plugin" definition → "package" definition
- All prose: "A plugin ships..." → "A package ships..."
- Section headers: "Single Plugin Discovery" → "Single Package Discovery"
- "Aggregated Discovery" section: "multiple plugin derivations" → "multiple package derivations"
- "Marketplace Generation" section: "aggregates multiple plugins" → "aggregates multiple packages"
- "Producing a Conformant Derivation" → keep title, update prose
- Directory references: `share/purse-first/<plugin-name>/` → `share/purse-first/<package-name>/` in prose (but actual path convention doesn't change yet)
- Add a note in the terminology table: "`plugin.json` — the manifest filename. Named for historical reasons; the file describes a package."

**Step 2: Commit**

```
git add docs/purse-first-protocol.md
git commit -m "Rewrite protocol doc: plugin → package terminology"
```

---

### Task 8: Update plugin-mcp skill

**Files:**
- Modify: `skills/plugin-mcp/SKILL.md`

**Step 1: Update prose (not skill name or directory)**

- Title: "Adding Purse-First Plugin Support" → "Adding Purse-First Package Support"
- All prose: "A purse-first plugin ships..." → "A purse-first package ships..."
- Flavor table: "MCP-only" stays but description uses "package"
- Section headers: "MCP Plugin" → "MCP Package", "Skills" subsection stays
- Checklist: "MCP Plugin" → "MCP Package", "Marketplace Registration" stays
- Keep `plugin.json` references (it's the filename)
- Keep `generate-plugin` references (it's the subcommand name)
- Keep `.claude-plugin/plugin.json` references (Claude Code's convention)

The skill `name` field in frontmatter ("Plugin MCP Integration") and the directory name (`plugin-mcp/`) stay as-is — they are identifiers.

**Step 2: Update skill description in frontmatter**

Update trigger phrases to include "package" alongside "plugin":
```yaml
description: This skill should be used when the user asks to "add purse-first support", "turn an MCP into a package", "create a package manifest", "add generate-plugin command", "register with purse-first marketplace", "add skills to a package", "bundle skills in a package", or mentions purse-first package integration, package manifest generation, or skill bundling.
```

**Step 3: Commit**

```
git add skills/plugin-mcp/SKILL.md
git commit -m "Update plugin-mcp skill prose to use package terminology"
```

---

### Task 9: Update plugin-mcp skill references

**Files:**
- Modify: `skills/plugin-mcp/references/existing-integrations.md`
- Modify: `skills/plugin-mcp/references/mapping-api.md`

**Step 1: Update existing-integrations.md**

- "Existing Purse-First Integrations" → "Existing Purse-First Packages"
- "all four MCP servers currently integrated" → "all four MCP packages currently integrated"
- Keep `plugin.json` and `generate-plugin` references (filenames/commands)
- "exposed by the plugin" → "exposed by the package"

**Step 2: Update mapping-api.md**

- "exposed by the plugin" → "exposed by the package"
- "every tool in the plugin" → "every tool in the package"
- Keep `NewPluginBuilder` references (Go API, unchanged)

**Step 3: Commit**

```
git add skills/plugin-mcp/references/
git commit -m "Update skill reference docs to use package terminology"
```

---

### Task 10: Add TODO markers to Go code

**Files:**
- Modify: `internal/marketplace/types.go`
- Modify: `internal/marketplace/generate.go`
- Modify: `internal/mcp/marketplace.go`
- Modify: `internal/localplugin/generate.go`
- Modify: `internal/install/install.go`
- Modify: `purse/writer.go`
- Modify: `libs/go-mcp/purse/purse.go`
- Modify: `libs/go-mcp/purse/writer.go`

**Step 1: Add TODO comments**

Add a single block comment at the top of each file (below the package line):

For `internal/marketplace/types.go`:
```go
// TODO(terminology): rename Plugin → Package, PluginMeta → PackageMeta,
// DiscoveredPlugin → DiscoveredPackage when breaking change lands.
```

For `internal/marketplace/generate.go`:
```go
// TODO(terminology): rename DiscoverPlugins → DiscoverPackages,
// discoverPluginSkills → discoverPackageSkills when breaking change lands.
```

For `internal/mcp/marketplace.go`:
```go
// TODO(terminology): rename DiscoverPlugins → DiscoverPackages,
// discoverFromPluginDir → discoverFromPackageDir when breaking change lands.
```

For `internal/localplugin/generate.go`:
```go
// TODO(terminology): rename package localplugin → localpackage
// when breaking change lands.
```

For `internal/install/install.go`:
```go
// TODO(terminology): rename marketplacePlugin → marketplacePackage
// when breaking change lands.
```

For `purse/writer.go`:
```go
// TODO(terminology): rename WritePlugin → WritePackage, plugin.json → package.json (or .toml)
// when breaking change lands.
```

For `libs/go-mcp/purse/purse.go`:
```go
// TODO(terminology): rename Plugin → Package, PluginBuilder → PackageBuilder,
// NewPluginBuilder → NewPackageBuilder when breaking change lands.
```

For `libs/go-mcp/purse/writer.go`:
```go
// TODO(terminology): rename WritePlugin → WritePackage, plugin.json → package.json (or .toml)
// when breaking change lands.
```

**Step 2: Add TODO markers for library renames**

For `libs/go-mcp/go.mod`, add a comment at the top:
```
// TODO(terminology): rename module path from go-mcp to TBD (e.g., go-purse)
// when breaking change lands.
```

For `libs/rust-mcp/` — add a comment in the `Cargo.toml` or `README.md`:
```
# TODO(terminology): rename crate from rust-mcp/mcp-server to TBD
# when breaking change lands.
```

**Step 3: Run tests to verify TODO comments don't break anything**

```bash
just test
```

**Step 4: Commit**

```
git add internal/ purse/ libs/
git commit -m "Add TODO(terminology) markers for deferred plugin → package rename"
```

---

### Task 11: Update go-mcp and rust-mcp READMEs

**Files:**
- Modify: `libs/go-mcp/README.md`
- Modify: `libs/rust-mcp/README.md`

**Step 1: Add purse-first context to go-mcp README**

After the first paragraph, add:
```markdown
`go-mcp` is part of [purse-first](../../README.md), a package framework for
bundling CLIs, MCP servers, and skills into composable, Nix-built packages.
```

**Step 2: Add purse-first context to rust-mcp README**

After the title, add:
```markdown
Part of [purse-first](../../README.md), a package framework for bundling CLIs,
MCP servers, and skills into composable, Nix-built packages.
```

**Step 3: Commit**

```
git add libs/go-mcp/README.md libs/rust-mcp/README.md
git commit -m "Add purse-first context to library READMEs"
```

---

### Task 12: Final verification

**Step 1: Search for stale "plugin" references in user-facing text**

```bash
grep -rn '[Pp]lugin' README.md CLAUDE.md docs/purse-first-protocol.md skills/ marketplace-config.json .claude-plugin/plugin.json cmd/purse-first/main.go internal/install/install.go
```

Verify every remaining "plugin" occurrence is one of:
- `plugin.json` (filename)
- `.claude-plugin/` (Claude Code directory)
- `claude plugin` (Claude Code CLI)
- `generate-plugin` (subcommand name)
- `--type plugin` (flag value)
- `mcp__plugin_` (tool name prefix)
- Go identifier (should have TODO marker)

**Step 2: Build and test**

```bash
just build-all
just test
```

**Step 3: Commit any fixes**

If any stale references found, fix and commit.
