# Bob Skill Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve the bob skill package so skills trigger reliably with broader intent-based descriptions, provide framework orientation to agents, and cross-reference each other.

**Architecture:** Add two new skills (`overview`, `using-packages`), rename `plugin-mcp` to `creating-packages`, update all trigger descriptions to be intent-based, add cross-references between skills, and update the plugin manifest.

**Tech Stack:** Markdown (SKILL.md files), JSON (plugin.json)

---

### Task 1: Rename `plugin-mcp` to `creating-packages`

**Files:**
- Rename: `skills/plugin-mcp/` → `skills/creating-packages/`
- Modify: `skills/creating-packages/SKILL.md` (frontmatter only for now — content updates in Task 4)

**Step 1: Rename the directory**

Run: `mv skills/plugin-mcp skills/creating-packages`

**Step 2: Update the YAML frontmatter name**

In `skills/creating-packages/SKILL.md`, change the frontmatter `name` field from:

```yaml
name: Plugin MCP Integration
```

to:

```yaml
name: Creating Packages
```

**Step 3: Verify the rename**

Run: `ls skills/creating-packages/SKILL.md`
Expected: file exists

**Step 4: Commit**

```bash
git add skills/plugin-mcp skills/creating-packages
git commit -m "refactor(skills): rename plugin-mcp to creating-packages

Clearer name that matches user mental model — skill is about creating
purse-first packages, not just MCP plugin integration."
```

---

### Task 2: Create the `overview` skill

**Files:**
- Create: `skills/overview/SKILL.md`

**Step 1: Create the skill directory**

Run: `mkdir -p skills/overview`

**Step 2: Write the SKILL.md**

Create `skills/overview/SKILL.md` with this content:

```markdown
---
name: Purse-First Overview
description: This skill should be used when the user asks "what is purse-first", "how do packages work", "how does the marketplace work", "getting started with purse-first", "explain purse-first", or is working in a repository that contains a `.claude-plugin/` directory, a `share/purse-first/` output path, a `flake.nix` that uses `mkMarketplace`, or any flake input referencing purse-first. Also applies when the user mentions purse-first without a specific task, wants to understand the package framework, or asks about the relationship between MCP servers, skills, and packages.
version: 0.1.0
---

# Purse-First Package Framework

Purse-first is a package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages for Claude Code. It lets package authors distribute tools that Claude Code agents discover and use automatically — no manual configuration by end users.

## Why Purse-First Exists

Without purse-first, getting an MCP server into Claude Code requires manual JSON editing of settings files. Purse-first automates this: package authors declare their tools in a standard manifest, and the framework handles discovery, installation, and tool routing.

## Core Concepts

### Packages

A **package** is the unit of distribution. Every package outputs a `share/purse-first/<name>/` directory containing a `plugin.json` manifest. Packages come in three flavors:

| Flavor | Ships | Examples |
|--------|-------|----------|
| **MCP-only** | MCP server(s) + optional tool mappings | grit, get-hubbed, lux |
| **Skill-only** | Skills only (no MCP server) | bob, robin, tap-dancer |
| **MCP + Skills** | MCP server(s) + bundled skills | chix |

### Marketplace

A **marketplace** aggregates multiple packages into a single installable derivation via Nix's `symlinkJoin`. The `purse-first generate-marketplace` command scans all packages and produces a `.claude-plugin/marketplace.json` that Claude Code consumes.

### Tool Mappings

**Mappings** redirect built-in Claude Code tools to MCP tools. For example, grit's mappings intercept `git status` Bash commands and suggest using the `grit status` MCP tool instead. The purse-first PreToolUse hook enforces these at runtime.

### Skills

**Skills** are Markdown documents (with YAML frontmatter) that teach Claude Code domain-specific workflows. They live in `skills/<skill-name>/SKILL.md` within a package directory and are discovered automatically.

## End-to-End Workflow

```
Author builds MCP server or CLI
        │
        ▼
Add purse-first support (bob:creating-packages skill)
  - Create plugin.json manifest
  - Define tool mappings (optional)
  - Ship skills (optional)
        │
        ▼
Build via Nix → $out/share/purse-first/<name>/plugin.json
        │
        ▼
Register in marketplace (add flake input + metadata)
        │
        ▼
Build marketplace → .claude-plugin/marketplace.json
        │
        ▼
Install: purse-first install → hooks registered in Claude Code
        │
        ▼
Runtime: PreToolUse hook routes tools via mappings
```

## Which Skill Do I Need?

| I want to... | Use this skill |
|--------------|----------------|
| Understand what purse-first is | You're reading it |
| Create a new package (MCP, skill, or both) | **bob:creating-packages** |
| Understand how installed packages work at runtime | **bob:using-packages** |
| Add output-limiting to MCP tools (pagination, truncation) | **bob:context-saving** |
| Build a Go CLI or MCP server with go-lib-mcp | **bob:go-cli-framework** |

## Terminology Reference

| Term | Meaning |
|------|---------|
| **Package** | A named unit of Claude Code functionality (not "plugin") |
| **Marketplace** | Aggregated JSON listing all available packages |
| **Mapping** | Rule redirecting a built-in tool to an MCP tool |
| **Skill** | Markdown doc teaching Claude Code a workflow |
| **plugin.json** | Package manifest (name retained for compatibility) |
| **mappings.json** | Tool routing rules file |
| **share/purse-first/** | Standard output directory for package metadata |
| **symlinkJoin** | Nix function composing multiple packages into one derivation |

## Protocol Summary

All packages follow the purse-first protocol. The key invariants:

- Package metadata lives at `$out/share/purse-first/<name>/`
- `plugin.json` is required; `mappings.json` and `skills/` are optional
- `plugin.name` must match the directory name
- Binaries resolve from `$out/bin/<command>`
- Packages compose via `symlinkJoin` without conflicts
- Mapping precedence: package-shipped < global user < project-local

For the full protocol specification, see `docs/purse-first-protocol.md` in the purse-first repository.
```

**Step 3: Verify the file**

Run: `head -5 skills/overview/SKILL.md`
Expected: YAML frontmatter with `name: Purse-First Overview`

**Step 4: Commit**

```bash
git add skills/overview
git commit -m "feat(skills): add overview skill for framework orientation

Provides agents with mental model of purse-first before they dive into
specific tasks. Includes concept definitions, end-to-end workflow,
skill routing table, and terminology reference."
```

---

### Task 3: Create the `using-packages` skill

**Files:**
- Create: `skills/using-packages/SKILL.md`

**Step 1: Create the skill directory**

Run: `mkdir -p skills/using-packages`

**Step 2: Write the SKILL.md**

Create `skills/using-packages/SKILL.md` with this content:

```markdown
---
name: Using Packages
description: This skill should be used when the user asks "how do purse-first hooks work", "why is my tool being denied", "how to customize mappings", "troubleshoot package discovery", "how does tool routing work", "how do I override a mapping", "debug purse-first", or is experiencing unexpected tool denials, mapping conflicts, or package discovery failures. Also applies when working with `.purse-first/` project-local overrides, `$XDG_STATE_HOME/purse-first/` user overrides, or `purse-first install` and `purse-first hook` commands.
version: 0.1.0
---

# Using Purse-First Packages

This skill covers the consumer side of purse-first: how installed packages work at runtime, how tool routing decisions are made, and how to troubleshoot issues.

For creating new packages, see the **bob:creating-packages** skill.
For framework orientation, see the **bob:overview** skill.

## How Tool Routing Works

When Claude Code invokes a built-in tool (Bash, Read, Grep, etc.), purse-first's PreToolUse hook fires:

1. Hook receives: tool name, tool input (command, file path, etc.), working directory
2. Loads mappings from three sources (lowest to highest priority):
   - **Package-shipped:** `share/purse-first/<name>/mappings.json`
   - **Global user:** `$XDG_STATE_HOME/purse-first/*.json` (default: `~/.local/state/purse-first/`)
   - **Project-local:** `.purse-first/*.json` in the working directory
3. Matches the tool invocation against mapping rules:
   - For Bash: matches `command_prefixes` against the command string
   - For file tools (Read, Grep, Glob): matches `extensions` against the file path
   - Catch-all mappings (no prefixes or extensions) match unconditionally
4. If a match is found: denies the built-in tool and suggests MCP alternatives
5. If no match: allows the tool to proceed

## Mapping Precedence

Higher-priority sources override lower-priority ones for the same `(replaces, server)` pair:

```
Package-shipped (lowest)  →  Global user  →  Project-local (highest)
```

This means you can always override or disable a package's mappings without modifying the package itself.

## Customizing Mappings

### Project-Local Overrides

Create `.purse-first/<name>.json` in your project root to override mappings for that project:

```json
{
  "server": "grit",
  "mappings": [
    {
      "replaces": "Bash",
      "command_prefixes": ["git log"],
      "tools": [
        {"name": "log", "use_when": "viewing commit history"}
      ],
      "reason": "Use grit log for structured commit data"
    }
  ]
}
```

### Global User Overrides

Place mapping files in `$XDG_STATE_HOME/purse-first/` (typically `~/.local/state/purse-first/`) to apply overrides across all projects.

### Disabling a Mapping

To disable a specific package mapping, create an override with an empty `tools` array for the same `replaces`/`server` pair. The higher-priority source will take precedence.

## Troubleshooting

### "Why is my tool being denied?"

When a tool is denied, purse-first prints the mapping's `reason` field and lists suggested MCP tools. To investigate:

1. Check which mappings are loaded: look at `share/purse-first/*/mappings.json` in the marketplace
2. Check for project-local overrides: `ls .purse-first/`
3. Check for user overrides: `ls $XDG_STATE_HOME/purse-first/`

### "Package not discovered"

Verify the package layout:

```bash
# Check the package directory exists
ls $PURSE_FIRST_PLUGINS_DIR/<package-name>/plugin.json

# Verify plugin.name matches directory name
cat $PURSE_FIRST_PLUGINS_DIR/<package-name>/plugin.json | jq .name

# Check the binary exists
ls $(dirname $PURSE_FIRST_PLUGINS_DIR)/../bin/<command>
```

### "Skills not loading"

Skills are discovered by globbing `skills/*/SKILL.md` under each package directory:

```bash
# Check skills exist in the package
ls $PURSE_FIRST_PLUGINS_DIR/<package-name>/skills/*/SKILL.md

# Verify YAML frontmatter has name and description
head -5 $PURSE_FIRST_PLUGINS_DIR/<package-name>/skills/<skill>/SKILL.md
```

### "Hook not firing"

Verify hooks are installed in Claude Code settings:

```bash
purse-first install --hooks
```

This registers the `purse-first hook` command as a PreToolUse handler in Claude Code's settings.

## Key Commands

| Command | Purpose |
|---------|---------|
| `purse-first install` | Install marketplace and register hooks |
| `purse-first install --hooks` | Only register hook handlers |
| `purse-first uninstall-hooks` | Remove hook handlers |
| `purse-first hook` | PreToolUse handler (called by Claude Code, not manually) |
| `purse-first validate` | Validate plugin/mapping/marketplace documents |
| `purse-first generate-marketplace` | Build marketplace.json from discovered packages |
| `purse-first generate-local-plugin` | Discover skills and update plugin.json |

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `PURSE_FIRST_PLUGINS_DIR` | `<exe>/../share/purse-first/` | Override package discovery root |
| `XDG_STATE_HOME` | `$HOME/.local/state` | Base for user-level mapping overrides |
| `XDG_CONFIG_HOME` | `$HOME/.config` | Base for user-level configuration |
```

**Step 3: Verify the file**

Run: `head -5 skills/using-packages/SKILL.md`
Expected: YAML frontmatter with `name: Using Packages`

**Step 4: Commit**

```bash
git add skills/using-packages
git commit -m "feat(skills): add using-packages skill for consumer-side guidance

Covers how tool routing works at runtime, mapping precedence,
customization via project-local and user overrides, and troubleshooting
common issues with package discovery and hook configuration."
```

---

### Task 4: Update `creating-packages` trigger description, framing, and cross-references

**Files:**
- Modify: `skills/creating-packages/SKILL.md`

**Step 1: Update the frontmatter description**

Replace the existing `description` field in the YAML frontmatter with:

```yaml
description: This skill should be used when the user asks to "add purse-first support", "turn an MCP into a package", "create a package manifest", "add generate-plugin command", "register with purse-first marketplace", "add skills to a package", "bundle skills in a package", "make this available as a Claude Code tool", "distribute this MCP server", "share this CLI with Claude", "set up plugin.json", "ship this as a package", or wants to package an MCP server, CLI, or skill set for distribution via purse-first. Also applies when working in a repo that has a `.claude-plugin/` directory, editing a `plugin.json` or `mappings.json`, or modifying a `flake.nix` that uses `mkMarketplace` or references purse-first as a flake input.
```

**Step 2: Update the opening paragraph to add "why" framing and cross-references**

Replace the opening paragraph (lines 7-9, after the frontmatter closing `---`):

```markdown
# Creating Purse-First Packages

If you're building an MCP server, CLI tool, or skill set and want Claude Code users to discover and use it without manual configuration, package it with purse-first. The framework handles discovery, installation, and tool routing automatically.

For a high-level understanding of the framework, see the **bob:overview** skill.
For understanding how installed packages behave at runtime, see the **bob:using-packages** skill.
For adding output-limiting to MCP tools, see the **bob:context-saving** skill.
For building Go MCP servers and CLIs, see the **bob:go-cli-framework** skill.
```

**Step 3: Update the cross-reference at the bottom of the file**

At the end of the file, after the existing "Reference Files" section, add:

```markdown
## Related Skills

- **bob:overview** — Framework orientation, terminology, and workflow overview
- **bob:using-packages** — How installed packages work at runtime, troubleshooting
- **bob:context-saving** — Adding output-limiting to MCP tools (pagination, truncation)
- **bob:go-cli-framework** — Building Go CLIs and MCP servers with go-lib-mcp
```

**Step 4: Add explicit reference-file guidance**

Replace the existing "Reference Files" section with:

```markdown
## Reference Files

Consult these when you need detailed implementation examples:

- **`references/existing-integrations.md`** — Read this when comparing approaches across languages. Shows side-by-side grit (Go/flag), lux (Go/cobra), get-hubbed (Go/cobra), and chix (Rust) implementations.
- **`references/mapping-api.md`** — Read this when adding tool mappings. Full MappingBuilder API reference with per-subcommand and catch-all mapping examples.
- **`examples/plugin.json`** — Minimal MCP package manifest template
- **`examples/plugin-skill-only.json`** — Skill-only package manifest template
- **`examples/plugin-mcp-with-skills.json`** — MCP package with bundled skills manifest template
- **`examples/generate-plugin-cobra.go`** — Cobra-based generate-plugin command
- **`examples/generate-plugin-flag.go`** — Flag-based generate-plugin command
- **`examples/flake-go.nix`** — Flake snippet for Go MCP with postInstall
- **`examples/flake-rust.nix`** — Flake snippet for Rust MCP with static copy
```

**Step 5: Commit**

```bash
git add skills/creating-packages/SKILL.md
git commit -m "feat(skills): update creating-packages with broader triggers, cross-refs

Adds intent-based trigger phrases, 'why' framing in opening paragraph,
cross-references to all related skills, and explicit guidance on when
to consult each reference file."
```

---

### Task 5: Update `context-saving` trigger description and cross-references

**Files:**
- Modify: `skills/context-saving/SKILL.md`

**Step 1: Update the frontmatter description**

Replace the existing `description` field with:

```yaml
description: This skill should be used when the user asks to "add context-saving", "add pagination", "add truncation", "limit output size", "add offset and limit", "reduce token usage", "add head and tail parameters", "limit results", or mentions unbounded output, large responses, PaginationInfo, TruncationInfo, output size management, or token waste in MCP server tools. Also applies when an MCP tool's output is too long, agent token usage is too high, or MCP responses need size limits.
```

**Step 2: Add cross-references after the opening paragraph**

After the existing opening paragraph (line 9: "Every MCP tool that can produce large output should implement one of two patterns."), add:

```markdown

For Go implementations of these patterns, see the **bob:go-cli-framework** skill (Context-Saving in Go section).
For packaging your MCP server after adding context-saving, see the **bob:creating-packages** skill.
```

**Step 3: Add a Related Skills section at the end of the file**

At the end of the file, after the existing "Reference Files" section, add:

```markdown
## Related Skills

- **bob:go-cli-framework** — Go implementations of pagination and truncation via the `output` package
- **bob:creating-packages** — Packaging your MCP server for distribution via purse-first
- **bob:overview** — Framework orientation and concept definitions
```

**Step 4: Commit**

```bash
git add skills/context-saving/SKILL.md
git commit -m "feat(skills): update context-saving with broader triggers, cross-refs

Adds intent-based trigger phrases for tool output size issues and
cross-references to go-cli-framework and creating-packages skills."
```

---

### Task 6: Update `go-cli-framework` trigger description and cross-references

**Files:**
- Modify: `skills/go-cli-framework/SKILL.md`

**Step 1: Update the frontmatter description**

Replace the existing `description` field with:

```yaml
description: This skill should be used when the user asks to "build a Go MCP server", "create a Go CLI tool", "add an MCP tool in Go", "use go-lib-mcp", "use command.App", "register MCP tools", "add context-saving in Go", "build a CLI with MCP support", "unified CLI and MCP server", or is building a Go project that imports go-lib-mcp, serves MCP tools, or combines CLI subcommands with MCP tool registration. Also applies when adding a subcommand to a go-lib-mcp-based project or working with the command, server, transport, output, or purse packages from go-lib-mcp.
```

Note: the overly-generic "add a command" trigger is removed and replaced with the scoped "adding a subcommand to a go-lib-mcp-based project".

**Step 2: Add cross-references after the opening paragraph**

After the existing opening paragraph (line 8: "...low-level packages for building MCP servers directly."), add:

```markdown

For framework orientation and when to use each skill, see the **bob:overview** skill.
For detailed context-saving patterns (pagination/truncation), see the **bob:context-saving** skill.
For packaging your tool for distribution via purse-first, see the **bob:creating-packages** skill.
```

**Step 3: Update the existing cross-reference to use the new skill name**

In the "Plugin Integration" section (line 154), change:

```markdown
For full plugin integration guidance, see the **bob:plugin-mcp** skill.
```

to:

```markdown
For full plugin integration guidance, see the **bob:creating-packages** skill.
```

**Step 4: Add a Related Skills section at the end of the file**

At the end of the file, after the existing "Reference Files" section, add:

```markdown
## Related Skills

- **bob:context-saving** — Detailed pagination and truncation patterns for MCP tools
- **bob:creating-packages** — Packaging your tool for distribution via purse-first
- **bob:overview** — Framework orientation, terminology, and workflow overview
- **bob:using-packages** — How installed packages work at runtime
```

**Step 5: Commit**

```bash
git add skills/go-cli-framework/SKILL.md
git commit -m "feat(skills): update go-cli-framework with scoped triggers, cross-refs

Removes overly-generic 'add a command' trigger, adds intent-based
phrases, cross-references to all related skills, and updates stale
bob:plugin-mcp reference to bob:creating-packages."
```

---

### Task 7: Update `.claude-plugin/plugin.json`

**Files:**
- Modify: `.claude-plugin/plugin.json`

**Step 1: Update the plugin manifest**

Replace the contents of `.claude-plugin/plugin.json` with:

```json
{
  "author": {
    "name": "friedenberg"
  },
  "description": "Package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages for humans and agents like Claude Code",
  "name": "bob",
  "skills": [
    "./skills/overview",
    "./skills/creating-packages",
    "./skills/using-packages",
    "./skills/context-saving",
    "./skills/go-cli-framework"
  ]
}
```

**Step 2: Verify the JSON is valid**

Run: `python3 -c "import json; json.load(open('.claude-plugin/plugin.json'))"`
Expected: no output (valid JSON)

**Step 3: Commit**

```bash
git add .claude-plugin/plugin.json
git commit -m "feat(skills): update plugin manifest with new and renamed skills

Adds overview and using-packages skills, updates plugin-mcp reference
to creating-packages. Skills now listed in recommended discovery order."
```

---

### Task 8: Verify all skills and build

**Step 1: Verify all skill directories exist with SKILL.md files**

Run: `ls skills/*/SKILL.md`
Expected:
```
skills/context-saving/SKILL.md
skills/creating-packages/SKILL.md
skills/go-cli-framework/SKILL.md
skills/overview/SKILL.md
skills/using-packages/SKILL.md
```

**Step 2: Verify YAML frontmatter in each skill**

Run: `for f in skills/*/SKILL.md; do echo "=== $f ==="; head -4 "$f"; echo; done`
Expected: each file has `---`, `name:`, `description:`, and closing `---`

**Step 3: Verify the old plugin-mcp directory no longer exists**

Run: `ls skills/plugin-mcp 2>&1`
Expected: "No such file or directory"

**Step 4: Run the project build if applicable**

Run: `just build` (or `nix build` if justfile not available)
Expected: build succeeds

**Step 5: Run validation if applicable**

Run: `just test` (or the relevant test command)
Expected: tests pass (may need adjustments for renamed skill path)
