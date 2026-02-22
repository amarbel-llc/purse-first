# Bats Runner Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extend batman's `bats` wrapper to be a self-contained entrypoint that handles binary path injection, sandcastle isolation, library path resolution, and TAP output defaults.

**Architecture:** The existing `bats` `writeShellApplication` in `flake.nix` gains flag parsing (`--bin-dir`, `--no-sandbox`), build-time `BATS_LIB_PATH` embedding, and TAP auto-detection. The `bats-libs` setup-hook is removed. Skills and examples are updated to document the new patterns and provide migration guidance.

**Tech Stack:** Nix (writeShellApplication), Bash, BATS

---

### Task 1: Update the bats wrapper in flake.nix

**Files:**
- Modify: `flake.nix:87-139`

**Step 1: Write the failing test**

Update `zz-tests_bats/bats_wrapper.bats` to add tests for the new flags. Add these test functions after the existing tests:

```bash
function bats_wrapper_prepends_bin_dir_to_path { # @test
  mkdir -p "${TEST_TMPDIR}/fake-bin"
  cat >"${TEST_TMPDIR}/fake-bin/my-tool" <<'EOF'
#!/usr/bin/env bash
echo "fake-tool-output"
EOF
  chmod +x "${TEST_TMPDIR}/fake-bin/my-tool"

  cat >"${TEST_TMPDIR}/bin_dir.bats" <<'INNER'
#! /usr/bin/env bats
function finds_tool_on_path { # @test
  run my-tool
  [ "$status" -eq 0 ]
  [ "$output" = "fake-tool-output" ]
}
INNER
  run bats --bin-dir "${TEST_TMPDIR}/fake-bin" --no-sandbox "${TEST_TMPDIR}/bin_dir.bats"
  assert_success
  assert_output --partial "ok 1"
}

function bats_wrapper_supports_multiple_bin_dirs { # @test
  mkdir -p "${TEST_TMPDIR}/bin-a" "${TEST_TMPDIR}/bin-b"
  cat >"${TEST_TMPDIR}/bin-a/tool-a" <<'EOF'
#!/usr/bin/env bash
echo "from-a"
EOF
  cat >"${TEST_TMPDIR}/bin-b/tool-b" <<'EOF'
#!/usr/bin/env bash
echo "from-b"
EOF
  chmod +x "${TEST_TMPDIR}/bin-a/tool-a" "${TEST_TMPDIR}/bin-b/tool-b"

  cat >"${TEST_TMPDIR}/multi.bats" <<'INNER'
#! /usr/bin/env bats
function finds_both_tools { # @test
  run tool-a
  [ "$status" -eq 0 ]
  [ "$output" = "from-a" ]
  run tool-b
  [ "$status" -eq 0 ]
  [ "$output" = "from-b" ]
}
INNER
  run bats --bin-dir "${TEST_TMPDIR}/bin-a" --bin-dir "${TEST_TMPDIR}/bin-b" --no-sandbox "${TEST_TMPDIR}/multi.bats"
  assert_success
  assert_output --partial "ok 1"
}

function bats_wrapper_defaults_to_tap_output { # @test
  cat >"${TEST_TMPDIR}/tap_default.bats" <<'EOF'
#! /usr/bin/env bats
function truth { # @test
  true
}
EOF
  run bats --no-sandbox "${TEST_TMPDIR}/tap_default.bats"
  assert_success
  # TAP output starts with version or plan line
  assert_line --index 0 --regexp "^(TAP version|1\.\.)"
}

function bats_wrapper_no_sandbox_skips_sandcastle { # @test
  cat >"${TEST_TMPDIR}/no_sandbox.bats" <<'EOF'
#! /usr/bin/env bats
function can_read_home_config { # @test
  # Without sandcastle, HOME/.config is accessible (if it exists)
  [[ -d "$HOME/.config" ]] || skip "no .config dir"
  ls "$HOME/.config" >/dev/null
}
EOF
  run bats --no-sandbox "${TEST_TMPDIR}/no_sandbox.bats"
  assert_success
}
```

**Step 2: Run tests to verify they fail**

Run: `nix build && bats zz-tests_bats/bats_wrapper.bats`
Expected: FAIL — `--bin-dir` and `--no-sandbox` are not recognized by the current wrapper.

**Step 3: Remove the bats-libs setup-hook**

In `flake.nix`, remove the `postBuild` block from the `bats-libs` derivation (lines 95-98):

Replace:
```nix
        bats-libs = pkgs.symlinkJoin {
          name = "bats-libs";
          paths = [
            bats-support
            bats-assert
            bats-assert-additions
            tap-writer
          ];
          postBuild = ''
            mkdir -p $out/nix-support
            echo 'export BATS_LIB_PATH="'"$out"'/share/bats''${BATS_LIB_PATH:+:$BATS_LIB_PATH}"' > $out/nix-support/setup-hook
          '';
        };
```

With:
```nix
        bats-libs = pkgs.symlinkJoin {
          name = "bats-libs";
          paths = [
            bats-support
            bats-assert
            bats-assert-additions
            tap-writer
          ];
        };
```

**Step 4: Rewrite the bats wrapper**

Replace the existing `bats` `writeShellApplication` (lines 103-139) with:

```nix
        bats = pkgs.writeShellApplication {
          name = "bats";
          runtimeInputs = [
            pkgs.bats
            pkgs.coreutils
            sandcastle-pkg
          ];
          text = ''
            bin_dirs=()
            sandbox=true

            while (( $# > 0 )); do
              case "$1" in
                --bin-dir)
                  bin_dirs+=("$(realpath "$2")")
                  shift 2
                  ;;
                --no-sandbox)
                  sandbox=false
                  shift
                  ;;
                --)
                  shift
                  break
                  ;;
                *)
                  break
                  ;;
              esac
            done

            # Prepend --bin-dir directories to PATH (leftmost = highest priority)
            for (( i = ''${#bin_dirs[@]} - 1; i >= 0; i-- )); do
              export PATH="''${bin_dirs[$i]}:$PATH"
            done

            # Append batman's bats-libs to BATS_LIB_PATH (caller paths take precedence)
            export BATS_LIB_PATH="''${BATS_LIB_PATH:+$BATS_LIB_PATH:}${bats-libs}/share/bats"

            # Default to TAP output unless a formatter flag is already present
            has_formatter=false
            for arg in "$@"; do
              case "$arg" in
                --tap|--formatter|-F|--output) has_formatter=true; break ;;
              esac
            done
            if ! $has_formatter; then
              set -- "$@" --tap
            fi

            if $sandbox; then
              config="$(mktemp)"
              trap 'rm -f "$config"' EXIT

              cat >"$config" <<SANDCASTLE_CONFIG
            {
              "filesystem": {
                "denyRead": [
                  "$HOME/.ssh",
                  "$HOME/.aws",
                  "$HOME/.gnupg",
                  "$HOME/.config",
                  "$HOME/.local",
                  "$HOME/.password-store",
                  "$HOME/.kube"
                ],
                "denyWrite": [],
                "allowWrite": [
                  "/tmp"
                ]
              },
              "network": {
                "allowedDomains": [],
                "deniedDomains": []
              }
            }
            SANDCASTLE_CONFIG

              exec sandcastle --shell bash --config "$config" bats "$@"
            else
              exec bats "$@"
            fi
          '';
        };
```

**Step 5: Build and run tests**

Run: `nix build && bats zz-tests_bats/bats_wrapper.bats`
Expected: All tests pass (old and new).

**Step 6: Commit**

```bash
git add flake.nix zz-tests_bats/bats_wrapper.bats
git commit -m "feat: extend bats wrapper with --bin-dir, --no-sandbox, TAP default, and BATS_LIB_PATH"
```

---

### Task 2: Update SKILL.md

**Files:**
- Modify: `skills/bats-testing/SKILL.md`

**Step 1: Update the "Common Test Helper" section (around line 99)**

Replace the paragraph about `BATS_LIB_PATH` being set automatically by setup-hook:

```markdown
The `bats_load_library` function searches `BATS_LIB_PATH` for each library. This is set automatically when `batman.packages.${system}.default` is in your devShell packages (see Nix Flake Integration below).
```

With:

```markdown
The `bats_load_library` function searches `BATS_LIB_PATH` for each library. Batman's `bats` wrapper automatically appends the bundled libraries to `BATS_LIB_PATH` at runtime — no devShell setup-hook or manual configuration needed. If you set `BATS_LIB_PATH` before invoking `bats`, your paths take precedence (searched first).
```

**Step 2: Update the "Nix Flake Integration" section (around line 139)**

Replace:

```markdown
Add `batman` as a flake input, then include `batman.packages.${system}.default` in the devShell packages. The default package bundles everything: assertion libraries (`bats-libs`), a sandcastle-wrapped `bats` binary, and the `robin` skill plugin. The `bats-libs` component includes a setup hook that automatically exports `BATS_LIB_PATH`, so `bats_load_library` works without any manual environment configuration.
```

With:

```markdown
Add `batman` as a flake input, then include `batman.packages.${system}.default` in the devShell packages. The default package bundles everything: assertion libraries (`bats-libs`), the `bats` wrapper, and the `robin` skill plugin. The `bats` wrapper automatically handles `BATS_LIB_PATH`, sandcastle isolation, and TAP output — no setup-hooks or manual environment configuration needed.
```

**Step 3: Update the paragraph at line 162**

Replace:

```markdown
With this setup, `bats_load_library bats-support`, `bats_load_library bats-assert`, and `bats_load_library bats-assert-additions` all resolve automatically via `BATS_LIB_PATH`, and every `bats` invocation is sandboxed transparently.
```

With:

```markdown
With this setup, `bats_load_library bats-support`, `bats_load_library bats-assert`, and `bats_load_library bats-assert-additions` all resolve automatically, and every `bats` invocation is sandboxed transparently.
```

**Step 4: Update the "Justfile Integration" section (around line 203)**

Replace the root justfile example:

```markdown
### Root justfile

Delegate test orchestration from the project root:

\```makefile
test-bats: build test-bats-run

test-bats-run $PATH=(dir_build / "debug" + ":" + env("PATH")):
  just zz-tests_bats/test

test: test-go test-bats
\```
```

With:

```markdown
### Root justfile

Delegate test orchestration from the project root. Use `--bin-dir` to make the freshly-built binary available to tests via PATH:

\```makefile
test-bats: build
  just zz-tests_bats/test --bin-dir {{dir_build}}/debug

test: test-go test-bats
\```

The `--bin-dir` flag prepends the given directory to `PATH` before running bats. Tests find the binary through standard command lookup — no special env vars needed.
```

Replace the test-suite justfile example:

```markdown
### Test-suite justfile (zz-tests_bats/justfile)

\```makefile
export CMD_BIN := "my-command"

bats_timeout := "5"

test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --tap --jobs {{num_cpus()}} {{targets}}

test-tags *tags:
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --tap --jobs {{num_cpus()}} --filter-tags {{tags}} *.bats

test: (test-targets "*.bats")
\```

Key patterns:
- TAP output via `--tap` for CI/pipeline compatibility (see `references/tap14.md` for the full TAP version 14 specification)
- Parallel execution via `--jobs {{num_cpus()}}`
- Per-test timeout via `BATS_TEST_TIMEOUT`
- Tag-based filtering via `--filter-tags`
- Sandcastle isolation is automatic — batman's `bats` binary handles it
```

With:

```markdown
### Test-suite justfile (zz-tests_bats/justfile)

\```makefile
bats_timeout := "5"

test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --jobs {{num_cpus()}} {{targets}}

test-tags *tags:
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --jobs {{num_cpus()}} --filter-tags {{tags}} *.bats

test: (test-targets "*.bats")
\```

Key patterns:
- TAP output is automatic (wrapper defaults to `--tap` unless another formatter is specified)
- Parallel execution via `--jobs {{num_cpus()}}`
- Per-test timeout via `BATS_TEST_TIMEOUT`
- Tag-based filtering via `--filter-tags`
- Sandcastle isolation is automatic — batman's `bats` binary handles it
- `--bin-dir` flags pass through from the root justfile via `{{targets}}`
```

**Step 5: Update the "Bundled Libraries" section (around line 259)**

Replace:

```markdown
All three libraries are packaged in `bats-libs` and available via `BATS_LIB_PATH` when the flake input is added to your devShell:
```

With:

```markdown
All libraries are packaged in `bats-libs` and available via `BATS_LIB_PATH` automatically when using batman's `bats` wrapper:
```

**Step 6: Commit**

```bash
git add skills/bats-testing/SKILL.md
git commit -m "docs: update SKILL.md for bats wrapper --bin-dir and auto BATS_LIB_PATH"
```

---

### Task 3: Update example justfile

**Files:**
- Modify: `skills/bats-testing/examples/justfile`

**Step 1: Replace the example justfile content**

Replace the full content with:

```makefile
# zz-tests_bats/justfile
# Test-suite justfile for BATS integration tests

bats_timeout := "5"

# Run specific test files (default: all .bats files)
test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --jobs {{num_cpus()}} {{targets}}

# Run tests matching specific tags
test-tags *tags:
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --jobs {{num_cpus()}} --filter-tags {{tags}} *.bats

# Run all tests
test: (test-targets "*.bats")
```

Changes from previous version:
- Removed `export CMD_BIN := "my-command"` (binary comes from PATH via `--bin-dir`)
- Removed explicit `--tap` flags (wrapper adds automatically)

**Step 2: Commit**

```bash
git add skills/bats-testing/examples/justfile
git commit -m "docs: simplify example justfile for bats wrapper defaults"
```

---

### Task 4: Update example common.bash

**Files:**
- Modify: `skills/bats-testing/examples/common.bash`

**Step 1: Update the command wrapper section**

Replace the `cmd_defaults` and `run_cmd` block:

```bash
# Command wrapper: runs the binary under test with normalized defaults
cmd_defaults=(
  # Add project-specific default flags here to normalize output
  # -print-colors=false
  # -print-time=false
)

run_cmd() {
  subcmd="$1"
  shift
  run timeout --preserve-status "2s" "$CMD_BIN" "$subcmd" ${cmd_defaults[@]} "$@"
}
```

With:

```bash
# Command wrapper: runs the binary under test with normalized defaults.
# The binary must be on PATH — use `bats --bin-dir <dir>` to inject it.
cmd_defaults=(
  # Add project-specific default flags here to normalize output
  # -print-colors=false
  # -print-time=false
)

run_cmd() {
  subcmd="$1"
  shift
  run timeout --preserve-status "2s" my-command "$subcmd" ${cmd_defaults[@]} "$@"
}
```

The change: `"$CMD_BIN"` becomes a literal command name (`my-command`), since the binary is expected to be on PATH via `--bin-dir`. Projects replace `my-command` with their actual binary name.

**Step 2: Commit**

```bash
git add skills/bats-testing/examples/common.bash
git commit -m "docs: update example common.bash to use PATH-based binary lookup"
```

---

### Task 5: Update sandcastle reference

**Files:**
- Modify: `skills/bats-testing/references/sandcastle.md`

**Step 1: Update the "Batman Bats Wrapper" section (around line 67)**

Replace:

```markdown
## Batman Bats Wrapper

Batman provides a `bats` wrapper that automatically invokes sandcastle. When you use `batman.packages.${system}.bats` in your devShell, every `bats` invocation is sandboxed with:

- **Read denied:** `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.config`, `~/.local`, `~/.password-store`, `~/.kube`
- **Write allowed:** `/tmp` only
- **Network:** unrestricted

No wrapper script or manual sandcastle configuration is needed. Just run `bats` normally.

For custom sandcastle policies beyond the defaults (e.g., network restrictions, additional deny paths), you can invoke sandcastle directly — see the CLI interface section above.
```

With:

```markdown
## Batman Bats Wrapper

Batman's `bats` wrapper automatically invokes sandcastle. Every `bats` invocation is sandboxed with:

- **Read denied:** `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.config`, `~/.local`, `~/.password-store`, `~/.kube`
- **Write allowed:** `/tmp` only
- **Network:** unrestricted

No wrapper script or manual sandcastle configuration is needed. Just run `bats` normally.

To skip sandcastle (e.g., for debugging sandbox issues), pass `--no-sandbox`:

```bash
bats --no-sandbox my_test.bats
```

For custom sandcastle policies beyond the defaults (e.g., network restrictions, additional deny paths), use `--no-sandbox` and invoke sandcastle directly — see the CLI interface section above.
```

**Step 2: Commit**

```bash
git add skills/bats-testing/references/sandcastle.md
git commit -m "docs: document --no-sandbox flag in sandcastle reference"
```

---

### Task 6: Create migration reference

**Files:**
- Create: `skills/bats-testing/references/migration.md`

**Step 1: Write the migration guide**

```markdown
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
```

**Step 2: Commit**

```bash
git add skills/bats-testing/references/migration.md
git commit -m "docs: add migration guide for bats runner transition"
```

---

### Task 7: Final verification

**Step 1: Build the full package**

Run: `nix build`
Expected: Builds successfully, `./result/bin/bats` is the updated wrapper.

**Step 2: Run batman's own bats tests**

Run: `bats zz-tests_bats/bats_wrapper.bats`
Expected: All tests pass (including new tests for `--bin-dir`, `--no-sandbox`, TAP default).

**Step 3: Verify BATS_LIB_PATH is not in setup-hook**

Run: `cat ./result/nix-support/setup-hook 2>/dev/null || echo "no setup-hook"`
Expected: Either no setup-hook file, or it doesn't contain `BATS_LIB_PATH`.

**Step 4: Verify wrapper contains embedded bats-libs path**

Run: `grep -c 'share/bats' ./result/bin/bats`
Expected: At least 1 match (the embedded `BATS_LIB_PATH` line).

**Step 5: Commit any remaining changes**

If any fixes were needed during verification, commit them.
