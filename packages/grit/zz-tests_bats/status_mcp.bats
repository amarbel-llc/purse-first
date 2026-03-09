#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
}

teardown() {
  teardown_test_home
}

function mcp_status_clean_repo { # @test
  setup_test_repo
  run run_grit_mcp "status" "$(printf '{"repo_path":"%s"}' "$TEST_REPO")"
  assert_success
  local head
  head=$(echo "$output" | jq -r '.branch.head')
  assert_equal "$head" "main"
  # No state on clean repo
  local state
  state=$(echo "$output" | jq -r '.state')
  assert_equal "$state" "null"
}

function mcp_status_during_rebase_conflict_shows_unmerged_entries { # @test
  setup_conflict_scenario
  # Start rebase (will conflict)
  run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"

  # Now check status
  run run_grit_mcp "status" "$(printf '{"repo_path":"%s"}' "$TEST_REPO")"
  assert_success

  # Entries should have UU state (both modified)
  local entry_state
  entry_state=$(echo "$output" | jq -r '.entries[] | select(.path == "file.txt") | .state')
  assert_equal "$entry_state" "UU"
}

function mcp_status_during_rebase_shows_operation { # @test
  setup_conflict_scenario
  run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"

  run run_grit_mcp "status" "$(printf '{"repo_path":"%s"}' "$TEST_REPO")"
  assert_success

  local operation
  operation=$(echo "$output" | jq -r '.state.operation')
  assert_equal "$operation" "rebase"
}

function mcp_status_during_rebase_shows_branch { # @test
  setup_conflict_scenario
  run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"

  run run_grit_mcp "status" "$(printf '{"repo_path":"%s"}' "$TEST_REPO")"
  assert_success

  local branch
  branch=$(echo "$output" | jq -r '.state.branch')
  assert_equal "$branch" "feature"
}

function mcp_status_during_rebase_shows_detached_head { # @test
  setup_conflict_scenario
  run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"

  run run_grit_mcp "status" "$(printf '{"repo_path":"%s"}' "$TEST_REPO")"
  assert_success

  local head
  head=$(echo "$output" | jq -r '.branch.head')
  assert_equal "$head" "(detached)"
}

function mcp_status_during_rebase_shows_step_progress { # @test
  setup_conflict_scenario
  run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"

  run run_grit_mcp "status" "$(printf '{"repo_path":"%s"}' "$TEST_REPO")"
  assert_success

  local step
  step=$(echo "$output" | jq -r '.state.step')
  assert_equal "$step" "1/1"
}

function mcp_status_no_state_after_rebase_abort { # @test
  setup_conflict_scenario
  run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"
  run_grit_mcp "rebase" "$(printf '{"repo_path":"%s","abort":true}' "$TEST_REPO")"

  run run_grit_mcp "status" "$(printf '{"repo_path":"%s"}' "$TEST_REPO")"
  assert_success

  local state
  state=$(echo "$output" | jq -r '.state')
  assert_equal "$state" "null"
}
