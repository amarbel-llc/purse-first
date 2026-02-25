# Claude Plugins Skill Design

**Date:** 2026-02-25
**Status:** Approved

## Goal

Create a new bob skill `claude-plugins` that serves as the authoritative
reference for the Claude Code plugin specification. Concise SKILL.md with
quick-reference tables, full spec in `references/`.

## Design Decisions

- **Opt-in only** (`disable-model-invocation: true`) — heavy reference content,
  only loaded when explicitly requested
- **Concise SKILL.md + references/** — matches `bob:mcp` skill pattern
- **Cross-references `bob:mcp`** for MCP protocol details rather than
  duplicating them
- **Full spec in `references/claude-plugin-specification.md`** — downloaded from
  official docs on 2026-02-25

## SKILL.md Structure (~130 lines)

1. YAML frontmatter (name, description, disable-model-invocation, version)
2. H1 + overview — what the plugin system is, cross-ref to `bob:mcp`
3. Plugin Components matrix — what a plugin can contain
4. Plugin Manifest Schema — quick-reference table of `plugin.json` fields
5. Skills Quick Reference — SKILL.md frontmatter fields
6. Agents Quick Reference — agent frontmatter fields
7. Hook Events Summary — event name, when it fires, can block?
8. Marketplace Schema — required fields, plugin source types
9. Finding What You Need — file-path table pointing into `references/`
10. Related Skills — cross-refs

## Additional Changes

- Add `disable-model-invocation: true` to `bob:mcp` skill
- Add `disable-model-invocation: true` to `bob:voldemort` skill
- Register `./skills/claude-plugins` in `.claude-plugin/plugin.json`

## File Layout

```
skills/claude-plugins/
├── SKILL.md
└── references/
    └── claude-plugin-specification.md   # moved from docs/
```
