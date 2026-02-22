---
name: TAP-14 Output
description: This skill should be used when the user asks to "format output as TAP", "add TAP output", "produce TAP-14", "write TAP test results", "use tap-dancer", or mentions TAP-14, TAP version 14, TAP output format, TAP writer, or test result formatting.
version: 0.1.0
---

# TAP-14 Output with tap-dancer

This skill provides guidance for producing spec-compliant TAP version 14 output using the tap-dancer libraries. TAP-14 is the standard text-based protocol for communicating test results between test modules and harnesses.

## When to Use TAP-14

Use TAP-14 output when building:
- CLI tools that report step-by-step results (build systems, installers, validators)
- Test runners or harnesses
- Recipe executors (justfile runners, task runners)
- Any tool where structured pass/fail output is consumed by other programs

## Go Library

Import: `github.com/amarbel-llc/tap-dancer/go`

### Basic Usage

```go
import (
    "os"
    tap "github.com/amarbel-llc/tap-dancer/go"
)

func main() {
    tw := tap.NewWriter(os.Stdout)  // Emits: TAP version 14
    tw.PlanAhead(3)                  // Emits: 1..3

    tw.Ok("database connected")      // Emits: ok 1 - database connected
    tw.Ok("schema validated")        // Emits: ok 2 - schema validated
    tw.NotOk("migration failed", map[string]string{
        "message":  "column 'name' already exists",
        "severity": "fail",
    })
    // Emits:
    // not ok 3 - migration failed
    //   ---
    //   message: column 'name' already exists
    //   severity: fail
    //   ...
}
```

### API Reference

| Method | Output | Returns |
|--------|--------|---------|
| `NewWriter(w)` | `TAP version 14` | `*Writer` |
| `Ok(desc)` | `ok N - desc` | test number |
| `NotOk(desc, diag)` | `not ok N - desc` + optional YAML block | test number |
| `Skip(desc, reason)` | `ok N - desc # SKIP reason` | test number |
| `Todo(desc, reason)` | `not ok N - desc # TODO reason` | test number |
| `PlanAhead(n)` | `1..n` (before tests) | — |
| `Plan()` | `1..n` (after tests, n = count) | — |
| `BailOut(reason)` | `Bail out! reason` | — |
| `Comment(text)` | `# text` | — |

### YAML Diagnostics

Pass a `map[string]string` to `NotOk` for structured failure info. Keys are sorted alphabetically. Multiline values automatically use YAML block scalar (`|`) format:

```go
tw.NotOk("compile failed", map[string]string{
    "exitcode": "1",
    "message":  "syntax error on line 42",
    "output":   "error: unexpected token\n  at main.go:42:5",
})
```

Pass `nil` to omit the YAML block entirely.

### Trailing Plan

When the total test count is unknown upfront, emit the plan after all tests:

```go
tw := tap.NewWriter(os.Stdout)
tw.Ok("step one")
tw.Ok("step two")
tw.Plan()  // Emits: 1..2
```

## Rust Library

Crate: `tap-dancer`

### Basic Usage

```rust
use std::io;
use tap_dancer::{write_version, write_plan, write_test_point, TestResult, TapWriter};

fn main() -> io::Result<()> {
    let stdout = &mut io::stdout();
    let mut tw = TapWriter::new();

    write_version(stdout)?;
    write_plan(stdout, 3)?;

    let n = tw.next();
    write_test_point(stdout, &TestResult {
        number: n,
        name: "database connected".into(),
        ok: true,
        error_message: None,
        exit_code: None,
        output: None,
    })?;

    let n = tw.next();
    write_test_point(stdout, &TestResult {
        number: n,
        name: "migration failed".into(),
        ok: false,
        error_message: Some("column already exists".into()),
        exit_code: Some(1),
        output: None,
    })?;

    let n = tw.next();
    tap_dancer::write_skip(stdout, n, "optional feature", "not supported")?;

    Ok(())
}
```

### API Reference

| Function | Output |
|----------|--------|
| `write_version(w)` | `TAP version 14` |
| `write_plan(w, n)` | `1..n` |
| `write_test_point(w, result)` | `ok/not ok N - name` + optional YAML |
| `write_skip(w, n, desc, reason)` | `ok N - desc # SKIP reason` |
| `write_todo(w, n, desc, reason)` | `not ok N - desc # TODO reason` |
| `write_bail_out(w, reason)` | `Bail out! reason` |
| `write_comment(w, text)` | `# text` |

### TapWriter Counter

Use `TapWriter` for sequential numbering:

```rust
let mut tw = TapWriter::new();
let n = tw.next();  // 1
let n = tw.next();  // 2
tw.count()          // 2
```

### YAML Diagnostics in TestResult

Set fields on `TestResult` to generate YAML diagnostic blocks:
- `error_message` → `message: "..."` (quoted)
- `exit_code` → `exitcode: N`
- `output` → `output: |` (block scalar for multiline)
- Failing tests (`ok: false`) automatically get `severity: fail`
- Passing tests with `output` set also get a YAML block

## TAP-14 Quick Reference

```
TAP version 14
1..N                          # Plan (before or after tests)
ok 1 - description            # Passing test
not ok 2 - description        # Failing test
  ---                         # YAML diagnostic start
  message: "error text"       # Diagnostic field
  severity: fail              # Severity indicator
  exitcode: 1                 # Exit code
  output: |                   # Multiline output
    line one
    line two
  ...                         # YAML diagnostic end
ok 3 - desc # SKIP reason    # Skipped test
not ok 4 - desc # TODO reason # Todo test
Bail out! reason              # Emergency halt
# comment text                # Comment
```

For the complete TAP-14 specification including subtests, pragmas, escaping, and parsing rules, see `references/tap14-spec.md`.
