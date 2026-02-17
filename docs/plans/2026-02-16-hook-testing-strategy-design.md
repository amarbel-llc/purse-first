# Hook Testing Strategy Design

## Context

Exercising purse-first hooks in a live Claude Code session revealed 10 issues
spanning logic bugs, integration failures, and design gaps. This design
describes a comprehensive testing strategy that validates all issues using both
Go unit tests and BATS integration tests.

## Issues Under Test

| # | Issue | Category | Test Status |
|---|-------|----------|-------------|
| 1 | GC'd binary in settings.json hooks | integration | TODO/skip |
| 2 | Wrong MCP tool name format (`mcp__server__tool` vs `mcp__plugin_plugin_server__tool`) | bug | FAILING |
| 3 | Lux mapping tool names have spurious `lsp_` prefix | data bug | FAILING |
| 4 | `hook.Install()` is dead code (never called) | dead code | FAILING |
| 5 | `blocking: false` on PreToolUse in install code | bug | FAILING |
| 6 | PostToolUse and Stop hooks are complete no-ops | design gap | TODO/skip |
| 7 | Missing chix and get-hubbed mappings | design gap | TODO/skip |
| 8 | Grep/Glob with directory path bypasses extension matching | bug | TODO/skip |
| 9 | Read denial too aggressive (denies all Reads on supported extensions) | design gap | TODO/skip |
| 10 | Hooks not installed by `purse-first install` | design gap | TODO/skip |

## Approach

Extend existing Go test files and add new BATS integration test files. No new
test infrastructure or harness packages.

### Go Unit Tests

#### `internal/hook/handler_test.go` (4 new tests)

**TestFormatDenyReasonUsesPluginPrefix** (issues #2, #3 — FAILING)
- Given: server="lux", tool.Name="hover"
- Assert: reason contains `mcp__plugin_lux_lux__hover`
- Validates the format string produces the correct plugin MCP naming convention.

**TestHandlerGrepWithDirectoryPathAndTypeFilter** (issue #8 — TODO/skip)
- Given: tool_name="Grep", path="/some/dir", type="go"
- Assert: deny
- Currently passes through because extension matching only checks `path`.

**TestHandlerGlobWithDirectoryPathDenies** (issue #8 — TODO/skip)
- Given: tool_name="Glob", pattern="**/*.go", path="/some/dir"
- Assert: deny
- Currently passes through because `extractFilePath` returns the directory.

**TestHandlerReadDenialGranularity** (issue #9 — TODO/skip)
- Documents that Read on `.go` denies even when user wants file contents.
- Design decision needed on granularity.

#### `internal/hook/install_test.go` (2 new tests)

**TestInstallSetsBlockingTrueForPreToolUse** (issue #5 — FAILING)
- Call `Install()`, parse resulting settings JSON.
- Assert PreToolUse hook has `blocking: true`.

**TestInstallSetsBlockingFalseForPostToolUse** (correctness — passing)
- Assert PostToolUse hook has `blocking: false`.

#### `internal/mapping/loader_test.go` (1 new test)

**TestFindMatchChecksGlobParam** (issue #8 — TODO/skip)
- Mapping has `extensions: [".go"]`.
- Input has `glob: "*.go"` but `path` is a directory.
- Currently fails to match.

### BATS Integration Tests

#### `zz-tests_bats/hook_io.bats` (9 tests)

Hook I/O simulation: pipe JSON payloads to the built binary, assert on
stdout/exit code. Uses `common.bash` helpers.

| Test | Issue | Status |
|------|-------|--------|
| `deny_read_go_suggests_correct_plugin_tool_names` | #2, #3 | FAILING |
| `deny_git_suggests_correct_plugin_tool_names` | #2 | FAILING |
| `passthrough_non_matching_extension` | correctness | passing |
| `passthrough_no_file_path` | correctness | passing |
| `deny_output_valid_hook_json` | correctness | passing |
| `grep_directory_path_with_type_filter` | #8 | TODO/skip |
| `glob_directory_path_pattern` | #8 | TODO/skip |
| `post_hook_reads_stdin_cleanly` | #6 | passing |
| `session_end_exits_cleanly` | #6 | passing |

#### `zz-tests_bats/hook_lifecycle.bats` (6 tests)

Full install+verify cycle with sandcastle-isolated `$HOME`.

| Test | Issue | Status |
|------|-------|--------|
| `install_registers_marketplace_and_plugins` | correctness | passing |
| `installed_hooks_reference_valid_binary` | #1, #10 | TODO/skip |
| `pretooluse_hook_is_blocking` | #4, #5 | TODO/skip |
| `marketplace_json_validates` | correctness | passing |
| `chix_has_mappings` | #7 | TODO/skip |
| `get_hubbed_has_mappings` | #7 | TODO/skip |

#### `zz-tests_bats/bin/run-sandcastle-lifecycle.bash`

Sandcastle wrapper for lifecycle tests that isolates `$HOME`:

- Creates a temp directory in `/tmp` for the sandboxed HOME.
- Sets `HOME=$tmp_home` in the parent environment (sandcastle inherits it).
- Sandcastle config: `denyRead: ["$REAL_HOME/.claude"]`, `allowWrite: ["/tmp"]`.
- Passes through `PURSE_FIRST_RESULT` and `BATS_CWD`.

**Limitation:** Sandcastle does not support `--env` flags or mount-path
remapping. The real `$HOME` is visible read-only but `.claude/` is blocked by
denyRead. A TODO exists for native `--env` support in sandcastle.

### Justfile Additions

```makefile
test-hooks:
    nix develop --command go test -v ./internal/hook/...
    nix build
    nix develop --command zz-tests_bats/bin/run-sandcastle-bats.bash \
      bats --tap zz-tests_bats/hook_io.bats

test-lifecycle:
    nix build
    nix develop --command zz-tests_bats/bin/run-sandcastle-lifecycle.bash \
      bats --tap zz-tests_bats/hook_lifecycle.bats
```

## Issue-to-Test Mapping

| # | Issue | Go Test | BATS Test |
|---|-------|---------|-----------|
| 1 | GC'd binary | — | `installed_hooks_reference_valid_binary` (TODO) |
| 2 | Wrong tool name format | `TestFormatDenyReasonUsesPluginPrefix` | `deny_read_go_suggests_correct_plugin_tool_names` |
| 3 | lsp_ prefix mismatch | (data issue, validated via #2) | `deny_read_go_suggests_correct_plugin_tool_names` |
| 4 | hook.Install dead code | `TestInstallSetsBlockingTrueForPreToolUse` | `pretooluse_hook_is_blocking` (TODO) |
| 5 | blocking:false | `TestInstallSetsBlockingTrueForPreToolUse` | `pretooluse_hook_is_blocking` (TODO) |
| 6 | No-op PostToolUse/Stop | — | `post_hook_reads_stdin_cleanly` |
| 7 | Missing chix/get-hubbed mappings | — | `chix_has_mappings` (TODO) |
| 8 | Grep/Glob dir path bypass | `TestHandler*DirectoryPath*` (TODO) | `grep_directory_path*` (TODO) |
| 9 | Aggressive Read denial | `TestHandlerReadDenialGranularity` (TODO) | — |
| 10 | Hooks not in install | — | `installed_hooks_reference_valid_binary` (TODO) |
