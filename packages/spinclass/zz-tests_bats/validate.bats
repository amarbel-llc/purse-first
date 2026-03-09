#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  create_repo
}

function validate_valid_sweatfile { # @test
  cat > "$TEST_REPO/sweatfile" <<'EOF'
claude-allow = ["Bash(git *)"]
git-excludes = [".worktrees"]
EOF

  cd "$TEST_REPO"
  run_sc validate
  assert_success
}

function validate_invalid_syntax { # @test
  cat > "$TEST_REPO/sweatfile" <<'EOF'
this is not valid toml [[[
EOF

  cd "$TEST_REPO"
  run_sc validate
  assert_failure
}

function validate_invalid_claude_allow { # @test
  cat > "$TEST_REPO/sweatfile" <<'EOF'
claude-allow = ["(unclosed"]
EOF

  cd "$TEST_REPO"
  run_sc validate
  assert_failure
  assert_output --partial "unmatched parenthesis"
}

function validate_unknown_field { # @test
  cat > "$TEST_REPO/sweatfile" <<'EOF'
bogus_field = "should fail"
EOF

  cd "$TEST_REPO"
  run_sc validate
  assert_failure
  assert_output --partial "unknown field"
}
