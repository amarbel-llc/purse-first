# cargo-test command design

## Summary

Add a `cargo-test` subcommand to tap-dancer that runs `cargo test -- --format json` and converts the JSON event stream to TAP-14, mirroring the existing `go-test` command.

## CLI interface

```
tap-dancer cargo-test [--verbose] [--skip-empty] [-- extra-cargo-args...]
```

Same params as `go-test`: `verbose` includes output for passing tests, `skip-empty` emits `# SKIP` for crates with 0 tests.

## libtest JSON events

Rust's libtest (stable since 1.82) emits these JSON events with `--format json`:

- `{ "type": "suite", "event": "started", "test_count": N }` -- suite begins
- `{ "type": "test", "event": "started", "name": "..." }` -- test begins
- `{ "type": "test", "event": "ok"|"failed"|"ignored", "name": "...", "stdout": "...", "exec_time": N }` -- test result
- `{ "type": "suite", "event": "ok"|"failed", "passed": N, "failed": N, "ignored": N, ... }` -- suite summary

## TAP-14 mapping

- Each test suite (from a separate test binary) maps to a TAP subtest (like packages in go-test)
- Suite name comes from the "Running ..." line (non-JSON, parsed as comment but used to name the subtest)
- `ok` maps to `ok`, `failed` maps to `not ok` with diagnostics (stdout), `ignored` maps to `# SKIP`
- Suite with `test_count: 0` + `skip-empty` maps to `# SKIP`
- Rust's `mod` nesting (e.g. `tests::inner::test_foo`) maps to nested subtests, splitting on `::`

## Files

New:
- `packages/tap-dancer/go/cargotest.go` -- `ConvertCargoTest(r, w, verbose, skipEmpty) int`
- `packages/tap-dancer/go/cargotest_test.go` -- tests mirroring gotest_test.go patterns

Modified:
- `packages/tap-dancer/go/cmd/tap-dancer/main.go` -- register `cargo-test` command + `handleCargoTest`

## Exit codes

Same as go-test: 0 all pass, 1 any failure, 2 build errors (bail out if cargo can't start).
