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

function execute_squash_commits { # @test
  setup_multi_commit_scenario

  # Get the commit hashes
  local hash1 hash2 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Squash second into first, pick third
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"squash","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2" "$hash3")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # Should have 2 commits after squash (was 3)
  local count
  count=$(git -C "$TEST_REPO" log --oneline main..HEAD | wc -l | tr -d ' ')
  assert_equal "$count" "2"
}

function execute_drop_commit { # @test
  setup_multi_commit_scenario

  local hash1 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Pick first and third, implicitly drop second
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash3")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # Should have 2 commits (dropped one)
  local count
  count=$(git -C "$TEST_REPO" log --oneline main..HEAD | wc -l | tr -d ' ')
  assert_equal "$count" "2"

  # second.txt should not exist (commit was dropped)
  assert [ ! -f "$TEST_REPO/second.txt" ]
}

function execute_reorder_commits { # @test
  setup_multi_commit_scenario

  local hash1 hash2 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Reverse the order: third, second, first
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"pick","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash3" "$hash2" "$hash1")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # First commit should now be "feature: add third"
  local first_subject
  first_subject=$(git -C "$TEST_REPO" log --reverse --format=%s main..HEAD | head -1)
  assert_equal "$first_subject" "feature: add third"
}

function execute_reword_commit { # @test
  setup_multi_commit_scenario

  local hash1 hash2 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Reword the first commit
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"reword","hash":"%s","message":"renamed first commit"},{"action":"pick","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2" "$hash3")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # First commit should have the new message
  local first_subject
  first_subject=$(git -C "$TEST_REPO" log --reverse --format=%s main..HEAD | head -1)
  assert_equal "$first_subject" "renamed first commit"
}

function execute_validates_squash_not_first { # @test
  setup_multi_commit_scenario

  local hash1 hash2
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')

  # squash as first action should fail validation
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"squash","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2")"
  assert_success
  assert_output --partial "cannot be the first"
}

function execute_validates_reword_needs_message { # @test
  setup_multi_commit_scenario

  local hash1
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')

  # reword without message should fail validation
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"reword","hash":"%s"}]}' "$TEST_REPO" "$hash1")"
  assert_success
  assert_output --partial "message"
}

function execute_blocked_on_main { # @test
  setup_test_repo
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"HEAD~1","todo":[{"action":"pick","hash":"abc"}]}' "$TEST_REPO")"
  assert_success
  assert_output --partial "blocked"
}

function execute_conflict_returns_conflict_status { # @test
  setup_test_repo
  git -C "$TEST_REPO" checkout -b feature
  echo "feature change" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "feature: modify file"

  echo "more" > "$TEST_REPO/more.txt"
  git -C "$TEST_REPO" add more.txt
  git -C "$TEST_REPO" commit -m "feature: add more"

  git -C "$TEST_REPO" checkout main
  echo "main change" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "main: modify file"
  git -C "$TEST_REPO" checkout feature

  local hash1 hash2
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')

  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "conflict"
}

function execute_empty_todo_rejected { # @test
  setup_multi_commit_scenario
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[]}' "$TEST_REPO")"
  assert_success
  assert_output --partial "must not be empty"
}

function execute_rejects_when_rebase_in_progress { # @test
  setup_conflict_scenario
  # Start a regular rebase that will conflict
  git -C "$TEST_REPO" rebase main || true

  local hash1
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..ORIG_HEAD | sed -n '1p')

  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1")"
  assert_success
  assert_output --partial "already in progress"
}

function execute_explicit_drop { # @test
  setup_multi_commit_scenario

  local hash1 hash2 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Explicitly drop the second commit
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"drop","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2" "$hash3")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # second.txt should not exist
  assert [ ! -f "$TEST_REPO/second.txt" ]

  # Should have 2 commits
  local count
  count=$(git -C "$TEST_REPO" log --oneline main..HEAD | wc -l | tr -d ' ')
  assert_equal "$count" "2"
}

function execute_fixup_commits { # @test
  setup_multi_commit_scenario

  local hash1 hash2 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Fixup second into first (like squash but discard second's message)
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"fixup","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2" "$hash3")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # Should have 2 commits
  local count
  count=$(git -C "$TEST_REPO" log --oneline main..HEAD | wc -l | tr -d ' ')
  assert_equal "$count" "2"

  # First commit message should be the original (not combined)
  local first_subject
  first_subject=$(git -C "$TEST_REPO" log --reverse --format=%s main..HEAD | head -1)
  assert_equal "$first_subject" "feature: add first"
}
