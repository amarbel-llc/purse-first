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
