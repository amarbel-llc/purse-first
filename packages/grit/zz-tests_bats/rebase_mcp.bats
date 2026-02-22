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
  # Create a feature branch at the same point as main
  git -C "$TEST_REPO" checkout -b feature
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
