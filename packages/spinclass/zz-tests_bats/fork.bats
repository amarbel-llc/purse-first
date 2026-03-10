#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_stubs
  create_repo
}

function fork_creates_new_branch { # @test
  cd "$TEST_REPO"
  local bin="${SPINCLASS_BIN:-spinclass}"
  "$bin" --format tap new --no-attach source_branch

  # Simulate being inside a session by setting SPINCLASS_SESSION
  local repo_name
  repo_name="$(basename "$TEST_REPO")"
  export SPINCLASS_SESSION="$repo_name/source_branch"

  # Run fork from the repo root (not the worktree) to avoid .git file issue
  run_sc fork new_branch
  assert_success

  # New worktree should exist
  assert [ -d "$TEST_REPO/.worktrees/new_branch" ]
  assert [ -f "$TEST_REPO/.worktrees/new_branch/.git" ]
}

function fork_auto_names_branch { # @test
  cd "$TEST_REPO"
  local bin="${SPINCLASS_BIN:-spinclass}"
  "$bin" --format tap new --no-attach auto_src

  local repo_name
  repo_name="$(basename "$TEST_REPO")"
  export SPINCLASS_SESSION="$repo_name/auto_src"

  # Run from repo root
  run_sc fork
  assert_success

  # Should have created auto_src-1
  assert [ -d "$TEST_REPO/.worktrees/auto_src-1" ]
}

function fork_requires_spinclass_session { # @test
  cd "$TEST_REPO"
  local bin="${SPINCLASS_BIN:-spinclass}"
  "$bin" --format tap new --no-attach session_test

  unset SPINCLASS_SESSION

  run_sc fork some_branch
  assert_failure
  assert_output --partial "SPINCLASS_SESSION"
}
