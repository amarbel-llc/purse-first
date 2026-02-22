---
name: BATS Testing Setup
description: This skill should be used when the user mentions testing, tests, bats, integration testing, integration tests, CLI testing, functional testing, functional tests, test setup, test infrastructure, test helpers, test assertions, or any task involving writing, running, debugging, or setting up tests of any kind. Also applies when mentioning bats-core, bats-assert, bats-support, sandcastle, test isolation, test targets, justfile test recipes, or .bats files.
version: 0.1.0
---

# BATS Testing Setup

This skill provides expert guidance for setting up BATS (Bash Automated Testing System) integration tests in Nix-backed repositories. It covers the full stack: bats-support assertion libraries, test helper infrastructure, justfile task integration, and sandcastle-based environment isolation.

## Directory Convention

Place all BATS tests in a `zz-tests_bats/` directory at the project root:

```
project/
├── zz-tests_bats/
│   ├── justfile
│   ├── common.bash
│   ├── some_feature.bats
│   └── another_feature.bats
├── justfile                  (root justfile delegates to zz-tests_bats/)
└── flake.nix
```

## Test File Format

Use the function-name-based test declaration pattern with `# @test` annotation:

```bash
#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
}

teardown() {
  # Runs after each test regardless of success/failure. BATS_TEST_TMPDIR is
  # cleaned up automatically by bats — use teardown for resources created
  # outside managed temp dirs (e.g. background processes, external files).
}

function descriptive_test_name { # @test
  run my_command arg1 arg2
  assert_success
  assert_output "expected output"
}

function another_descriptive_name { # @test
  run my_command --flag
  assert_failure
  assert_output --partial "error message"
}
```

Key conventions:
- Shebang: `#! /usr/bin/env bats`
- Test functions use `function name { # @test` (not `@test "description"`)
- Function names serve as test identifiers -- make them descriptive enough to avoid comments
- Always `export output` in setup for assertion access
- Load helpers relative to `$BATS_TEST_FILE`

## Test Data Self-Containment

Tests must never rely on data outside the test itself. Every test must create or declare all data it needs — no reading from the user's home directory, no depending on pre-existing files, no assuming environment state.

**Required:** All test data comes from one of:
- **Inline fixtures** — data declared directly in the test function or helper
- **Fixture files** — static data stored in the test directory (e.g. `zz-tests_bats/migration/v1/`)
- **Generated fixtures** — data created programmatically in `setup()` or a helper function
- **`$BATS_TEST_TMPDIR`** — all generated files go here, cleaned up automatically

**Forbidden:**
- Reading files from `$HOME`, `$PWD` (project root), or any path outside the test directory
- Depending on tools, configs, or services not provided by the devShell
- Assuming environment variables are set (other than those explicitly exported in `setup()`)
- Sharing state between test functions (each test must stand alone)

```bash
# ❌ BAD: depends on data outside the test
function imports_user_config { # @test
  run my_command import ~/.config/myapp/settings.json
  assert_success
}

# ✅ GOOD: creates its own fixture
function imports_config { # @test
  cat > "$BATS_TEST_TMPDIR/settings.json" <<-EOF
    {"key": "value", "debug": false}
  EOF
  run my_command import "$BATS_TEST_TMPDIR/settings.json"
  assert_success
}
```

This principle is enforced by sandcastle at runtime — tests that read from denied paths will fail. But write tests correctly from the start rather than relying on sandcastle to catch violations. See `references/patterns.md` for fixture management patterns.

## Common Test Helper (common.bash)

Create `zz-tests_bats/common.bash` to load assertion libraries and define shared utilities:

```bash
bats_load_library bats-support
bats_load_library bats-assert
bats_load_library bats-assert-additions
```

The `bats_load_library` function searches `BATS_LIB_PATH` for each library. Batman's `bats` wrapper automatically appends the bundled libraries to `BATS_LIB_PATH` at runtime — no devShell setup-hook or manual configuration needed. If you set `BATS_LIB_PATH` before invoking `bats`, your paths take precedence (searched first).

Add project-specific helpers here: XDG isolation, command wrappers with default flags, fixture loaders, and cleanup functions. See `references/patterns.md` for detailed examples.

## Assertion Libraries

### Core Assertions (bats-assert)

| Function | Purpose |
|----------|---------|
| `assert_success` | Exit code is 0 |
| `assert_failure` | Exit code is non-zero |
| `assert_output "text"` | Exact match on full output |
| `assert_output --partial "text"` | Substring match |
| `assert_output --regexp "pattern"` | Regex match |
| `assert_output -` | Read expected from stdin (heredoc) |
| `refute_output "text"` | Output does NOT match |
| `assert_line "text"` | At least one line matches |
| `assert_line --index N "text"` | Specific line matches |
| `assert_equal "actual" "expected"` | String equality |

### Custom Assertions (bats-assert-additions)

Two additional assertion functions in bats-assert-additions extend bats-assert for common CLI testing patterns:

- **`assert_output_unsorted`** -- Sorts output before comparing. Accepts `--regexp`, `--partial`, and stdin (`-`). Essential for testing commands with non-deterministic output ordering.
- **`assert_output_cut`** -- Pipes output through `cut` before comparing. Accepts `-d` (delimiter), `-f` (fields), and `-s` (also sort). Useful for field-based output validation.

See `references/patterns.md` for usage examples.

## Nix Flake Integration

Add `batman` as a flake input, then include `batman.packages.${system}.default` in the devShell packages. The default package bundles everything: assertion libraries (`bats-libs`), the `bats` wrapper, and the `robin` skill plugin. The `bats` wrapper automatically handles `BATS_LIB_PATH`, sandcastle isolation, and TAP output — no setup-hooks or manual environment configuration needed.

```nix
inputs = {
  batman.url = "github:amarbel-llc/batman";
};
```

```nix
devShells.default = pkgs.mkShell {
  packages = (with pkgs; [
    just
    gum
  ]) ++ [
    batman.packages.${system}.default
  ];
};
```

Do **not** add `pkgs.bats` separately — batman provides its own `bats` binary that wraps sandcastle for automatic environment isolation. Adding `pkgs.bats` alongside would shadow it.

With this setup, `bats_load_library bats-support`, `bats_load_library bats-assert`, and `bats_load_library bats-assert-additions` all resolve automatically, and every `bats` invocation is sandboxed transparently.

## Sandcastle Environment Isolation

Sandcastle and XDG isolation are complementary layers:

- **Sandcastle** catches leaks by denying access to real `$HOME/.config`, `$HOME/.ssh`, etc. It is the enforcement mechanism: if a test accidentally reads or writes outside its sandbox, sandcastle makes it fail loudly.
- **XDG isolation** (`set_xdg`) prevents leaks by redirecting `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, etc. into `$BATS_TEST_TMPDIR`. It is the prevention mechanism: tests write to the right place in the first place.

Both layers are required. Without sandcastle, XDG isolation can silently fail (e.g. `GIT_CONFIG_GLOBAL` overriding `XDG_CONFIG_HOME`). Without XDG isolation, sandcastle will block legitimate test operations that need config directories.

### Git Config Isolation

`git config --global` uses `$XDG_CONFIG_HOME/git/config` by default, but **`GIT_CONFIG_GLOBAL` takes precedence** if set. Many dotfile setups (rcm, direnv) export `GIT_CONFIG_GLOBAL` to an absolute path, which bypasses `$HOME` and `$XDG_CONFIG_HOME` redirection entirely. Always override `GIT_CONFIG_GLOBAL` alongside the XDG vars in `setup_test_home`:

```bash
setup_test_home() {
  export REAL_HOME="$HOME"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
  set_xdg "$BATS_TEST_TMPDIR"
  mkdir -p "$XDG_CONFIG_HOME/git"
  export GIT_CONFIG_GLOBAL="$XDG_CONFIG_HOME/git/config"
  git config --global user.name "Test User"
  git config --global user.email "test@example.com"
  git config --global init.defaultBranch main
}
```

The `mkdir -p "$XDG_CONFIG_HOME/git"` is required because `set_xdg` creates the top-level XDG directories but git needs the `git/` subdirectory to exist before it can write config files.

Batman's packaged `bats` binary wraps sandcastle transparently — every `bats` invocation is automatically sandboxed with sensible defaults:

- **Read denied:** `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.config`, `~/.local`, `~/.password-store`, `~/.kube`
- **Write allowed:** `/tmp` (and `/private/tmp` on macOS)
- **Network:** unrestricted

No wrapper script or manual sandcastle configuration is needed. Just run `bats` normally.

For custom sandcastle policies beyond the defaults (e.g., network restrictions, additional deny paths), use `--no-sandbox` and invoke sandcastle directly. See `references/sandcastle.md` for configuration details.

## Justfile Integration

### Root justfile

Delegate test orchestration from the project root. Use `--bin-dir` to make the freshly-built binary available to tests via PATH:

```makefile
test-bats: build
  just zz-tests_bats/test --bin-dir {{dir_build}}/debug

test: test-go test-bats
```

The `--bin-dir` flag prepends the given directory to `PATH` before running bats. Tests find the binary through standard command lookup — no special env vars needed.

### Test-suite justfile (zz-tests_bats/justfile)

```makefile
bats_timeout := "5"

test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --jobs {{num_cpus()}} {{targets}}

test-tags *tags:
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --jobs {{num_cpus()}} --filter-tags {{tags}} *.bats

test: (test-targets "*.bats")
```

Key patterns:
- TAP output is automatic (wrapper defaults to `--tap` unless another formatter is specified)
- Parallel execution via `--jobs {{num_cpus()}}`
- Per-test timeout via `BATS_TEST_TIMEOUT`
- Tag-based filtering via `--filter-tags`
- Sandcastle isolation is automatic — batman's `bats` binary handles it
- `--bin-dir` flags pass through from the root justfile via `{{targets}}`

## Setup Checklist

When setting up BATS in a new repo:

1. Add `batman` flake input to `flake.nix`
2. Add `batman.packages.${system}.default` to devShell packages (do not add `pkgs.bats` separately)
3. Create `zz-tests_bats/` directory structure
4. Create `common.bash` using `bats_load_library` to load assertion libraries
5. Add test justfile with `test-targets`, `test-tags`, and `test` recipes
6. Wire root justfile to delegate to `zz-tests_bats/test`
7. Create first `.bats` test file following the function-name pattern

## Additional Resources

### Bundled Libraries

All libraries are packaged in `bats-libs` and available via `BATS_LIB_PATH` automatically when using batman's `bats` wrapper:
- **bats-support** -- Core support library (output formatting, error helpers, lang utilities)
- **bats-assert** -- Standard assertion library (assert_success, assert_output, assert_line, etc.)
- **bats-assert-additions** -- Custom assertions (assert_output_unsorted, assert_output_cut)

### Reference Files

For detailed patterns and advanced techniques, consult:
- **`references/patterns.md`** -- Common helper patterns, custom assertions, fixture management, XDG isolation, command wrappers, server testing
- **`references/sandcastle.md`** -- Sandcastle configuration format, security policies, network restrictions, advanced isolation patterns
- **`references/tap14.md`** -- TAP version 14 specification. Load this when you need to understand, produce, or validate TAP output format (version line, plan, test points, YAML diagnostics, subtests, directives, escaping rules)
- **`references/migration.md`** -- Step-by-step guide for migrating from manual PATH/sandcastle/TAP setup to the bats wrapper

### Example Files

Working templates in `examples/`:
- **`examples/common.bash`** -- Starter common.bash with XDG isolation and cleanup
- **`examples/example.bats`** -- Annotated example test file
- **`examples/justfile`** -- Test-suite justfile template
