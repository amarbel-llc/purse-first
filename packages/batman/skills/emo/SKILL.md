---
name: Conformance Test Suites
description: Use when writing BATS integration tests for MCP servers or CLI tools, when hardcoded build paths appear in tests, when making tests portable across implementations, when adding binary injection to test suites, or when auditing test conformance. Also applies when mentioning require_bin, binary injection, conformance testing, test portability, or implementation-agnostic tests.
version: 0.1.0
---

# Conformance Test Suites

This skill covers the pattern of writing BATS integration tests as portable
conformance suites. Tests validate behavior, not build artifacts. The binary
under test is injected from outside the test --- never discovered or built by
the test itself.

## Core Principle

A conformance test suite answers: "Does this binary implement the expected
behavior?" It never answers: "Can I build this binary?" These are fundamentally
different questions with different audiences. Build verification belongs in CI
pipelines and Nix expressions. Behavioral verification belongs in test suites.

When tests hardcode paths like `../result/bin/grit`, they become coupled to a
specific build system (Nix), a specific directory layout (result symlink), and
a specific build target. A Rust rewrite of a Go MCP server should be able to
run the same conformance suite without modifying a single test file.

## Anti-Pattern: Hardcoded Build Paths

```bash
# BAD: test knows how to find the build output
setup() {
  export GRIT_BIN="$BATS_TEST_DIRNAME/../result/bin/grit"
}
```

This fails when:
- The binary is built with cargo instead of nix
- The binary lives in a different output path
- The test runs in CI with a different directory layout
- Someone wants to test an installed system binary

```bash
# GOOD: test receives the binary from outside
setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
}

# In common.bash:
bats_load_library bats-emo
require_bin GRIT_BIN grit
```

The test suite is now agnostic to where the binary comes from. The caller
decides --- via environment variable, PATH, or `--bin-dir`.

## The `bats-emo` Library

The `bats-emo` bats library provides `require_bin`, a validation function for
binary injection. Load it in your `common.bash`:

```bash
bats_load_library bats-emo
```

### `require_bin` API

Two signatures handle different injection scenarios:

**Environment variable with PATH fallback:**

```bash
require_bin GRIT_BIN grit
```

1. If `GRIT_BIN` is set, verify the path is executable
2. If `GRIT_BIN` is unset, check if `grit` is on PATH
3. If neither works, fail with a clear message

This is the common case. The env var provides explicit control; PATH provides
convenience when the binary is already available (e.g., in a devShell).

**Environment variable only (no PATH fallback):**

```bash
require_bin BATS_WRAPPER
```

Use this when the binary under test shares a name with the test runner or
another tool on PATH. For example, batman's `bats` wrapper is tested by `bats`
itself --- searching PATH for `bats` would find the test runner, not the binary
under test.

### Error Messages

`require_bin` produces actionable error messages:

```
error: GRIT_BIN=/bad/path is not executable
error: grit not found. Set GRIT_BIN or use --bin-dir
error: BATS_WRAPPER not set
```

Each message tells the user exactly what to do next.

## Three-Layer Architecture

Conformance test suites use three layers, each with a single responsibility:

### Layer 1: Test File (*.bats)

Contains test cases. Loads `common.bash` in `setup()`. Never references build
paths, binary locations, or injection mechanisms.

```bash
#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
}

function my_tool_returns_json { # @test
  run my-tool --format json
  assert_success
  echo "$output" | jq -e '.version'
}
```

### Layer 2: common.bash

Loads libraries, validates binary availability, defines shared helpers. This is
where `require_bin` is called and where the binary variable is resolved.

```bash
bats_load_library bats-support
bats_load_library bats-assert
bats_load_library bats-island
bats_load_library bats-emo

require_bin MY_TOOL_BIN my-tool

run_my_tool() {
  local bin="${MY_TOOL_BIN:-my-tool}"
  "$bin" "$@"
}
```

### Layer 3: Justfile Adapter (root justfile)

Builds the binary, sets injection variables, delegates to the test justfile.
This is the only layer that knows about the build system.

```makefile
test-my-tool-bats:
    nix build .#my-tool
    MY_TOOL_BIN=result/bin/my-tool {{cmd_nix_dev}} just packages/my-tool/zz-tests_bats/test
```

The inner `zz-tests_bats/justfile` is build-system agnostic:

```makefile
bats_timeout := "10"

test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --tap --jobs {{num_cpus()}} {{targets}}

test: (test-targets "*.bats")
```

## Injection Mechanisms

Three mechanisms for providing the binary under test, from most explicit to most
convenient:

### 1. Environment Variable

```bash
GRIT_BIN=result/bin/grit just test-grit-bats
```

Most explicit. Used in root justfile recipes. The env var is checked first by
`require_bin`. Appropriate when you need to test a specific build output.

### 2. PATH via `--bin-dir`

```bash
bats --bin-dir result/bin --tap tests/
```

Batman's bats wrapper prepends `--bin-dir` directories to PATH. The binary is
found by name. Appropriate when the binary name is unambiguous.

```makefile
test-sandcastle-bats:
    nix build .#sandcastle
    {{cmd_nix_dev}} just packages/sandcastle/zz-tests_bats/test --bin-dir result/bin
```

### 3. Environment Variable Only (No PATH Fallback)

```bash
BATS_WRAPPER=result/bin/bats just test-batman-bats
```

Used when the binary name collides with another tool on PATH. Batman's `bats`
wrapper is itself tested by `bats` --- using PATH would find the test runner
instead of the binary under test. Call `require_bin BATS_WRAPPER` (one argument,
no command name) to enforce the env var.

## Choosing an Injection Mechanism

| Scenario | Mechanism | `require_bin` call |
|----------|-----------|-------------------|
| Binary name is unique | env var + PATH | `require_bin VAR_NAME cmd_name` |
| Binary name collides with test infra | env var only | `require_bin VAR_NAME` |
| Binary already on PATH (devShell) | PATH (implicit) | `require_bin VAR_NAME cmd_name` |

## Justfile Adapter Pattern

Every root justfile recipe follows the same structure:

1. **Build** the package (Nix-specific)
2. **Set** the injection variable or PATH
3. **Delegate** to the inner test justfile

```makefile
test-grit-bats:
    nix build .#grit
    GRIT_BIN=result/bin/grit {{cmd_nix_dev}} just packages/grit/zz-tests_bats/test
```

The inner justfile never builds anything. It only runs tests. This separation
means:

- The inner justfile works with any build system
- CI can build once, test multiple times
- Alternative implementations provide their own adapter

### Alternative Implementation Adapter

A Rust rewrite of grit could run the same conformance suite:

```makefile
test-grit-conformance:
    cargo build --release
    GRIT_BIN=target/release/grit bats packages/grit/zz-tests_bats/
```

No changes to any `.bats` file or `common.bash`.

## Industry Precedents

This pattern is well-established in software standards:

- **test262**: ECMAScript conformance suite used by V8, SpiderMonkey,
  JavaScriptCore, and dozens of other engines
- **POSIX shell test suites**: Validate sh implementations regardless of whether
  the shell is bash, dash, zsh, or busybox
- **SQL conformance suites**: Same queries tested against PostgreSQL, MySQL,
  SQLite
- **h2spec**: HTTP/2 conformance testing tool that validates any HTTP/2 server
- **OpenAPI/Swagger validators**: Test API implementations against a spec

The common thread: tests encode the specification, not the build.

## Conformance Audit Checklist

When auditing a test suite for conformance:

1. **Grep for hardcoded paths**: Search for `result/bin`, `../result`,
   `BATS_TEST_DIRNAME/..` followed by build output paths
2. **Check common.bash**: Does it load `bats-emo` and call `require_bin`?
3. **Check test files**: Do any `setup()` functions set binary paths directly?
4. **Check inner justfile**: Does it build anything, or only run tests?
5. **Check root justfile**: Does it build, set injection, then delegate?
6. **Test without build output**: Remove `result/`, set the env var to a known
   binary, run the tests. They should pass or fail based on behavior, not
   missing build artifacts.

### Current Inventory

| Package | Status | Injection | Root Recipe |
|---------|--------|-----------|-------------|
| spinclass | Conformant | PATH (devShell) | `test-spinclass-bats` |
| sandcastle | Conformant | PATH (`--bin-dir`) | `test-sandcastle-bats` |
| grit | Conformant | `GRIT_BIN` + bats-emo | `test-grit-bats` |
| batman bats_wrapper | Conformant | `BATS_WRAPPER` + bats-emo | `test-batman-bats` |
| batman island | Conformant | No binary under test | --- |
| repo-root tests | Conformant | `PURSE_FIRST_RESULT` env var | existing recipes |

## Adding Conformance to a New Package

1. Create `zz-tests_bats/common.bash` with library loads and `require_bin`
2. Create `zz-tests_bats/justfile` using the standard template
3. Write `.bats` files that load `common.bash` and use helpers
4. Add a root justfile recipe that builds and delegates
5. Add the recipe to the `test` dependency list
6. Verify: remove `result/`, set env var, run tests
