# Stderr Truncation Across the purse-first Stack

**Date:** 2026-02-20
**Status:** Proposed

## Problem

Every MCP tool that shells out to an external command (nix, git, gh, formatters)
captures stderr and returns it in the tool response. This stderr is unbounded.
In chix, `nix search --json` produced ~8.5M characters of stderr
(per-package evaluation progress), inflating the response far beyond token
limits.

The existing context-saving skill defines two patterns — pagination (arrays) and
truncation (text/JSON blobs) — but both target the tool's **primary output**
(stdout). Stderr is ignored entirely.

The decision checklist marks tools like `hash_path`, `cachix_status`, and
`fh_resolve` as "None needed" because their stdout is bounded. But their stderr
is not. Any external command can produce unbounded stderr (warnings, progress
messages, deprecation notices, evaluation traces).

### Key insight

Stderr is fundamentally different from stdout:
- **Stdout** is caller-controllable via head/tail/max_bytes/offset/limit
- **Stderr** is never caller-controllable — it's a byproduct of command execution
- Stderr needs **automatic, default-on truncation**, not opt-in parameters

## Scope

- Add `LimitStderr()` helper to go-lib-mcp `output` package
- Update the context-saving skill with Pattern 3: Stderr Truncation
- Update implementation-patterns.md with before/after examples
- Apply stderr truncation in downstream Go MCP servers (grit, get-hubbed, lux)
- Future TODO: framework-level default truncation (not in scope)

## Design

### Layer 1: go-lib-mcp `output` package

Add `LimitStderr` function to `output/text.go`:

```go
// LimitStderr applies default max_bytes truncation to stderr output.
// Use this for stderr from external commands before including it in tool results.
// Stderr is never caller-controllable, so defaults are always applied.
func LimitStderr(stderr string) LimitedText {
    defaults := StandardDefaults()
    return LimitText(stderr, TextLimits{MaxBytes: defaults.MaxBytes})
}
```

Only `MaxBytes` (100KB) is applied — no head/tail/max_lines. Stderr is
unstructured and line-based control adds no value for this use case.

### Layer 2: context-saving skill (SKILL.md)

Add "Pattern 3: Stderr Truncation (Command Output)" section after Pattern 2.

Key points to document:
1. Stderr is a hidden, unbounded channel in every tool that shells out
2. Unlike stdout, stderr is not caller-controllable — it needs automatic defaults
3. Every tool that executes an external command must truncate stderr
4. The convenience helper makes this a one-liner
5. Stderr truncation applies to ALL tools that shell out, even those marked
   "naturally bounded" for their primary output

Update the Decision Checklist table to add a "Stderr" column:

| Output Type | Primary Output | Stderr | Example Tools |
|-------------|---------------|--------|---------------|
| Vec/Array | Pagination | Truncate | search, diagnostics |
| Text/JSON blob | Truncation | Truncate | build, eval, flake show |
| Single scalar | None needed | Truncate | hash, status, resolve |
| User-initiated | None needed | Truncate | run, develop_run |

Update the Implementation Checklist to include stderr:

> 7. Apply `LimitStderr()` to stderr from any external command before including
>    it in the result

### Layer 3: implementation-patterns.md

Add a section showing the stderr pattern with before/after examples.

**Before (unbounded stderr):**
```go
result, err := exec.RunCommand(ctx, args...)
if err != nil {
    return nil, fmt.Errorf("command failed: %s", result.Stderr)
}
return &ToolResult{
    Output: result.Stdout,
    Stderr: result.Stderr,  // unbounded
}, nil
```

**After (truncated stderr):**
```go
result, err := exec.RunCommand(ctx, args...)
limited := output.LimitStderr(result.Stderr)
if err != nil {
    return nil, fmt.Errorf("command failed: %s", limited.Content)
}
return &ToolResult{
    Output:         result.Stdout,
    Stderr:         limited.Content,
    Truncated:      limited.Truncated,
    TruncationInfo: limited.TruncationInfo,
}, nil
```

**Combined truncation (stdout + stderr):**
```go
limitedStdout := output.LimitText(result.Stdout, limits)
limitedStderr := output.LimitStderr(result.Stderr)
truncated := limitedStdout.Truncated || limitedStderr.Truncated
```

**Important pattern:** When a tool needs to inspect stderr before truncation
(e.g., checking for authentication status), read it first, then truncate:
```go
isAuthed := strings.Contains(result.Stderr, "authenticated")
limited := output.LimitStderr(result.Stderr)
```

### Layer 4: Downstream Go MCP servers

**grit** (`/Users/sfriedenberg/eng/repos/grit`):
- `internal/git/exec.go` — stderr captured into `bytes.Buffer`, passed
  unbounded into error messages
- Apply `output.LimitStderr()` to `stderr.String()` before embedding in errors
- Audit all tool files in `internal/tools/` for direct stderr passthrough

**get-hubbed** (`/Users/sfriedenberg/eng/repos/get-hubbed`):
- `internal/gh/exec.go` — stderr captured into `bytes.Buffer`, passed
  unbounded into error messages
- Apply `output.LimitStderr()` to `stderr.String()` before embedding in errors

**lux** (`/Users/sfriedenberg/eng/repos/lux`):
- `internal/formatter/executor.go` — stderr from formatters captured into
  `bytes.Buffer`, returned in `Stderr` field
- `internal/subprocess/nix.go` — stderr from nix processes piped through
- Apply `output.LimitStderr()` to captured stderr before returning

## Verification

1. go-lib-mcp: `go test ./output/...` passes with new LimitStderr tests
2. grit: `just build && just test`
3. get-hubbed: `just build && just test`
4. lux: `just build && just test`
5. Manual: invoke a tool that produces large stderr and verify response is under
   100KB

## Future Work (Out of Scope)

- **Framework-level defaults**: Push truncation into the executor/transport
  layer so tools get it automatically without explicit opt-in
- **Configurable stderr limits**: Allow per-tool stderr limits via tool
  registration
- **Automatic envelope**: Include truncation metadata in the MCP response
  envelope rather than per-tool result structs
