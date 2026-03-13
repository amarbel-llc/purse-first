---
name: Purse-First Overview
description: This skill should be used when the user asks "what is purse-first", "how do packages work", "how does the marketplace work", "getting started with purse-first", "explain purse-first", or is working in a repository that contains a `.claude-plugin/` directory, a `share/purse-first/` output path, a `flake.nix` that uses `mkMarketplace`, or any flake input referencing purse-first. Also applies when the user mentions purse-first without a specific task, wants to understand the package framework, or asks about the relationship between MCP servers, skills, and packages.
version: 0.1.0
---

# Purse-First Package Framework

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

Purse-first is a package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages for Claude Code. It lets package authors distribute tools that Claude Code agents discover and use automatically — no manual configuration by end users.

## Why Purse-First Exists

Without purse-first, getting an MCP server into Claude Code requires manual JSON editing of settings files. Purse-first automates this: package authors declare their tools in a standard manifest, and the framework handles discovery, installation, and tool routing.

## Core Concepts

### Packages

A **package** is the unit of distribution. Every package outputs a `share/purse-first/<name>/` directory containing a `plugin.json` manifest. Packages come in three flavors:

| Flavor | Ships | Examples |
|--------|-------|----------|
| **MCP-only** | MCP server(s) + optional tool mappings | git-mcp, github-mcp, lsp-mcp |
| **Skill-only** | Skills only (no MCP server) | bob, robin, tap-dancer |
| **MCP + Skills** | MCP server(s) + bundled skills | nix-mcp |

### Marketplace

A **marketplace** aggregates multiple packages into a single installable derivation via Nix's `symlinkJoin`. The `purse-first generate-marketplace` command scans all packages and produces a `.claude-plugin/marketplace.json` that Claude Code consumes.

### Tool Mappings

**Mappings** redirect built-in Claude Code tools to MCP tools. For example, git-mcp's mappings intercept `git status` Bash commands and suggest using the `git-mcp status` MCP tool instead. The purse-first PreToolUse hook enforces these at runtime.

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
| Build a Go CLI or MCP server with go-mcp | **bob:go-cli-framework** |
| Create or modify a justfile | **bob:design_patterns-just** |

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
