# Voldemort Skill Design

## Overview

`voldemort` is a product-manager skill for purse-first that helps identify the
top user stories for a given project and inventories the current state of the
project against those stories. It lives in the purse-first repo as a skill-only
addition alongside existing bob skills.

## Modes

The skill auto-detects its mode by checking for `docs/user-stories.toml`:

### Initial Assessment (file absent)

1. Read project context (CLAUDE.md, README, docs/)
2. Scan GitHub issues/PRs via get-hubbed MCP
3. Analyze recent git history via grit MCP
4. Ask user clarifying questions about vision and priorities
5. Synthesize top user stories ranked by priority
6. Write `docs/user-stories.toml`

### Update (file present)

1. Read existing `docs/user-stories.toml`
2. Re-scan codebase, issues, git history for changes
3. Update story statuses with evidence (commits, files, PRs)
4. Surface newly discovered stories
5. Present changelog-style diff to user for confirmation
6. Write updated `docs/user-stories.toml`

## TOML Schema

```toml
[meta]
project = "example"
repo = "owner/example"
last_updated = 2026-02-19

[[stories]]
id = "kebab-case-id"
title = "Human-readable story name"
priority = 1
status = "done"  # done | partial | not-started | blocked
evidence = ["path/to/file.go", "https://github.com/owner/repo/issues/1"]
acceptance = ["Criterion one", "Criterion two"]
blockers = ["Optional blocking concern"]
```

## Constraints

- Maximum 10-15 stories to force prioritization
- Status changes must be evidence-based
- Does not plan implementation (that is writing-plans)
- Does not create issues or modify code
- Does not make architectural decisions
- Asks one question at a time, prefers multiple-choice

## Data Sources

- Codebase and docs (Read, Glob, Grep)
- GitHub issues/PRs (get-hubbed MCP)
- Git history (grit MCP)
- User conversation (AskUserQuestion)

## Packaging

- Location: `skills/voldemort/SKILL.md` in purse-first repo
- Registration: add `"./skills/voldemort"` to `.claude-plugin/plugin.json`
- Discovery: automatic via `generate-local-plugin`
