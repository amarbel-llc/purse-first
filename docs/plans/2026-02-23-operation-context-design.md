# Operation Context Design

Structured operational context for go-mcp that carries metadata through nested
operations and streams output to pluggable writers. Domain code uses `Run`,
`ControlFail`, `DiagSet`, etc. and never thinks about output format. Writers
(TAP-14, JSON, structured logs) observe lifecycle events and render
independently.

## Motivation

Three problems converge:

1. **dodder's alfa/errors Context** overuses Go errors for control flow (skip,
   abort, retry), requiring expensive `errors.As` type-switching on every path.
   Panics are cheaper and semantically clearer for exceptional control flow.

2. **tap-dancer's WriteAll API** requires domain code to construct TestPoints
   explicitly. Domain code shouldn't think about output format.

3. **MCP tool handlers** return flat text/JSON results with no structured
   lifecycle, nesting, or operational metadata (idempotency, destructiveness).

## Design Principles

- **Errors are expected failures.** `return err` means "this operation failed."
  Stack info only at dispatch point. No sentinel error types.
- **Panics are exceptional.** Two uses: (1) control flow directives (skip,
  abort) caught by `Run`'s recover, (2) bugs caught at boundary and converted to
  diagnostics with full stack traces.
- **No panic leakage.** Every callback boundary (`Run`, `Must`, `After`) uses
  `callSafe` with `recover`. Unknown panics become `severity: panic` diagnostics.
- **Streaming output.** Writers receive `BeginOperation`/`EndOperation` events as
  operations complete. No collecting-then-rendering.
- **Independent of dodder.** Fresh implementation in go-mcp, but interface is
  similar enough for future dodder migration.

## Package: `libs/go-mcp/operation`

### Outcome and Annotation Types

```go
type Outcome int

const (
    Success Outcome = iota
    Failure
    Skipped
    Aborted
)

type Annotation int

const (
    Idempotent  Annotation = 1 << iota
    Destructive
    Recoverable
    ReadOnly
)
```

### Diagnostic

```go
type Diagnostic struct {
    File     string
    Line     int
    Message  string
    Severity string         // "error", "panic", or custom
    Source   string         // "" for domain, "external" for wrapped
    Extras   map[string]any // from DiagSet calls
}
```

### Context Interface

```go
type Context interface {
    // No context.Context embedding — distinct concepts.

    Run(description string, fn func(Context) error, annotations ...Annotation) error

    // Control flow — all noreturn (panic internally, return error for compiler)

    // Domain failures — file:line captured via runtime.Caller at call site
    ControlFail(msg string) error
    ControlFailf(format string, args ...any) error

    // External error wrapping — file:line at wrap site, Source: "external"
    ControlWrap(err error) error
    ControlWrapf(err error, format string, args ...any) error

    // Skip — operation not applicable
    ControlSkip(reason string) error
    ControlSkipf(format string, args ...any) error

    // Abort — cancel entire context tree
    ControlAbort(err error) error

    // Diagnostics — metadata on any outcome (including success)
    DiagSet(key string, value any)
    DiagHelper() // marks calling function for frame-skipping (like t.Helper)

    // Cleanup — scoped to the Run that registers them, LIFO order
    After(fn func() error) // best-effort, errors swallowed
    Must(fn func() error)  // required — failure turns success into failure
}
```

### Writer Interface

```go
type Writer interface {
    BeginOperation(depth int, op *OperationEvent)
    EndOperation(depth int, op *OperationEvent)
}

type OperationEvent struct {
    Description string
    Annotations []Annotation
    Outcome     Outcome
    Diagnostic  *Diagnostic // nil if success with no DiagSet calls
    MustErrors  []error     // one per failed Must callback
}
```

## Error Model: Three Tiers

### Tier 1: Domain failure — `ControlFail`

Domain code knows what went wrong. File:line captured automatically via
`runtime.Caller(1)`.

```go
ctx.Run("validate schema", func(ctx Context) error {
    if conflict {
        return ctx.ControlFail("schema conflict on users table")
    }
    return nil
})
```

TAP output:

```yaml
  ---
  file: internal/migrate/validate.go
  line: 47
  message: schema conflict on users table
  severity: error
  ...
```

### Tier 2: External error — `ControlWrap`

Error from a library or external system. File:line captured at the wrap site.
Marked `source: external` to indicate limited diagnostic data.

```go
ctx.Run("apply migration", func(ctx Context) error {
    if err := db.Exec(query); err != nil {
        return ctx.ControlWrap(err)
    }
    return nil
}, Destructive, Idempotent)
```

TAP output:

```yaml
  ---
  file: internal/migrate/runner.go
  line: 52
  message: "pq: relation \"users\" already exists"
  severity: error
  source: external
  ...
```

`ControlWrapf` adds context:

```go
return ctx.ControlWrapf(err, "migration %s", m.Name)
// message: "migration 002_add_index: pq: relation already exists"
```

### Tier 3: Plain `return err`

Bare error return. No file:line (the error likely originated deep in a call
stack). Error message becomes the diagnostic message.

```go
ctx.Run("step", func(ctx Context) error {
    return someFunction() // plain error, minimal diagnostics
})
```

TAP output:

```yaml
  ---
  message: "connection refused"
  severity: error
  ...
```

## Control Flow via Panic

All `Control*` methods panic internally. `Run` recovers and handles them. Domain
code writes `return ctx.ControlFail(...)` for compiler satisfaction — the panic
fires before the return executes.

Internal sentinel types:

```go
type failSentinel struct{ diag Diagnostic }
type skipSentinel struct{ diag Diagnostic }
type abortSentinel struct{ err error }
```

## Run Semantics

```go
func (c *ctx) Run(
    description string,
    fn func(Context) error,
    annotations ...Annotation,
) (retErr error) {
    child := c.child(description, annotations)
    c.writer.BeginOperation(child.depth, child.event())

    defer func() {
        // 1. Must callbacks — failure turns success into failure
        child.runMust()
        // 2. After callbacks — best-effort, errors swallowed
        child.runAfter()
        // 3. Emit end event
        c.writer.EndOperation(child.depth, child.event())
    }()

    defer func() {
        if r := recover(); r != nil {
            switch v := r.(type) {
            case failSentinel:
                child.outcome = Failure
                child.diagnostic = &v.diag
            case skipSentinel:
                child.outcome = Skipped
                child.diagnostic = &v.diag
            case abortSentinel:
                child.outcome = Aborted
                retErr = v.err
                c.cancel(v.err)
            default:
                // Bug — capture with full stack, no leakage
                child.outcome = Failure
                child.diagnostic = &Diagnostic{
                    Message:  fmt.Sprintf("panic: %v", r),
                    Severity: "panic",
                    Extras:   map[string]any{"stack": string(debug.Stack())},
                }
            }
        }
    }()

    retErr = fn(child)
    if retErr != nil {
        child.outcome = Failure
        child.diagnostic = &Diagnostic{
            Message:  retErr.Error(),
            Severity: "error",
        }
    } else if child.outcome == 0 {
        child.outcome = Success
    }

    return retErr
}
```

## Panic-Safe Callbacks

Every callback boundary recovers panics. No exceptions.

```go
func (c *ctx) callSafe(fn func() error) (retErr error) {
    defer func() {
        if r := recover(); r != nil {
            retErr = fmt.Errorf("panic in callback: %v\n%s",
                r, debug.Stack())
        }
    }()
    return fn()
}

func (c *ctx) runMust() {
    for i := len(c.musts) - 1; i >= 0; i-- {
        if err := c.callSafe(c.musts[i]); err != nil {
            c.event.MustErrors = append(c.event.MustErrors, err)
            if c.outcome == Success {
                c.outcome = Failure
            }
        }
    }
}

func (c *ctx) runAfter() {
    for i := len(c.afters) - 1; i >= 0; i-- {
        _ = c.callSafe(c.afters[i])
    }
}
```

## Must vs After

Both are cleanup callbacks scoped to the `Run` that registers them, executed
LIFO.

| Aspect | `After` | `Must` |
|--------|---------|--------|
| Execution | Always runs | Always runs |
| On callback error | Swallowed | Added to `MustErrors`, may flip Success → Failure |
| On callback panic | Recovered, swallowed | Recovered, added to `MustErrors` |
| Use case | Best-effort cleanup (temp files, logging) | Required cleanup (lock release, flush) |

Must is registered at the scope where it matters:

```go
ctx.Run("deploy", func(ctx Context) error {
    var lock Lock

    ctx.Run("acquire lock", func(ctx Context) error {
        if err := lock.Acquire(); err != nil {
            return ctx.ControlWrap(err)
        }
        return nil
    }, Idempotent)

    // Must on parent — survives sibling failures
    ctx.Must(func() error { return lock.Release() })

    ctx.Run("run migrations", func(ctx Context) error {
        return ctx.ControlAbort(errors.New("fatal"))
    })

    return nil
})
```

## DiagHelper — Frame Skipping

Borrowed from `testing.T.Helper()`. When a helper function calls
`ctx.DiagHelper()`, subsequent `ControlFail`/`ControlWrap` calls skip that
frame, attributing file:line to the caller of the helper.

```go
func assertNonEmpty(ctx Context, s string) {
    ctx.DiagHelper()
    if s == "" {
        ctx.ControlFail("expected non-empty string")
        // file:line points to the caller of assertNonEmpty, not here
    }
}
```

## TAP Writer

The TAP writer implements `operation.Writer` and maps operation events to TAP-14
output using tap-dancer's existing `Writer`:

| Operation concept | TAP-14 output |
|-------------------|---------------|
| `Run` with children | `# Subtest: description`, indented children, trailing plan |
| `Run` leaf, Success | `ok N - description` |
| `Run` leaf, Failure | `not ok N - description` + YAML diagnostics |
| `Run` leaf, Skipped | `ok N - description # SKIP reason` |
| Annotations | Keys in YAML diagnostics (e.g., `idempotent: true`) |
| `DiagSet` extras | Keys in YAML diagnostics |
| `MustErrors` | `must_errors` array in YAML diagnostics |
| `source: external` | `source: external` in YAML diagnostics |
| `severity: panic` | `severity: panic` with `stack` in YAML diagnostics |

### Example TAP output

```tap
TAP version 14
    # Subtest: deploy
    ok 1 - acquire lock
    ok 2 - backup database
      ---
      size_mb: 420
      ...
        # Subtest: apply migrations
        ok 1 - 001_create_users
          ---
          applied_at: 2026-02-23T10:30:00Z
          ...
        not ok 2 - 002_add_index
          ---
          file: internal/migrate/runner.go
          line: 87
          message: "migration 002_add_index: pq: relation already exists"
          severity: error
          source: external
          ...
        1..2
    not ok 3 - apply migrations
    1..3
not ok 1 - deploy
1..1
```

## Other Writers

The `Writer` interface is format-agnostic. Other implementations:

- **JSON writer** — emits NDJSON with full OperationEvent per line
- **Structured log writer** — emits slog records with operation fields
- **MCP progress writer** — emits MCP progress notifications for long-running
  tool calls
- **Null writer** — discards events (for testing or silent execution)

## MCP Integration

Tool handlers receive an `operation.Context`. The framework attaches a writer and
collects output as the tool result:

```go
Run: func(ctx operation.Context, args map[string]string) error {
    ctx.Run("validate", func(ctx Context) error { ... }, ReadOnly)
    ctx.Run("execute", func(ctx Context) error { ... }, Destructive)
    return nil
}
```

The MCP handler wraps the writer's output into a `ToolCallResult` automatically.
Domain code never constructs result types.

## Relationship to Existing Code

- **dodder alfa/errors Context**: Independent implementation. Similar interface
  (`Run`, `After`, `Must`, lifecycle states). Key difference: panics for control
  flow instead of error type-switching. Designed for future dodder migration.
- **tap-dancer WriteAll**: TAP writer uses tap-dancer's `Writer` internally.
  `WriteAll` and `TestPoint` remain available for direct use. The operation
  context is a higher-level abstraction that feeds into tap-dancer.
- **go-mcp command.Result**: Operation context replaces the `Result` return
  pattern for tool handlers that need structured operational output. Simple tools
  can still return `Result` directly.
