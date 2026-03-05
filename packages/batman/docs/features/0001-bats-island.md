---
status: proposed
date: 2026-03-01
promotion-criteria: >
  Promote to proposed when the consumer audit is reviewed and the interface
  (function signatures, env var coverage, helper scope) is finalized.
---

# bats-island

## Motivation

Every batman bats lib consumer that needs test isolation must copy-paste
`set_xdg`, `setup_test_home`, `chflags_nouchg`, and `setup_test_repo` into
their own `common.bash`. This duplication is error-prone: implementers may
forget to call `set_xdg`, omit the `GIT_CONFIG_GLOBAL` override, skip
`GIT_CEILING_DIRECTORIES`, or diverge from the canonical implementation over
time. Batman should ship test isolation as a loadable bats library so consumers
get correct isolation and common helpers without copying boilerplate.

Two approaches are viable: a loadable bats library (opt-in, per-test isolation)
and wrapper-level automatic isolation (zero-effort, per-run isolation). This FDR
presents both, recommends a phased rollout starting with the library.

### Consumer audit

An audit of all batman bats lib consumers in `~/eng/repos` found that the
problem is worse than the two known duplication sites suggest:

| Consumer | Has isolation? | Sufficient? | Gap |
|---|---|---|---|
| dodder | Partial (app-level `-override-xdg-with-cwd` flag) | No | 12 tests missing the flag; `set_xdg` diverged (adds validation, skips `mkdir -p`) |
| grit | Yes (canonical `setup_test_home`) | Nearly | `GIT_CONFIG_SYSTEM` and `GIT_CEILING_DIRECTORIES` not sterilized |
| pivy | No | No | `pivy_agent.bats` writes `$HOME/Library` and `$HOME/.config/systemd/` via ad-hoc per-command `HOME=` overrides |
| purse-first (root) | No | No | `hook_lifecycle.bats` reads `$HOME/.claude/`; validate tests call `claude` against real HOME |
| sandcastle | N/A | N/A | Pure output tests, no HOME/XDG interaction |
| batman | Yes (via sandcastle) | Yes | None |

Beyond isolation, the audit found two helpers duplicated across 5+ consumers
(`chflags_nouchg`, `setup_test_repo`) that belong alongside isolation in a
single library.

Key findings:

1. **More consumers need isolation than currently have it.** Pivy and purse-first
   root have real leaks today — they read/write user HOME without any isolation.

2. **Dodder proves divergence is already happening.** Its `set_xdg` skips
   `mkdir -p` and adds input validation the canonical version lacks. One
   consumer, one copy, one divergence — a 100% drift rate.

3. **Dodder illustrates how roll-your-own isolation creates layered gaps.**
   Dodder built its own application-level XDG override (`-override-xdg-with-cwd`)
   instead of using test-level `set_xdg` consistently. This created two
   isolation mechanisms with different coverage: the app flag covers most init
   paths but 12 tests use an init helper that forgot the flag, and the
   test-level `set_xdg` covers the remaining 3 tests but diverged from the
   canonical implementation. A canonical library makes these gaps visible — when
   isolation is a single `bats_load_library` call in setup(), it's obvious
   which tests have it and which don't. Dodder's pattern of splitting isolation
   between application flags and test helpers is exactly the kind of ad-hoc
   layering the library is designed to replace with a single, auditable source.

4. **`GIT_CONFIG_SYSTEM` is unsterilized everywhere.** No consumer sets it.
   Risk is low (system `/etc/gitconfig` is typically empty) but the library
   should include `export GIT_CONFIG_SYSTEM=/dev/null` in `setup_test_home`.

5. **`GIT_CEILING_DIRECTORIES` is unset in most consumers.** Only spinclass
   sets it. Without it, git commands inside `$BATS_TEST_TMPDIR` can "see
   through" the test repo to the real repo that contains the project. The
   library should set it to `$BATS_TEST_TMPDIR` in `setup_test_home`.

6. **Pivy uses ad-hoc `HOME=` per-command instead of systematic isolation.**
   This is the most fragile pattern — it's easy to forget on one command and
   leak to real HOME. A library providing `setup_test_home` in `setup()` would
   eliminate this entire class of bug.

7. **`chflags_nouchg` is the most duplicated helper after `set_xdg`.** It
   appears in 5+ consumers and is critical for macOS teardown — dodder and git
   can set immutable flags that prevent `rm -rf`. Every consumer using
   `BATS_TEST_TMPDIR` on macOS needs it.

8. **`setup_test_repo` appears in 2-3 consumers with near-identical logic.**
   It naturally pairs with `setup_test_home` — grit's version calls it first.

## Interface

### Approach A: bats-island loadable library (recommended first)

Batman ships a new bats library, `bats-island`, loadable like any other batman
library:

```bash
bats_load_library bats-island
```

The library provides five functions spanning isolation, setup, and teardown:

#### `set_xdg <base-dir>`

Redirects all five XDG Base Directory variables into `<base-dir>/.xdg/` and
creates the directories:

- `XDG_DATA_HOME` -> `<base-dir>/.xdg/data`
- `XDG_CONFIG_HOME` -> `<base-dir>/.xdg/config`
- `XDG_STATE_HOME` -> `<base-dir>/.xdg/state`
- `XDG_CACHE_HOME` -> `<base-dir>/.xdg/cache`
- `XDG_RUNTIME_HOME` -> `<base-dir>/.xdg/runtime`

Includes input validation (non-empty argument, `realpath` succeeds) and always
creates directories. Dodder's version omitted `mkdir -p`; the library should not
repeat that mistake.

#### `setup_test_home`

Builds on `set_xdg` to fully isolate a test's HOME environment:

1. Saves `$HOME` to `$REAL_HOME`
2. Sets `$HOME` to `$BATS_TEST_TMPDIR/home`
3. Calls `set_xdg "$BATS_TEST_TMPDIR"`
4. Sets `GIT_CONFIG_GLOBAL` inside the isolated XDG config dir
5. Sets `GIT_CONFIG_SYSTEM=/dev/null` to prevent system config leakage
6. Sets `GIT_CEILING_DIRECTORIES=$BATS_TEST_TMPDIR` to prevent git from
   discovering parent repos outside the test directory
7. Seeds minimal git config (user.name, user.email, init.defaultBranch)
8. Sets `GIT_EDITOR=true` to prevent interactive editor hangs

Consumers call `setup_test_home` in their `setup()` function and get complete
HOME + XDG + git isolation with no additional code.

#### `setup_test_repo [dir]`

Creates an isolated git repository for testing. Calls `setup_test_home` first
if `$REAL_HOME` is not already set (i.e., isolation hasn't been established).

1. Sets `$TEST_REPO` to `<dir>` (default: `$BATS_TEST_TMPDIR/repo`)
2. Creates the directory and runs `git init`
3. Creates an initial commit (`file.txt` with "initial" content)

Exported variable `$TEST_REPO` points to the repo root for use in test
assertions.

#### `chflags_nouchg`

Teardown helper for macOS compatibility. Removes immutable flags (`nouchg`)
recursively on `$BATS_TEST_TMPDIR` so BATS can clean it up. No-ops gracefully on
Linux where `chflags` is unavailable. Call in `teardown()` for any test suite
where the binary under test may set immutable file flags.

#### `teardown_test_home`

Convenience teardown that calls `chflags_nouchg`. Pairs with `setup_test_home`
for symmetry in `setup()`/`teardown()`.

### Approach B: wrapper-level automatic isolation (future consideration)

The batman bats wrapper already creates the test execution environment (PATH
injection, sandcastle invocation, BATS_LIB_PATH setup). It can also sterilize
HOME and XDG before invoking bats:

1. The wrapper creates a temp dir (e.g., via `mktemp -d`)
2. Exports `REAL_HOME="$HOME"`, then `HOME=<tmpdir>/home`
3. Exports all five XDG variables pointing into `<tmpdir>/.xdg/`
4. Creates the directories
5. Configures `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM`, and
   `GIT_CEILING_DIRECTORIES`
6. Seeds minimal git config
7. Invokes bats (or sandcastle + bats) with these variables inherited

A `--no-isolation` flag allows opting out.

Since the wrapper runs before bats, `$BATS_TEST_TMPDIR` is not yet available.
Isolation is per-run (all tests in the invocation share the same fake HOME),
not per-test. The wrapper cleans up the temp dir on exit.

### Comparison

| | Approach A: library | Approach B: wrapper |
|---|---|---|
| Isolation granularity | Per-test | Per-run |
| Consumer effort | `bats_load_library` + call in setup() | Zero — automatic |
| Opt-out mechanism | Don't load the library | `--no-isolation` flag |
| Test-to-test leakage | Impossible | Possible (shared HOME) |
| Forgettable | Yes (the whole problem) | No — on by default |
| GIT_CONFIG_GLOBAL | Handled | Handled |
| GIT_CONFIG_SYSTEM | Handled | Handled |
| GIT_CEILING_DIRECTORIES | Handled | Handled |
| Composable with custom setup | Yes — call `set_xdg` with any base dir | Limited — vars are pre-set |

The "forgettable" row is the crux. The library approach reduces duplication but
still requires consumers to opt in — the same class of bug that motivates this
feature. The wrapper approach eliminates the category entirely: tests that run
through batman's bats wrapper are isolated by default.

Per-run isolation is sufficient for most test suites. Tests within a bats file
run sequentially, and the primary goal is preventing reads/writes against the
user's real `$HOME` and XDG directories. Inter-test isolation (preventing one
test from seeing another's XDG state) is a secondary concern that can be
addressed by consumers who need it.

The approaches are not mutually exclusive. The wrapper can provide per-run
isolation as the default safety net, while the library remains available for
consumers who need per-test granularity.

## Risks

### Approach A: library

**Low risk, incremental, reversible.**

- Each consumer opts in individually. If the interface turns out wrong,
  consumers revert one `bats_load_library` line and go back to their
  copy-pasted functions. No other test suite is affected.
- The code being consolidated (`set_xdg`, `setup_test_home`, `chflags_nouchg`,
  `setup_test_repo`) is already battle-tested across multiple consumers — the
  library just gives it an address.
- Does not solve the "forgettable" problem: consumers who don't know about the
  library will keep copy-pasting. But this is the existing failure mode, not a
  new one.

### Approach B: wrapper

**Higher risk, harder to reverse, changes semantics for all consumers.**

- **Rollback trap.** Once consumers remove their manual `set_xdg` code because
  "the wrapper handles it now," reverting the wrapper change breaks every
  consumer that cleaned up. This creates a hidden dependency with no
  compile-time signal.
- **Silent breakage of existing tests.** Every test suite that runs through the
  wrapper suddenly gets a different HOME. Tests that accidentally depend on the
  real HOME (reading a real gitconfig, a real `~/.config` file) will break.
  This is arguably correct, but it is surprise breakage in tests that were
  previously passing.
- **Subtle interaction with manual isolation.** Consumers that already call
  `set_xdg` in `setup()` will have two layers: the wrapper sets per-run vars,
  then `setup()` overwrites with per-test vars. This should be fine, but
  "should be fine" is exactly the kind of assumption that bites when
  integrating quickly.
- **Per-run leakage creates a new class of flaky test.** Test A writes
  `$HOME/.cache/something`, test B reads it. Tests pass together but fail in
  isolation, or vice versa. This is miserable to diagnose because it depends on
  test ordering.

### Shipping both simultaneously

**Highest risk.** Doubles the surface area and creates a decision point every
consumer must answer: "do I need the library too, or is the wrapper enough?"
That question is a new source of confusion — ironic for a feature motivated by
reducing confusion.

## Recommendation

Ship Approach A first. It is boring, incremental, and solves the DRY problem
today. Live with it for a release cycle. If consumers still forget to load the
library, that is real evidence for Approach B — and by then there is a stable
`bats-island` library to fall back on when wrapper-level isolation is not enough.

### Expected adoption per consumer

| Consumer | Action with Approach A |
|---|---|
| grit | Replace copy-pasted functions with `bats_load_library bats-island`; remove `set_xdg`, `setup_test_home`, `setup_test_repo`, `chflags_nouchg` definitions |
| dodder | Replace divergent `set_xdg` with library version for the 3 direct callers; replace `chflags_nouchg`; fix 12 broken tests separately (app-level bug) |
| pivy | Add `bats_load_library bats-island` + `setup_test_home` in setup(); remove ad-hoc `HOME=` overrides; adopt `chflags_nouchg` from library |
| purse-first (root) | Add `bats_load_library bats-island` + `setup_test_home` for hook_lifecycle and validate tests |
| batman example | Update template to use library instead of inline functions |

## Examples

### Before: copy-pasted boilerplate in every common.bash

```bash
# packages/grit/zz-tests_bats/common.bash
bats_load_library bats-support
bats_load_library bats-assert

set_xdg() {
  loc="$(realpath "$1" 2>/dev/null)"
  export XDG_DATA_HOME="$loc/.xdg/data"
  export XDG_CONFIG_HOME="$loc/.xdg/config"
  export XDG_STATE_HOME="$loc/.xdg/state"
  export XDG_CACHE_HOME="$loc/.xdg/cache"
  export XDG_RUNTIME_HOME="$loc/.xdg/runtime"
  mkdir -p "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "$XDG_STATE_HOME" \
    "$XDG_CACHE_HOME" "$XDG_RUNTIME_HOME"
}

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

setup_test_repo() {
  setup_test_home
  export TEST_REPO="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$TEST_REPO"
  git -C "$TEST_REPO" init
  echo "initial" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "initial commit"
}

chflags_and_rm() {
  chflags -R nouchg "$BATS_TEST_TMPDIR" 2>/dev/null || true
  rm -rf "$BATS_TEST_TMPDIR"
}

setup() {
  setup_test_home
}

teardown() {
  chflags_and_rm
}
```

### After: single library load

```bash
# packages/grit/zz-tests_bats/common.bash
bats_load_library bats-support
bats_load_library bats-assert
bats_load_library bats-island

setup() {
  setup_test_home
}

teardown() {
  teardown_test_home
}
```

### Git repo tests

```bash
bats_load_library bats-island

setup() {
  setup_test_repo
  # $TEST_REPO is now an isolated git repo with an initial commit.
  # HOME, XDG, and git config are all isolated.
}

teardown() {
  teardown_test_home
}
```

### Lightweight: XDG isolation without HOME or git config

```bash
bats_load_library bats-island

setup() {
  set_xdg "$BATS_TEST_TMPDIR"
}
```

### Both approaches combined (future, if Approach B is adopted)

```bash
# Wrapper provides per-run isolation (safety net).
# Library provides per-test isolation (strict independence).
bats_load_library bats-island

setup() {
  setup_test_home  # overrides wrapper-level vars with per-test dirs
}
```

## Limitations

- `setup_test_home` (both approaches) hardcodes git config values (user "Test
  User", email "test@example.com", defaultBranch "main") and `GIT_EDITOR=true`.
  These are suitable for most integration tests but are not configurable.
- Only git-specific env vars are handled (`GIT_CONFIG_GLOBAL`,
  `GIT_CONFIG_SYSTEM`, `GIT_CEILING_DIRECTORIES`, `GIT_EDITOR`). Other tools
  that ignore XDG (e.g., AWS CLI's `AWS_CONFIG_FILE`) must still be overridden
  by the consumer. No other tool-specific env vars (`GNUPGHOME`,
  `DOCKER_CONFIG`, `KUBECONFIG`, `CARGO_HOME`, etc.) were referenced by any
  consumer in the audit, so they are omitted.
- `set_xdg` uses `XDG_RUNTIME_HOME` rather than the spec's `XDG_RUNTIME_DIR`.
  This matches the existing convention across the codebase.
- `XDG_DATA_DIRS` and `XDG_CONFIG_DIRS` (the system-level search path
  variables) are not sterilized. No consumer referenced them in the audit.
- Approach B (wrapper-level) provides per-run isolation, not per-test. Tests
  that write to `$HOME` or XDG dirs can see each other's writes within the same
  bats invocation.
- The library does not retroactively fix application-level XDG overrides (e.g.,
  dodder's `-override-xdg-with-cwd`). However, by establishing test-level
  `set_xdg` as the standard isolation mechanism, the library discourages the
  pattern of building bespoke application-level overrides in the first place.
  When a canonical `bats_load_library bats-island` exists, there is less reason
  for applications to invent their own XDG isolation flags — and less chance of
  the coverage gaps that arise when isolation is split across layers.

## More Information

- Batman bats-testing skill: `packages/batman/skills/bats-testing/SKILL.md`
- XDG isolation patterns reference: `packages/batman/skills/bats-testing/references/patterns.md`
- Duplication sites (to be replaced by library):
  - `packages/grit/zz-tests_bats/common.bash` — `set_xdg`, `setup_test_home`, `setup_test_repo`, `chflags_nouchg`
  - `packages/batman/skills/bats-testing/examples/common.bash` — template copy
- Consumers with isolation gaps (to adopt library):
  - `~/eng/repos/dodder/zz-tests_bats/common.bash` — divergent `set_xdg`, duplicated `chflags_nouchg`
  - `~/eng/repos/pivy/zz-tests_bats/common.bash` — duplicated `chflags_nouchg`
  - `~/eng/repos/pivy/zz-tests_bats/pivy_agent.bats` — ad-hoc `HOME=` overrides
  - `zz-tests_bats/hook_lifecycle.bats` — reads `$HOME/.claude/` without isolation
