# Migrating to Batman's Bats Runner

This guide covers migrating from the old pattern (manual PATH/env var setup, per-repo sandcastle scripts, explicit `--tap` flags) to batman's self-contained `bats` wrapper.

## What the Wrapper Handles

Batman's `bats` wrapper now handles four concerns that previously required manual setup:

| Concern | Old Pattern | New Pattern |
|---------|-------------|-------------|
| Binary path | `export PATH=...; export DODDER_BIN=...` | `bats --bin-dir ./build/debug` |
| Sandcastle | Per-repo `run-sandcastle-bats.bash` scripts | Automatic (use `--no-sandbox` to skip) |
| Library path | `BATS_LIB_PATH` via nix setup-hook in devShell | Automatic (wrapper appends at runtime) |
| TAP output | Explicit `--tap` in every justfile recipe | Automatic (wrapper adds unless formatter specified) |

## Migration Steps

### 1. Remove per-repo sandcastle wrapper scripts

Delete files like `zz-tests_bats/bin/run-sandcastle-bats.bash`. The `bats` wrapper handles sandcastle automatically.

### 2. Remove preflight checks

Remove recipes like `_test-bats-preflight` that validate `bats`, `sandcastle`, and `BATS_LIB_PATH` are available. The wrapper bundles all three concerns.

### 3. Simplify root justfile

**Before:**
```makefile
test-bats-targets *targets: build _test-bats-preflight
  #!/usr/bin/env bash
  set -euo pipefail
  export PATH="{{dir_build}}/debug:$PATH"
  export DODDER_BIN="{{dir_build}}/debug/dodder"
  just zz-tests_bats/test-targets {{targets}}
```

**After:**
```makefile
test-bats-targets *targets: build
  just zz-tests_bats/test-targets --bin-dir {{dir_build}}/debug {{targets}}
```

The `--bin-dir` flag is consumed by the `bats` wrapper. It flows through the sub-justfile's `{{targets}}` parameter to `bats`.

### 4. Simplify test-suite justfile

**Before:**
```makefile
export DODDER_BIN := env("DODDER_BIN", "dodder")

test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" ./bin/run-sandcastle-bats.bash \
    bats --tap --jobs {{num_cpus()}} {{targets}}
```

**After:**
```makefile
test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --jobs {{num_cpus()}} {{targets}}
```

Changes:
- Remove `export DODDER_BIN` (binary is on PATH via `--bin-dir`)
- Remove `./bin/run-sandcastle-bats.bash` (sandcastle is automatic)
- Remove `--tap` (wrapper adds it by default)

### 5. Remove BATS_LIB_PATH from flake outputs

If your `flake.nix` devShell relied on `bats-libs` setup-hook to set `BATS_LIB_PATH`, this is no longer needed. The wrapper appends the library path at runtime.

### 6. Update test helpers (optional, per-project)

If tests use `$DODDER_BIN` or `$GRIT_BIN` to find the binary, they can optionally be updated to call the command by name directly (e.g., `dodder` instead of `$DODDER_BIN`), since `--bin-dir` puts it on PATH. This migration is optional and can happen per-project at any pace.

## Wrapper Flags Reference

```
bats [--bin-dir <dir>]... [--no-sandbox] [--] <bats-args>...
```

- `--bin-dir <dir>` — Prepend directory to PATH (repeatable, resolved to absolute path)
- `--no-sandbox` — Skip sandcastle wrapping
- `--` — Separator between batman flags and bats flags
- Without `--bin-dir`, PATH is unchanged (nvim/direnv use case)
- Without formatter flags (`--tap`, `--formatter`, `-F`, `--output`), `--tap` is added automatically
- `BATS_LIB_PATH` has batman's libraries appended (caller paths searched first)
