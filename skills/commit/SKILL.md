---
name: commit
description: Use when the user asks to commit, requests a commit, says "commit this", or when you need to create a git commit as part of completing work
---

# Commit

## Overview

Use grit's `try_commit` MCP tool for all commits. It stages, commits, and returns status in a single call — replacing the multi-step cycle of status, diff, log, add, commit.

## The Process

### Step 1: Determine What to Commit

Before calling `try_commit`, identify:

1. **Which files to stage** — use grit's `status` tool or your knowledge of what you changed
2. **The commit message** — draft a concise message following the repo's commit style

If unsure which files changed, call grit's `status` tool first.

### Step 2: Draft the Commit Message

Check recent commits for style:

```
grit log (repo_path, max_count: 5)
```

Draft a message that:
- Summarizes the "why" not the "what"
- Follows the repo's existing convention (conventional commits, imperative mood, etc.)
- Ends with `Co-Authored-By: Claude <co-author> <noreply@anthropic.com>` when appropriate

### Step 3: Commit with try_commit

Call `try_commit` with all three required parameters:

```
grit try_commit (
  repo_path: "<path>",
  message: "<commit message>",
  paths: ["file1.go", "file2.go"]
)
```

`try_commit` handles staging, committing, and returns structured results including staged diff stats and post-commit status.

### Step 4: Verify the Result

`try_commit` returns:
- **commit**: parsed commit info (empty if commit failed)
- **staged**: diff numstat of what was staged
- **status**: post-commit porcelain status

If the commit field is empty, the commit failed. Check the status and staged fields for context — likely a pre-commit hook failure. Fix the issue and call `try_commit` again (do NOT fall back to the multi-step cycle).

## Quick Reference

| Step | Tool | Purpose |
|------|------|---------|
| Check changes | `grit status` | See what's modified (optional if you already know) |
| Check style | `grit log` | Match commit message convention (optional) |
| Commit | `grit try_commit` | Stage + commit + verify in one call |

## Common Mistakes

**Falling back to multi-step cycle**
- **Problem:** Using separate status, diff, log, add, commit calls
- **Fix:** Always use `try_commit` — it does all of this in one call

**Using grit's `commit` tool instead of `try_commit`**
- **Problem:** `commit` only commits already-staged files; requires separate `add` call
- **Fix:** `try_commit` stages and commits together

**Using Bash for git commit**
- **Problem:** Hooks will deny `git commit` via Bash in favor of grit
- **Fix:** Use `try_commit` directly

**Omitting paths parameter**
- **Problem:** `try_commit` requires explicit file paths to stage
- **Fix:** List all files that should be included; use `status` first if unsure
