---
name: Project Story Inventory
description: OPT-IN ONLY — invoke ONLY when user types /voldemort. Never auto-trigger. Generates a docs/user-stories.toml tracking user story completion against the codebase. Does NOT apply to general investigation, analysis, surveying, or auditing tasks.
disable-model-invocation: true
version: 0.1.0
---

# Project Story Inventory

A product-manager skill that identifies the top user stories for a project and tracks their completion status against the codebase.

## Mode Detection

Check for `docs/user-stories.toml` in the project root. If absent, run **initial assessment**. If present, run **update**.

## Checklist

You MUST create a task for each of these items and complete them in order:

1. **Detect mode** — check for `docs/user-stories.toml`
2. **Read project context** — CLAUDE.md, README, docs/ directory
3. **Scan GitHub issues and PRs** — use get-hubbed MCP to list open issues and PRs
4. **Analyze git history** — use grit MCP to review recent commits for direction and velocity
5. **Ask user clarifying questions** — one at a time, multiple-choice preferred
6. **Synthesize or update stories** — rank by user value, cap at 10-15 stories
7. **Present summary for confirmation** — show stories and priorities, get user approval
8. **Write `docs/user-stories.toml`** — commit the file

## Initial Assessment Mode

When `docs/user-stories.toml` does not exist:

1. Read CLAUDE.md and README to understand project intent and scope.
2. Read any existing docs/ for specifications, plans, or design documents.
3. Use get-hubbed MCP (`issue_list`, `pr_list`) to scan open issues and PRs. These represent planned or in-progress work.
4. Use grit MCP (`log`) to analyze recent commits. Identify what areas are actively developed and what direction the project is moving.
5. Ask the user clarifying questions about their vision. Focus on:
   - Who are the primary users?
   - What problem does this project solve?
   - What is the most important thing it does not yet do?
   - Are there stories you already have in mind?
6. Synthesize findings into user stories. Rank by user value, not implementation difficulty.
7. Present the proposed stories with priorities to the user. Ask them to confirm or adjust ordering.
8. Write `docs/user-stories.toml`.

## Update Mode

When `docs/user-stories.toml` already exists:

1. Read the existing TOML file. Parse current stories, statuses, and evidence.
2. Re-scan the codebase for changes since `meta.last_updated`:
   - Use grit MCP (`log`) filtered to commits after the last update date.
   - Use get-hubbed MCP to check for newly opened/closed issues and merged PRs.
   - Read files referenced in `evidence` arrays to verify they still exist and are relevant.
3. Assess each story's status:
   - If acceptance criteria are met (evidence in code, tests, merged PRs), mark `done`.
   - If some criteria are met, mark `partial` and note what remains.
   - If new blockers are discovered, mark `blocked` with explanation.
4. Identify new stories from:
   - Newly opened GitHub issues not covered by existing stories.
   - Code areas that have grown but lack corresponding stories.
   - User input during the conversation.
5. Present a changelog-style diff to the user:
   - Stories with status changes (old status -> new status, with evidence).
   - New stories discovered.
   - Any re-prioritization suggestions.
6. Get user confirmation before writing.
7. Update `docs/user-stories.toml` with new `meta.last_updated`.

## TOML Schema

```toml
[meta]
project = "project-name"
repo = "owner/repo"
last_updated = 2026-02-19

[[stories]]
id = "kebab-case-identifier"
title = "Human-readable story name"
priority = 1
status = "done"
evidence = [
  "path/to/implementation.go",
  "https://github.com/owner/repo/pull/42",
]
acceptance = [
  "First acceptance criterion",
  "Second acceptance criterion",
]

[[stories]]
id = "another-story"
title = "Another user story"
priority = 2
status = "partial"
evidence = ["path/to/file.go"]
acceptance = [
  "Criterion that is done",
  "Criterion that is not yet done",
]
blockers = ["Description of what blocks completion"]
```

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Kebab-case unique identifier |
| `title` | string | yes | Human-readable story name |
| `priority` | integer | yes | 1 = highest priority |
| `status` | string | yes | One of: `done`, `partial`, `not-started`, `blocked` |
| `evidence` | string[] | yes | File paths, issue URLs, PR URLs, or commit SHAs |
| `acceptance` | string[] | yes | Acceptance criteria for the story |
| `blockers` | string[] | no | Blocking concerns (required when status is `blocked`) |

## Constraints

- **Maximum 10-15 stories.** Force prioritization. If more than 15 stories emerge, ask the user to cut or merge.
- **Evidence-based statuses.** Every status claim must cite specific files, commits, issues, or PRs. Do not guess.
- **User-value ranking.** Rank stories by value to the user, not by implementation complexity or developer convenience.
- **One question at a time.** When asking clarifying questions, ask one per message. Prefer multiple-choice options.
- **No implementation planning.** This skill identifies and tracks WHAT to build. Use `writing-plans` for HOW.
- **No code changes.** This skill only reads code and writes `docs/user-stories.toml`.
- **No issue creation.** Do not create GitHub issues. Only read them.
- **No architectural opinions.** Stories describe user-facing outcomes, not technical approaches.
