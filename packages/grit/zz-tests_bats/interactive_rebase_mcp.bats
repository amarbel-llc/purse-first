#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
}

teardown() {
  teardown_test_home
}

# Helper: create a repo with 3 commits on feature branch ahead of main
setup_multi_commit_scenario() {
  setup_test_repo

  # Create feature branch with multiple commits
  git -C "$TEST_REPO" checkout -b feature
  echo "first" > "$TEST_REPO/first.txt"
  git -C "$TEST_REPO" add first.txt
  git -C "$TEST_REPO" commit -m "feature: add first"

  echo "second" > "$TEST_REPO/second.txt"
  git -C "$TEST_REPO" add second.txt
  git -C "$TEST_REPO" commit -m "feature: add second"

  echo "third" > "$TEST_REPO/third.txt"
  git -C "$TEST_REPO" add third.txt
  git -C "$TEST_REPO" commit -m "feature: add third"
}

function plan_returns_commit_list { # @test
  setup_multi_commit_scenario
  run run_grit_mcp "interactive_rebase_plan" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "plan"

  local count
  count=$(echo "$output" | jq '.commits | length')
  assert_equal "$count" "3"

  # Commits should be in chronological order (oldest first)
  local first_subject
  first_subject=$(echo "$output" | jq -r '.commits[0].subject')
  assert_equal "$first_subject" "feature: add first"
}

function plan_up_to_date { # @test
  setup_test_repo
  git -C "$TEST_REPO" checkout -b feature
  run run_grit_mcp "interactive_rebase_plan" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "up_to_date"

  local count
  count=$(echo "$output" | jq '.commits | length')
  assert_equal "$count" "0"
}

function plan_blocked_on_main { # @test
  setup_test_repo
  run run_grit_mcp "interactive_rebase_plan" "$(printf '{"repo_path":"%s","upstream":"HEAD~1"}' "$TEST_REPO")"
  assert_success
  assert_output --partial "blocked"
}
