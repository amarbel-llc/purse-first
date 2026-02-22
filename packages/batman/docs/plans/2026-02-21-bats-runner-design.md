# Bats Runner Design

## Problem

The boundary between "outside bats" (environment setup) and "inside bats"
(test execution) is implicit and fragile. Multiple callers — nvim, justfiles,
agents — each need to correctly configure PATH, BATS_LIB_PATH, sandcastle, and
TAP output before invoking bats. The assumptions are scattered across justfiles,
common.bash files, env vars, and per-repo sandcastle wrapper scripts. This
causes confusing failures unrelated to actual test results, especially for
agents running tests to validate changes.

## Solution

Extend batman's existing `bats` wrapper to be a self-contained entrypoint that
handles all "outside bats" concerns: binary path injection, sandcastle
isolation, library path resolution, and TAP output defaults.

## Interface

```
bats [--bin-dir <dir>]... [--no-sandbox] [--] <bats-args>...
```

- `--bin-dir <dir>` — Prepend `<dir>` to `PATH`. Can be specified multiple
  times; directories are prepended in order (leftmost = highest priority).
  Resolved to absolute path via `realpath`.
- `--no-sandbox` — Skip sandcastle wrapping (useful for debugging sandbox
  issues).
- `--` — Optional separator between batman flags and bats flags.
- Everything else passes through to `bats`.

### TAP Default

If none of `--tap`, `--formatter`, `-F`, or `--output` appear in the bats args,
the wrapper adds `--tap` automatically.

### Without `--bin-dir`

PATH is unchanged. This is the nvim/direnv use case — the wrapper adds
sandcastle, libraries, and TAP default only.

## Implementation

The wrapper is a `writeShellApplication` in `flake.nix`, replacing the current
sandcastle-only wrapper.

1. **Parse batman-specific flags** — consume `--bin-dir` and `--no-sandbox`
   from the front of `$@`, stop at `--` or the first unrecognized flag.
2. **PATH manipulation** — for each `--bin-dir`, resolve to absolute path via
   `realpath`, prepend to `PATH`.
3. **BATS_LIB_PATH** — at build time, the `bats-libs` store path is embedded
   in the script. At runtime, it is **appended** to `BATS_LIB_PATH`:
   `BATS_LIB_PATH="${BATS_LIB_PATH:+$BATS_LIB_PATH:}<bats-libs>/share/bats"`.
   Caller-set paths appear first and take precedence during library lookup.
4. **TAP default** — scan remaining args for `--tap`, `--formatter`, `-F`,
   `--output`. If none found, append `--tap`.
5. **Sandcastle** — unless `--no-sandbox`, generate the sandcastle config (same
   deny policies as today) and exec through sandcastle.
6. **Exec bats** — pass all remaining args to bats.

### Build-Time Wiring

The `bats-libs` output path is interpolated into the script text at nix build
time. The `bats-libs` package's `postBuild` setup-hook
(`nix-support/setup-hook`) is removed. The wrapper is the sole source of truth
for library path resolution.

### Runtime Inputs

- `pkgs.bats`
- `sandcastle-pkg`
- `pkgs.coreutils` (for `realpath`, `mktemp`)

## Consumer Impact

### Justfiles

Root justfiles simplify from:

```makefile
# before
test-bats-targets *targets: build _test-bats-preflight
  #!/usr/bin/env bash
  set -euo pipefail
  export PATH="{{dir_build}}/debug:$PATH"
  export DODDER_BIN="{{dir_build}}/debug/dodder"
  just zz-tests_bats/test-targets {{targets}}
```

to:

```makefile
# after
test-bats-targets *targets: build
  just zz-tests_bats/test-targets --bin-dir {{dir_build}}/debug {{targets}}
```

Sub-level justfiles simplify from:

```makefile
# before
test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" ./bin/run-sandcastle-bats.bash \
    bats --tap --jobs {{num_cpus()}} {{targets}}
```

to:

```makefile
# after
test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --jobs {{num_cpus()}} {{targets}}
```

The `--bin-dir` flag flows through naturally since `{{targets}}` captures all
positional args.

### nvim

No changes needed. `makeprg` stays `bats --jobs 8 --tap %`. The wrapper adds
sandcastle and library path. TAP default doesn't double up because `--tap` is
already present in the args.

### Agents

Can run `bats --bin-dir ./build/debug *.bats` directly. Clear, explicit, no
hidden env var setup needed.

### Dead Code

Per-repo sandcastle scripts (e.g., `zz-tests_bats/bin/run-sandcastle-bats.bash`)
become dead code. Preflight checks validating `bats`, `sandcastle`, and
`BATS_LIB_PATH` can be simplified or removed.

## Skill Updates

### SKILL.md

- Justfile integration: replace sandcastle/TAP/env var ceremony with `--bin-dir`
- Environment setup: remove `BATS_LIB_PATH` devshell guidance, document wrapper
- Sandcastle: note automatic wrapping, document `--no-sandbox`
- Binary path: document `--bin-dir` as the standard injection mechanism

### examples/justfile

Simplify to use new wrapper flags.

### examples/common.bash

Remove any `BATS_LIB_PATH` validation. Wrapper guarantees library availability.

### references/sandcastle.md

Note sandcastle is handled by wrapper. Custom policies require `--no-sandbox`
plus manual sandcastle invocation.

### references/migration.md (new)

Migration guide covering:

- Removing per-repo `run-sandcastle-bats.bash` scripts
- Removing `_test-bats-preflight` checks for `bats`, `sandcastle`,
  `BATS_LIB_PATH`
- Removing explicit `--tap` from justfile recipes (wrapper adds it by default)
- Replacing `PATH=` / `export *_BIN=` patterns with `--bin-dir`
- Removing `BATS_LIB_PATH` from devshell/flake outputs
- Before/after examples for root justfile and `zz-tests_bats/justfile`
