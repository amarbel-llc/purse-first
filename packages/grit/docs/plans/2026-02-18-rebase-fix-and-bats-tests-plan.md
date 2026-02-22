# Rebase Fix and BATS Test Infrastructure Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the rebase tool hanging on interactive prompts, bootstrap BATS integration tests using batman, and use TDD to verify the fixes.

**Architecture:** Two-layer fix — global safety in `git.Run()` (env vars + stdin) plus rebase-specific `os.Stat` fix. BATS tests validate behavior at both the git level and MCP JSON-RPC level.

**Tech Stack:** Go, BATS (batman), bats-assert, Nix flakes, JSON-RPC/MCP

---

### Task 1: Add batman to flake.nix

**Files:**
- Modify: `flake.nix`

**Step 1: Add batman flake input**

Add the batman input to the inputs section of `flake.nix`:

```nix
inputs = {
  nixpkgs-master.url = "github:NixOS/nixpkgs/b28c4999ed71543e71552ccfd0d7e68c581ba7e9";
  nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
  utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
  go.url = "github:friedenberg/eng?dir=devenvs/go";
  shell.url = "github:friedenberg/eng?dir=devenvs/shell";
  batman.url = "github:amarbel-llc/batman";
};
```

Add `batman` to the outputs function parameters:

```nix
outputs =
  {
    self,
    nixpkgs,
    utils,
    go,
    shell,
    nixpkgs-master,
    batman,
  }:
```

Add `batman.packages.${system}.bats` and `batman.packages.${system}.bats-libs` to devShell packages. Also add `jq` (needed for MCP integration tests):

```nix
devShells.default = pkgs.mkShell {
  packages = (with pkgs; [
    just
    jq
  ]) ++ [
    batman.packages.${system}.bats
    batman.packages.${system}.bats-libs
  ];

  inputsFrom = [
    go.devShells.${system}.default
    shell.devShells.${system}.default
  ];

  shellHook = ''
    echo "grit - dev environment"
  '';
};
```

**Step 2: Lock the new input**

Run: `nix flake lock` (use the nix MCP tool)

**Step 3: Verify the devShell builds**

Run: `nix develop --command bats --version`
Expected: bats version output (no errors)

**Step 4: Commit**

```
git add flake.nix flake.lock
git commit -m "build: add batman for bats integration testing"
```

---

### Task 2: Create BATS test infrastructure

**Files:**
- Create: `zz-tests_bats/justfile`
- Create: `zz-tests_bats/common.bash`

**Step 1: Create the test justfile**

Create `zz-tests_bats/justfile`:

```makefile
bats_timeout := "10"

test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --tap --jobs {{num_cpus()}} {{targets}}

test-tags *tags:
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --tap --jobs {{num_cpus()}} --filter-tags {{tags}} *.bats

test: (test-targets "*.bats")
```

Note: timeout set to 10 seconds since rebase operations involve real git commands.

**Step 2: Create common.bash**

Create `zz-tests_bats/common.bash`:

```bash
bats_load_library bats-support
bats_load_library bats-assert
bats_load_library bats-assert-additions

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

chflags_and_rm() {
  chflags -R nouchg "$BATS_TEST_TMPDIR" 2>/dev/null || true
  rm -rf "$BATS_TEST_TMPDIR"
}

# Create a git repo with an initial commit
setup_test_repo() {
  setup_test_home
  export TEST_REPO="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$TEST_REPO"
  git -C "$TEST_REPO" init
  echo "initial" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "initial commit"
}

# Create a conflict scenario:
# - main branch has one change to file.txt
# - feature branch has a different change to file.txt
# After calling, you're on the "feature" branch ready to rebase onto main.
setup_conflict_scenario() {
  setup_test_repo

  # Create divergent change on main
  echo "main change" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "main: modify file"

  # Create feature branch from initial commit with conflicting change
  git -C "$TEST_REPO" checkout -b feature HEAD~1
  echo "feature change" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "feature: modify file"
}

# Create a clean rebase scenario:
# - main branch has changes to file_a.txt
# - feature branch has changes to file_b.txt (no overlap)
# After calling, you're on the "feature" branch ready to rebase onto main.
setup_clean_rebase_scenario() {
  setup_test_repo

  # Create change on main to a different file
  echo "main addition" > "$TEST_REPO/file_a.txt"
  git -C "$TEST_REPO" add file_a.txt
  git -C "$TEST_REPO" commit -m "main: add file_a"

  # Create feature branch from initial commit with non-conflicting change
  git -C "$TEST_REPO" checkout -b feature HEAD~1
  echo "feature addition" > "$TEST_REPO/file_b.txt"
  git -C "$TEST_REPO" add file_b.txt
  git -C "$TEST_REPO" commit -m "feature: add file_b"
}

# Send a JSON-RPC tools/call request to grit and capture the response.
# Usage: run_grit_mcp <tool_name> <json_args>
# Sets $output to the parsed result content text (JSON), and $status to exit code.
run_grit_mcp() {
  local tool_name="$1"
  local tool_args="$2"
  local grit_bin="${GRIT_BIN:-grit}"

  local init_request='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}'
  local initialized_notification='{"jsonrpc":"2.0","method":"notifications/initialized"}'
  local call_request
  call_request=$(printf '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"%s","arguments":%s}}' "$tool_name" "$tool_args")

  local response
  response=$(printf '%s\n%s\n%s\n' "$init_request" "$initialized_notification" "$call_request" \
    | timeout --preserve-status 5s "$grit_bin" 2>/dev/null \
    | grep -F '"id":2' \
    | head -1)

  if [ -z "$response" ]; then
    echo "no response from grit"
    return 1
  fi

  # Extract the text content from the result
  echo "$response" | jq -r '.result.content[0].text'
}
```

**Step 3: Wire root justfile**

Add to the root `justfile`:

```makefile
# Run BATS integration tests
test-bats: build
  just zz-tests_bats/test

# Run all tests
test-all: test test-bats
```

**Step 4: Commit**

```
git add zz-tests_bats/justfile zz-tests_bats/common.bash justfile
git commit -m "test: add bats integration test infrastructure"
```

---

### Task 3: Write failing rebase.bats tests (TDD red phase)

**Files:**
- Create: `zz-tests_bats/rebase.bats`

**Step 1: Write the test file**

Create `zz-tests_bats/rebase.bats`:

```bash
#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
}

teardown() {
  chflags_and_rm
}

function clean_rebase_completes { # @test
  setup_clean_rebase_scenario
  run git -C "$TEST_REPO" rebase main
  assert_success
  # Verify both changes present after rebase
  assert [ -f "$TEST_REPO/file_a.txt" ]
  assert [ -f "$TEST_REPO/file_b.txt" ]
}

function rebase_with_conflicts_does_not_hang { # @test
  setup_conflict_scenario
  # This should fail (conflicts) but NOT hang
  run git -C "$TEST_REPO" rebase main
  assert_failure
  # Verify conflict markers exist
  run git -C "$TEST_REPO" diff --name-only --diff-filter=U
  assert_success
  assert_output "file.txt"
}

function continue_after_resolving_does_not_hang { # @test
  setup_conflict_scenario
  git -C "$TEST_REPO" rebase main || true

  # Resolve the conflict
  echo "resolved" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt

  # Continue should not hang (this is the key test)
  run git -C "$TEST_REPO" rebase --continue
  assert_success
}

function abort_rebase_succeeds { # @test
  setup_conflict_scenario
  git -C "$TEST_REPO" rebase main || true

  run git -C "$TEST_REPO" rebase --abort
  assert_success

  # Should be back on feature branch
  run git -C "$TEST_REPO" rev-parse --abbrev-ref HEAD
  assert_success
  assert_output "feature"
}

function skip_conflicting_commit { # @test
  setup_conflict_scenario
  git -C "$TEST_REPO" rebase main || true

  run git -C "$TEST_REPO" rebase --skip
  assert_success
}

function up_to_date_rebase { # @test
  setup_test_repo
  # Rebase main onto itself — already up to date
  run git -C "$TEST_REPO" rebase main
  assert_success
  assert_output --partial "is up to date"
}
```

**Step 2: Run tests to verify they work (baseline)**

Run: `nix develop --command just zz-tests_bats/test-targets rebase.bats`

These tests exercise raw git behavior and should pass — they establish the baseline. The `continue_after_resolving_does_not_hang` test is the critical one; it validates that `git rebase --continue` doesn't hang when `GIT_EDITOR` is properly set (the batman bats wrapper + test isolation handle this).

If `continue_after_resolving_does_not_hang` hangs, that confirms the bug at the git level. In that case, add `export GIT_EDITOR=true` to `setup_test_home()` and re-run to verify the fix works.

**Step 3: Commit**

```
git add zz-tests_bats/rebase.bats
git commit -m "test: add git-level rebase behavior tests"
```

---

### Task 4: Write failing MCP integration tests (TDD red phase)

**Files:**
- Create: `zz-tests_bats/rebase_mcp.bats`

**Step 1: Build grit**

Run: `nix build` (use the nix MCP build tool)

**Step 2: Write the MCP test file**

Create `zz-tests_bats/rebase_mcp.bats`:

```bash
#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  export GRIT_BIN="$BATS_TEST_DIRNAME/../result/bin/grit"
}

teardown() {
  chflags_and_rm
}

function mcp_clean_rebase { # @test
  setup_clean_rebase_scenario
  run run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"
  assert_success
  # Parse the JSON result
  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"
}

function mcp_rebase_with_conflicts_returns_conflict_status { # @test
  setup_conflict_scenario
  run run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"
  assert_success
  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "conflict"
  # Should list conflicted files
  local conflicts
  conflicts=$(echo "$output" | jq -r '.conflicts[]')
  assert_equal "$conflicts" "file.txt"
}

function mcp_continue_after_resolving { # @test
  setup_conflict_scenario
  # Start rebase (will conflict)
  run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"

  # Resolve conflict
  echo "resolved" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt

  # Continue — this is the hang test
  run run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","continue":true}' "$TEST_REPO")"
  assert_success
  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"
}

function mcp_abort_rebase { # @test
  setup_conflict_scenario
  run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"

  run run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","abort":true}' "$TEST_REPO")"
  assert_success
  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "aborted"
}

function mcp_skip_commit { # @test
  setup_conflict_scenario
  run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"

  run run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","skip":true}' "$TEST_REPO")"
  assert_success
  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "skipped"
}

function mcp_up_to_date { # @test
  setup_test_repo
  run run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"
  assert_success
  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "up_to_date"
}

function mcp_rebase_blocked_on_main { # @test
  setup_test_repo
  # We're on main — rebase should be blocked
  run run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"HEAD~1"}' "$TEST_REPO")"
  assert_success
  # Should be an error result
  assert_output --partial "blocked"
}
```

**Step 3: Run MCP tests to verify they fail**

Run: `nix develop --command just zz-tests_bats/test-targets rebase_mcp.bats`

Expected: Tests that involve `--continue` should hang/timeout (confirming the bug). Other tests may pass or fail depending on the `git test -d` bug.

**Step 4: Commit**

```
git add zz-tests_bats/rebase_mcp.bats
git commit -m "test: add MCP integration tests for rebase (red phase)"
```

---

### Task 5: Fix git.Run() — global interactive prompt suppression

**Files:**
- Modify: `internal/git/exec.go`

**Step 1: Add environment variables and stdin to git.Run()**

Update `internal/git/exec.go`:

```go
package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Run(ctx context.Context, dir string, args ...string) (string, error) {
	if strings.ContainsRune(dir, 0) {
		return "", fmt.Errorf("dir contains null byte")
	}

	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return "", fmt.Errorf("argument contains null byte")
		}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	// Prevent interactive prompts from hanging the process
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_EDITOR=true",
	)
	cmd.Stdin = nil

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, stderr.String())
	}

	return stdout.String(), nil
}
```

Key changes:
- `cmd.Env` explicitly set with `os.Environ()` plus `GIT_TERMINAL_PROMPT=0` and `GIT_EDITOR=true`
- `cmd.Stdin = nil` is already the default but making it explicit for clarity

**Step 2: Run Go unit tests**

Run: `nix develop --command go test ./...`
Expected: All existing unit tests pass

**Step 3: Commit**

```
git add internal/git/exec.go
git commit -m "fix: prevent git from hanging on interactive prompts"
```

---

### Task 6: Fix rebase.go — broken rebase-in-progress check

**Files:**
- Modify: `internal/tools/rebase.go`

**Step 1: Replace git test -d with os.Stat**

In `internal/tools/rebase.go`, add `"os"` and `"path/filepath"` to the imports and replace the broken check at lines 133-137.

Old code:

```go
// Check for existing rebase state
rebaseDir := ".git/rebase-merge"
if _, err := git.Run(ctx, params.RepoPath, "test", "-d", rebaseDir); err == nil {
    return protocol.ErrorResult("a rebase operation is already in progress; use continue, abort, or skip"), nil
}
```

New code:

```go
// Check for existing rebase state
rebaseMergeDir := filepath.Join(params.RepoPath, ".git", "rebase-merge")
rebaseApplyDir := filepath.Join(params.RepoPath, ".git", "rebase-apply")
if _, err := os.Stat(rebaseMergeDir); err == nil {
    return protocol.ErrorResult("a rebase operation is already in progress; use continue, abort, or skip"), nil
}
if _, err := os.Stat(rebaseApplyDir); err == nil {
    return protocol.ErrorResult("a rebase operation is already in progress; use continue, abort, or skip"), nil
}
```

Note: git uses both `.git/rebase-merge` (interactive rebase) and `.git/rebase-apply` (am-based rebase), so check both.

**Step 2: Run Go unit tests**

Run: `nix develop --command go test ./...`
Expected: All existing unit tests pass

**Step 3: Commit**

```
git add internal/tools/rebase.go
git commit -m "fix: use os.Stat for rebase-in-progress detection"
```

---

### Task 7: Rebuild and run all tests (TDD green phase)

**Step 1: Rebuild grit**

Run: `nix build` (use the nix MCP build tool)

**Step 2: Run git-level rebase tests**

Run: `nix develop --command just zz-tests_bats/test-targets rebase.bats`
Expected: All 6 tests pass

**Step 3: Run MCP integration tests**

Run: `nix develop --command just zz-tests_bats/test-targets rebase_mcp.bats`
Expected: All 7 tests pass (including continue — no hang)

**Step 4: Run all tests together**

Run: `nix develop --command just zz-tests_bats/test`
Expected: All tests pass with TAP output

**Step 5: Run Go tests too**

Run: `nix develop --command go test ./...`
Expected: All pass

**Step 6: Update gomod2nix if needed**

If imports changed, run: `nix develop --command gomod2nix`

**Step 7: Commit any adjustments**

If any test adjustments were needed during the green phase, commit them:

```
git add -A
git commit -m "test: fix integration tests (green phase)"
```

---

### Task 8: Wire test-bats into root justfile

**Files:**
- Modify: `justfile`

**Step 1: Add bats targets to root justfile**

Add after the existing `test-v` target:

```makefile
# Run BATS integration tests
test-bats: build
  just zz-tests_bats/test

# Run all tests (Go + BATS)
test-all: test test-bats
```

**Step 2: Run the full test suite**

Run: `nix develop --command just test-all`
Expected: Go tests pass, then bats tests pass

**Step 3: Commit**

```
git add justfile
git commit -m "build: wire bats tests into root justfile"
```
