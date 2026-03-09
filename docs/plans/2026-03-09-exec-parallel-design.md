# exec-parallel command design

## Problem

Parallel command execution with structured output is a recurring pattern (e.g.
`update-repos` in eng/justfile). Each instance reinvents PID tracking, temp-dir
log capture, and result reporting. A tap-dancer command that runs commands in
parallel and emits TAP-14 test points would replace this boilerplate.

## Interface

```
tap-dancer exec-parallel [--verbose] <template> ::: <arg1> <arg2> ...
```

- `--verbose` — include stdout/stderr YAML diagnostics on successful test
  points (default: failures only)
- `<template>` — shell command string; `{}` is replaced with the current
  argument
- `:::` — separator (matches GNU parallel convention)
- `<arg1> ...` — arguments to substitute into the template

## Example

```sh
tap-dancer exec-parallel 'cd {} && git sync' ::: repos/*/
```

Output:

```tap
TAP version 14
ok 1 - cd repos/grit && git sync
not ok 2 - cd repos/lux && git sync
  ---
  exit-code: 1
  stdout: |
    Already up to date.
  stderr: |
    fatal: not a git repository
  ...
ok 3 - cd repos/purse-first && git sync
1..3
```

## Architecture

Two cleanly separated concerns:

### Executor (how commands run)

```go
type ExecResult struct {
    Arg      string
    Command  string
    ExitCode int
    Stdout   []byte
    Stderr   []byte
    Err      error
}

type Executor interface {
    Run(ctx context.Context, template string, args []string) []ExecResult
}
```

The initial implementation (`GoroutineExecutor`) uses goroutines and
`sync.WaitGroup`. A future implementation could forward to GNU `parallel` and
parse its output.

### TAP emitter (how results become test points)

`ConvertExecParallel` takes `[]ExecResult` and writes TAP-14 using the existing
`Writer`. Test point description is the fully substituted command string. YAML
diagnostics on failure include `exit-code`, `stdout`, and `stderr`.

## File layout

- `execparallel.go` — `Executor` interface, `ExecResult`, `GoroutineExecutor`,
  `ConvertExecParallel`
- `main.go` — CLI-only command registration (same pattern as `go-test`,
  `cargo-test`)

## Exit code

0 if all commands succeed, 1 if any fail.

## Scope limits

- No `--jobs` concurrency limit (all jobs run at once)
- No input linking (`:::+`), no replacement strings beyond `{}`
- No `--keep-order` flag (output is always in argument order)
- CLI-only (no MCP tool exposure)
