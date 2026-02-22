# Design: `tap-dancer go-test` Subcommand

## Summary

Add a `tap-dancer go-test` subcommand that runs `go test -json` and converts
the output to TAP-14. This requires extending the Go Writer with subtest
support.

## Motivation

There is no existing `go test` to TAP-14 formatter. The few tools that exist
(patter, tapper) are stale and target older TAP versions. Since tap-dancer
already has a Go TAP-14 writer and a CLI with subcommand dispatch via
`command.App`, adding this as a native subcommand is the natural fit.

## Design

### Writer Subtest Support

Add a `Subtest(name string) *Writer` method to `tap.Writer`:

- Returns a new `Writer` that prefixes every emitted line with `depth * 4`
  spaces of indentation
- Emits the indented `# Subtest: name` comment before the child's first output
- The child has its own independent test counter starting at 0
- The parent does not auto-increment — the caller emits the parent's summary
  test point (`Ok`/`NotOk`) after the child's `Plan()`
- Internally, the child writes to a wrapper around the parent's `io.Writer`
  that prepends indentation

### `go-test` Subcommand

Registered as a `command.Command` in `registerCommands()`.

**Usage:**

```
tap-dancer go-test [flags] [go test args...]
```

**Flags:**

- `-v` — passes `-v` to `go test`; includes output diagnostics for passing
  tests (not just failures)

**Behavior:**

1. Spawns `go test -json [user args...]` as a subprocess
2. Reads the JSON event stream line-by-line from stdout
3. Buffers events per package to avoid interleaved output from parallel packages
4. Emits TAP-14 output atomically per package when each package completes

### TAP-14 Output Structure

One top-level subtest per package:

```tap
TAP version 14
    # Subtest: github.com/foo/bar
    ok 1 - TestSimple
    not ok 2 - TestBroken
      ---
      message: |
        bar_test.go:42: expected 1, got 2
      elapsed: 0.003
      package: github.com/foo/bar
      file: bar_test.go
      line: "42"
      ...
    1..2
not ok 1 - github.com/foo/bar
    # Subtest: github.com/foo/baz
    ok 1 - TestOther
    1..1
ok 2 - github.com/foo/baz
1..2
```

Go subtests (`TestFoo/sub1/sub2`) map to nested TAP-14 subtest blocks. Each
`/`-delimited level becomes a deeper indentation level.

### Build Failures

If a package has `FailedBuild` set, emit `Bail out!` inside that package's
subtest block but continue processing other packages.

### YAML Diagnostics

- **Failing tests:** Always include `message` (captured output), `elapsed`,
  `package`, and `file`/`line` (parsed from output lines matching
  `filename.go:NN:` patterns)
- **Passing tests with `-v`:** Include `message` (captured output) and
  `elapsed`
- **Passing tests without `-v`:** No YAML block

### Event Processing Model

`go test -json` emits a `TestEvent` per line:

```go
type TestEvent struct {
    Time    time.Time
    Action  string    // run, pause, cont, pass, fail, output, skip, start
    Package string
    Test    string
    Elapsed float64
    Output  string
}
```

The converter:

1. Accumulates `output` events per `(Package, Test)` key into a string buffer
2. On `run` events, registers the test in a per-package tree
3. On terminal events (`pass`/`fail`/`skip`), marks the test result with
   elapsed time and accumulated output
4. On package-level terminal events (where `Test` is empty), flushes that
   package's entire subtest block to stdout
5. Output is buffered per package and emitted atomically when the package
   completes — avoiding interleaved TAP from parallel packages

### Error Handling

- If `go test` fails to start (binary not found), bail out immediately
- If JSON parsing fails on a line, emit it as a TAP comment and continue
- Stderr from `go test` is passed through to stderr (not captured into TAP)

### Exit Code

Mirror `go test`'s exit code: 0 if all pass, 1 if any fail, 2 if build errors.

### Testing

- Unit tests for the Writer's `Subtest()` method
- Unit tests for the event-to-TAP conversion logic using canned JSON input
- Integration test in the justfile: `tap-dancer go-test ./...` run against a
  small test fixture package, output piped to `tap-dancer validate` to confirm
  valid TAP-14
