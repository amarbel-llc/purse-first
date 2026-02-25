---
status: accepted
date: 2026-02-25
---

# Use bash AST parsing for shell-aware hook matching

## Context and Problem Statement

PreToolUse hooks matched Bash commands using string prefix matching. This broke for compound commands like `cd /tmp && git status` or `git log | head` because the prefix matcher only saw the first command (e.g., `cd`) and missed the actual tool-mapped command inside the compound expression. Agents frequently compose commands this way, making prefix matching unreliable for hook-based tool routing.

## Considered Options

* Keep string prefix matching
* Parse with mvdan.cc/sh/v3 bash AST parser to extract simple commands
* Use regex matching

## Decision Outcome

Chosen option: "Parse with mvdan.cc/sh/v3 bash AST parser", because it correctly extracts simple commands from compound statements (pipelines, `&&`, `||`, semicolons, subshells) and matches against each one individually, accepting the added dependency on mvdan.cc/sh/v3.

### Consequences

* Good, because compound commands like `cd /foo && git log` now correctly match the `git log` tool mapping.
* Good, because the parser handles all bash compound forms (pipelines, subshells, redirections) without case-by-case regex patterns.
* Good, because on parse failure, it falls back to raw prefix matching, preserving existing behavior for non-bash input.
* Bad, because it adds a new Go dependency (mvdan.cc/sh/v3) to go-mcp, increasing the vendor hash and build closure.
* Neutral, because packages continue declaring `CommandPrefixes` as static strings -- no per-package changes are required.

## More Information

* Design: `docs/plans/2026-02-25-shell-aware-hooks-design.md`
* Implementation scope: `libs/go-mcp/command/shellparse.go` and `HandleHook` in `hook.go`
* Future direction: pivot to delegation-based matching where packages provide custom matchers instead of static prefix strings.
