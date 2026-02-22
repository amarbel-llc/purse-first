# Stderr Truncation Across the purse-first Stack

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add stderr truncation to the purse-first framework and all downstream Go MCP servers so external command stderr never inflates tool responses beyond token limits.

**Architecture:** Add a `LimitStderr()` convenience function to the go-lib-mcp `output` package, update the context-saving skill documentation with a new "Pattern 3: Stderr Truncation" section, and apply the helper in grit, get-hubbed, and lux where stderr is currently passed through unbounded.

**Tech Stack:** Go (go-lib-mcp, grit, get-hubbed, lux), Markdown (purse-first skills)

---

### Task 1: Add LimitStderr to go-lib-mcp output package

**Files:**
- Modify: `/Users/sfriedenberg/eng/repos/purse-first/libs/go-mcp/output/text.go` (append after line 167)
- Create: (no new files — tests go in existing test file)
- Test: `/Users/sfriedenberg/eng/repos/purse-first/libs/go-mcp/output/text_test.go`

**Step 1: Write failing tests**

Append to `/Users/sfriedenberg/eng/repos/purse-first/libs/go-mcp/output/text_test.go`:

```go
func TestLimitStderrSmallInput(t *testing.T) {
	result := LimitStderr("some warning\n")
	if result.Truncated {
		t.Fatal("small stderr should not be truncated")
	}

	if result.Content != "some warning\n" {
		t.Fatalf("expected original content, got %q", result.Content)
	}

	if result.TruncationInfo != nil {
		t.Fatal("expected nil TruncationInfo when not truncated")
	}
}

func TestLimitStderrEmptyInput(t *testing.T) {
	result := LimitStderr("")
	if result.Truncated {
		t.Fatal("empty stderr should not be truncated")
	}

	if result.Content != "" {
		t.Fatalf("expected empty content, got %q", result.Content)
	}
}

func TestLimitStderrLargeInput(t *testing.T) {
	// Build stderr larger than 100KB default
	line := strings.Repeat("x", 99) + "\n" // 100 bytes per line
	input := strings.Repeat(line, 1500)     // 150,000 bytes

	result := LimitStderr(input)
	if !result.Truncated {
		t.Fatal("expected truncation for 150KB stderr")
	}

	if len(result.Content) > 100_000 {
		t.Fatalf("expected content <= 100KB, got %d bytes", len(result.Content))
	}

	if result.TruncationInfo == nil {
		t.Fatal("expected TruncationInfo when truncated")
	}

	if result.TruncationInfo.OriginalBytes != 150_000 {
		t.Fatalf("expected OriginalBytes=150000, got %d", result.TruncationInfo.OriginalBytes)
	}
}

func TestLimitStderrUsesMaxBytesOnly(t *testing.T) {
	// Verify LimitStderr uses MaxBytes but not Head/Tail/MaxLines
	// A 50-line input under 100KB should not be truncated even though
	// StandardDefaults().MaxLines is 2000
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "stderr line"
	}
	input := strings.Join(lines, "\n")

	result := LimitStderr(input)
	if result.Truncated {
		t.Fatal("input under MaxBytes should not be truncated")
	}

	if result.Content != input {
		t.Fatalf("expected original content preserved")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/libs/go-mcp && just test`
Expected: FAIL — `LimitStderr` undefined

**Step 3: Write LimitStderr implementation**

Append to `/Users/sfriedenberg/eng/repos/purse-first/libs/go-mcp/output/text.go` after the `truncateUTF8` function (after line 167):

```go
// LimitStderr applies default max_bytes truncation to stderr output.
// Use this for stderr from external commands before including it in tool results.
// Stderr is never caller-controllable, so defaults are always applied.
func LimitStderr(stderr string) LimitedText {
	defaults := StandardDefaults()
	return LimitText(stderr, TextLimits{MaxBytes: defaults.MaxBytes})
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/libs/go-mcp && just test`
Expected: PASS — all existing tests + 4 new tests

**Step 5: Commit**

```bash
cd /Users/sfriedenberg/eng/repos/purse-first/libs/go-mcp
git add output/text.go output/text_test.go
git commit -m "feat(output): add LimitStderr convenience function

Applies default max_bytes (100KB) truncation to stderr from external
commands. Stderr is never caller-controllable, so it needs automatic
default-on truncation rather than opt-in parameters."
```

---

### Task 2: Update context-saving skill with Pattern 3

**Files:**
- Modify: `/Users/sfriedenberg/eng/repos/purse-first/skills/context-saving/SKILL.md`

**Step 1: Add Pattern 3 section after Pattern 2**

Insert after the "Where to Apply" list under Pattern 2 (after line 112) and before the "Decision Checklist" section (line 114):

```markdown
## Pattern 3: Stderr Truncation (Command Output)

Every tool that executes an external command captures stderr. This stderr is
**never caller-controllable** — unlike stdout, there are no parameters the
caller can set to limit it. Commands like `nix search --json` emit per-package
evaluation progress on stderr (~8.5M characters for nixpkgs), inflating tool
responses far beyond token limits.

Stderr requires **automatic, default-on truncation** via a convenience function.
This is fundamentally different from Patterns 1 and 2, which are opt-in via
caller parameters.

### When to Apply

**Every tool that shells out to an external command.** This includes tools whose
primary output is "naturally bounded" (hash computation, status checks, single
results). The primary output may be small, but stderr is always unbounded.

### Implementation

Use the `LimitStderr` convenience function from the `output` package:

```go
result, err := exec.RunCommand(ctx, args...)
limited := output.LimitStderr(result.Stderr)

return &ToolResult{
    Output:         result.Stdout,
    Stderr:         limited.Content,
    Truncated:      limited.Truncated,
    TruncationInfo: limited.TruncationInfo,
}, nil
```

For Rust, use `limit_stderr()` from the `output` module (same 100KB default).

### Important: Inspect Before Truncating

When a tool needs to inspect stderr before truncation (e.g., checking for
authentication status or specific error patterns), read it first:

```go
isAuthed := strings.Contains(result.Stderr, "authenticated")
limited := output.LimitStderr(result.Stderr)
```

### Combined Truncation

When both stdout and stderr are independently truncated, combine the signals:

```go
limitedStdout := output.LimitText(result.Stdout, limits)
limitedStderr := output.LimitStderr(result.Stderr)
truncated := limitedStdout.Truncated || limitedStderr.Truncated
```
```

**Step 2: Update the Decision Checklist**

Replace the existing Decision Checklist table (lines 114-123) with:

```markdown
## Decision Checklist

For each tool, determine context-saving applicability:

| Output Type | Primary Output | Stderr | Example Tools |
|-------------|---------------|--------|---------------|
| `Vec<T>` or JSON array | Pagination | Truncate | store_ls, search, diagnostics, completions, list APIs |
| Text or JSON object/blob | Truncation | Truncate | build logs, eval, flake show/metadata, check output |
| Single scalar value | None needed | Truncate | hash, status, resolve |
| User-initiated output | None needed | Truncate | run, develop_run (user controls the command) |

**Note:** The "Stderr" column applies to every tool that executes an external
command. Even tools with naturally bounded primary output can produce unbounded
stderr.
```

**Step 3: Update the Implementation Checklist**

Add step 7 after the existing 6 steps (line 134):

```markdown
7. Apply `LimitStderr()` to stderr from any external command before including
   it in the result
```

**Step 4: Commit**

```bash
cd /Users/sfriedenberg/eng/repos/purse-first
git add skills/context-saving/SKILL.md
git commit -m "docs(context-saving): add Pattern 3 for stderr truncation

Stderr is a hidden, unbounded channel in every tool that shells out
to an external command. Unlike stdout, it's never caller-controllable
and requires automatic default-on truncation."
```

---

### Task 3: Update implementation-patterns.md with stderr examples

**Files:**
- Modify: `/Users/sfriedenberg/eng/repos/purse-first/skills/context-saving/references/implementation-patterns.md`

**Step 1: Add stderr truncation section**

Append after the "Full Audit Results from nix-mcp" section (after line 315):

````markdown
---

## Stderr Truncation Pattern

### The Problem

Every tool that shells out to an external command captures stderr. This stderr
is returned verbatim — unbounded. In nix-mcp, `nix search --json` produced
~8.5M characters of per-package evaluation progress on stderr, inflating the
tool response far beyond token limits.

The existing pagination and truncation patterns only address the tool's primary
output (stdout). Stderr is a separate, hidden channel that must be truncated
independently.

### Before (unbounded stderr)

```go
func Run(ctx context.Context, args ...string) (string, error) {
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("command failed: %w: %s", err, stderr.String())
    }

    return stdout.String(), nil
}
```

### After (truncated stderr)

```go
import "github.com/amarbel-llc/go-lib-mcp/output"

func Run(ctx context.Context, args ...string) (string, output.LimitedText, error) {
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        limited := output.LimitStderr(stderr.String())
        return "", limited, fmt.Errorf("command failed: %w: %s", err, limited.Content)
    }

    limited := output.LimitStderr(stderr.String())
    return stdout.String(), limited, nil
}
```

### Key details

- `LimitStderr()` applies only `MaxBytes` (100KB) — no head/tail/max_lines
- Stderr is never caller-controllable, so no tool parameters are needed
- Apply before embedding stderr in error messages or result structs
- When inspecting stderr before truncation (e.g., auth checks), read first, then truncate

### Rust equivalent (from nix-mcp/chix)

```rust
use crate::output::limit_stderr;

let limited_stderr = limit_stderr(&result.stderr);

Ok(ToolResult {
    success: result.success,
    output: result.stdout,
    stderr: limited_stderr.content,
    truncated: if limited_stderr.truncated { Some(true) } else { None },
    truncation_info: limited_stderr.truncation_info,
})
```

### Combined stdout + stderr truncation

When a tool truncates both stdout and stderr independently, combine the signals
in the result:

```go
limitedStdout := output.LimitText(result.Stdout, limits)
limitedStderr := output.LimitStderr(result.Stderr)
truncated := limitedStdout.Truncated || limitedStderr.Truncated

return &ToolResult{
    Output:         limitedStdout.Content,
    Stderr:         limitedStderr.Content,
    Truncated:      truncated,
    TruncationInfo: limitedStdout.TruncationInfo, // prefer stdout info
}
```

### Updated audit: Stderr treatment

**All tools that shell out to external commands** should truncate stderr,
regardless of their primary output pattern:

| Tool Category | Primary Output | Stderr Treatment |
|---------------|---------------|-----------------|
| Pagination tools (search, ls, diagnostics) | Paginated | `LimitStderr()` |
| Truncation tools (build, eval, logs) | Truncated | `LimitStderr()` |
| Scalar tools (hash, status, resolve) | None needed | `LimitStderr()` |
| User-initiated (run, develop_run) | None needed | `LimitStderr()` |
| Pure computation (no external command) | N/A | N/A |
````

**Step 2: Commit**

```bash
cd /Users/sfriedenberg/eng/repos/purse-first
git add skills/context-saving/references/implementation-patterns.md
git commit -m "docs(context-saving): add stderr truncation examples to implementation patterns"
```

---

### Task 4: Apply LimitStderr in grit

**Files:**
- Modify: `/Users/sfriedenberg/eng/repos/grit/internal/git/exec.go`

**Step 1: Read the current file**

Read `/Users/sfriedenberg/eng/repos/grit/internal/git/exec.go` to confirm current state.

**Step 2: Update Run function to truncate stderr**

The current `Run` function (lines 12-39) embeds raw `stderr.String()` in error
messages and discards it on success. Update to truncate before embedding:

```go
package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/output"
)

func Run(ctx context.Context, dir string, args ...string) (string, error) {
	if strings.ContainsRune(dir, 0) {
		return "", fmt.Errorf("dir contains null byte")
	}

	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return "", fmt.Errorf("argument contains null byte")
		}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_EDITOR=true",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		limited := output.LimitStderr(stderr.String())
		return "", fmt.Errorf("git %v: %w: %s", args, err, limited.Content)
	}

	return stdout.String(), nil
}
```

**Step 3: Update go dependencies if needed**

Run: `cd /Users/sfriedenberg/eng/repos/grit && just deps`

This ensures `go.mod` and `gomod2nix.toml` are in sync after the import change
(go-lib-mcp may already be in go.mod; if so, just `go mod tidy` suffices).

**Step 4: Build and test**

Run: `cd /Users/sfriedenberg/eng/repos/grit && just build && just test`
Expected: Build succeeds, all tests pass.

**Step 5: Commit**

```bash
cd /Users/sfriedenberg/eng/repos/grit
git add internal/git/exec.go go.mod go.sum gomod2nix.toml
git commit -m "fix: truncate stderr in git command execution

Apply output.LimitStderr() to prevent unbounded stderr from inflating
MCP tool responses beyond token limits."
```

---

### Task 5: Apply LimitStderr in get-hubbed

**Files:**
- Modify: `/Users/sfriedenberg/eng/repos/get-hubbed/internal/gh/exec.go`

**Step 1: Read the current file**

Read `/Users/sfriedenberg/eng/repos/get-hubbed/internal/gh/exec.go` to confirm current state.

**Step 2: Update Run function to truncate stderr**

The current `Run` function (lines 10-22) embeds raw `stderr.String()` in error
messages. Update:

```go
package gh

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/output"
)

func Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		limited := output.LimitStderr(stderr.String())
		return "", fmt.Errorf("gh %v: %w: %s", args, err, limited.Content)
	}

	return stdout.String(), nil
}
```

**Step 3: Update go dependencies if needed**

Run: `cd /Users/sfriedenberg/eng/repos/get-hubbed && just deps`

**Step 4: Build and test**

Run: `cd /Users/sfriedenberg/eng/repos/get-hubbed && just build && just test`
Expected: Build succeeds, all tests pass.

**Step 5: Commit**

```bash
cd /Users/sfriedenberg/eng/repos/get-hubbed
git add internal/gh/exec.go go.mod go.sum gomod2nix.toml
git commit -m "fix: truncate stderr in gh command execution

Apply output.LimitStderr() to prevent unbounded stderr from inflating
MCP tool responses beyond token limits."
```

---

### Task 6: Apply LimitStderr in lux

**Files:**
- Modify: `/Users/sfriedenberg/eng/repos/lux/internal/formatter/executor.go`

**Step 1: Read the current file**

Read `/Users/sfriedenberg/eng/repos/lux/internal/formatter/executor.go` to confirm current state.

**Step 2: Update formatStdin to truncate stderr**

The `formatStdin` function (lines 59-78) passes `stderr.String()` both into
error messages and the `Result.Stderr` field. Update both:

Replace lines 68-77 with:

```go
	if err := cmd.Run(); err != nil {
		limited := output.LimitStderr(stderr.String())
		return nil, fmt.Errorf("formatter %s failed: %w\nstderr: %s", binPath, err, limited.Content)
	}

	formatted := stdout.String()
	limited := output.LimitStderr(stderr.String())
	return &Result{
		Formatted: formatted,
		Stderr:    limited.Content,
		Changed:   formatted != string(content),
	}, nil
```

**Step 3: Update formatFilepath to truncate stderr**

The `formatFilepath` function (lines 80-115) has the same pattern. Replace
lines 101-113 with:

```go
	if err := cmd.Run(); err != nil {
		limited := output.LimitStderr(stderr.String())
		return nil, fmt.Errorf("formatter %s failed: %w\nstderr: %s", binPath, err, limited.Content)
	}

	formatted, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("reading formatted file: %w", err)
	}

	limited := output.LimitStderr(stderr.String())
	return &Result{
		Formatted: string(formatted),
		Stderr:    limited.Content,
		Changed:   string(formatted) != string(content),
	}, nil
```

**Step 4: Add the import**

Add `"github.com/amarbel-llc/purse-first/libs/go-mcp/output"` to the imports block.

**Step 5: Update go dependencies if needed**

Run: `cd /Users/sfriedenberg/eng/repos/lux && just deps`

**Step 6: Build and test**

Run: `cd /Users/sfriedenberg/eng/repos/lux && just build && just test`
Expected: Build succeeds, all tests pass.

**Step 7: Commit**

```bash
cd /Users/sfriedenberg/eng/repos/lux
git add internal/formatter/executor.go go.mod go.sum gomod2nix.toml
git commit -m "fix: truncate stderr in formatter execution

Apply output.LimitStderr() to formatStdin and formatFilepath to prevent
unbounded formatter stderr from inflating MCP tool responses."
```

---

### Task 7: Commit design doc in chix

**Files:**
- Stage: `/Users/sfriedenberg/eng/repos/chix/docs/plans/2026-02-20-stderr-truncation-upstream-design.md`

**Step 1: Commit the design doc**

```bash
cd /Users/sfriedenberg/eng/repos/chix
git add docs/plans/2026-02-20-stderr-truncation-upstream-design.md
git commit -m "docs: add stderr truncation upstream design plan"
```

---

## Execution Order and Dependencies

```
Task 1 (go-lib-mcp: LimitStderr)
  ├── Task 2 (skill: SKILL.md)          [independent of Task 1]
  ├── Task 3 (skill: impl-patterns.md)  [independent of Task 1]
  ├── Task 4 (grit)                     [depends on Task 1]
  ├── Task 5 (get-hubbed)               [depends on Task 1]
  └── Task 6 (lux)                      [depends on Task 1]
Task 7 (chix design doc)                [independent]
```

Tasks 2, 3, and 7 are documentation-only and can run in parallel with each
other and with Task 1. Tasks 4, 5, and 6 depend on Task 1 being committed and
available in go-lib-mcp, but are independent of each other and can run in
parallel.

## Verification

After all tasks complete:

1. `cd /Users/sfriedenberg/eng/repos/purse-first/libs/go-mcp && just test` — all output package tests pass
2. `cd /Users/sfriedenberg/eng/repos/grit && just build && just test` — builds and tests pass
3. `cd /Users/sfriedenberg/eng/repos/get-hubbed && just build && just test` — builds and tests pass
4. `cd /Users/sfriedenberg/eng/repos/lux && just build && just test` — builds and tests pass
