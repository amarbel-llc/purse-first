#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_stubs
  create_repo
}

function spinclass_new_creates_worktree { # @test
  cd "$TEST_REPO"
  run_sc new test-branch --no-attach

  assert_success
  assert [ -d "$TEST_REPO/.worktrees/test-branch" ]
  # Should be a git worktree (has .git file, not directory)
  assert [ -f "$TEST_REPO/.worktrees/test-branch/.git" ]
  # Branch should exist
  run git -C "$TEST_REPO" rev-parse --verify refs/heads/test-branch
  assert_success
}

function spinclass_new_auto_name { # @test
  cd "$TEST_REPO"
  run_sc new --no-attach

  assert_success
  # Should have created a worktree dir — at least one entry in .worktrees/
  run ls "$TEST_REPO/.worktrees/"
  assert_success
  assert [ -n "$output" ]
}

function spinclass_new_no_attach_skips_zmx { # @test
  cd "$TEST_REPO"
  run_sc new --no-attach test-noattach

  assert_success
  assert [ -d "$TEST_REPO/.worktrees/test-noattach" ]
  # zmx should NOT have been called
  assert [ ! -f "$BATS_TEST_TMPDIR/stubs/zmx.log" ]
}

function spinclass_new_idempotent { # @test
  cd "$TEST_REPO"
  run_sc new --no-attach test-idem
  assert_success

  # Second run should succeed with SKIP
  run_sc new --no-attach test-idem
  assert_success
  assert_output --partial "SKIP"
}

function spinclass_status_shows_worktrees { # @test
  cd "$TEST_REPO"
  # Create some worktrees
  spinclass --format tap new --no-attach branch-a
  spinclass --format tap new --no-attach branch-b

  run_sc status
  assert_success
  assert_output --partial "branch-a"
  assert_output --partial "branch-b"
}

function spinclass_merge_fast_forwards { # @test
  cd "$TEST_REPO"
  spinclass --format tap new --no-attach merge-test

  local wt="$TEST_REPO/.worktrees/merge-test"

  # Make a commit on the worktree branch
  echo "new content" > "$wt/new-file.txt"
  git -C "$wt" add new-file.txt
  git -C "$wt" commit -m "add new file"

  # Clean untracked files created by sweatfile apply so worktree remove succeeds
  git -C "$wt" clean -fd

  # Merge from the main repo
  run_sc merge merge-test
  assert_success

  # Commit should now be on main
  run git -C "$TEST_REPO" log --oneline --all
  assert_output --partial "add new file"

  # Worktree should be removed
  assert [ ! -d "$wt" ]
}

function spinclass_clean_removes_merged { # @test
  cd "$TEST_REPO"
  spinclass --format tap new --no-attach clean-test

  # Clean untracked files so worktree remove succeeds
  git -C "$TEST_REPO/.worktrees/clean-test" clean -fd

  # Merge the worktree first (makes the branch fully merged)
  spinclass --format tap merge clean-test

  # Create another worktree that IS merged (no extra commits)
  spinclass --format tap new --no-attach clean-noop

  # Clean untracked files from sweatfile apply
  git -C "$TEST_REPO/.worktrees/clean-noop" clean -fd

  run_sc clean
  assert_success
  # The noop worktree with zero commits ahead should be cleaned
  assert [ ! -d "$TEST_REPO/.worktrees/clean-noop" ]
}
