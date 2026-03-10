#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_stubs
  create_repo
}

function apply_writes_claude_settings { # @test
  # Create a sweatfile with claude-allow rules
  mkdir -p "$TEST_REPO"
  cat > "$TEST_REPO/sweatfile" <<'EOF'
claude-allow = ["Bash(git *)"]
EOF

  cd "$TEST_REPO"
  run_sc new --no-attach test_settings
  assert_success

  local settings="$TEST_REPO/.worktrees/test_settings/.claude/settings.local.json"
  assert [ -f "$settings" ]

  # Check that claude-allow rules appear in the settings
  run cat "$settings"
  assert_output --partial '"Bash(git *)"'
  # Should also have the default Read rule
  assert_output --partial '"defaultMode"'
}

function apply_merges_hierarchy { # @test
  # Global sweatfile (hierarchy loads from $HOME/.config, not XDG_CONFIG_HOME)
  mkdir -p "$HOME/.config/spinclass"
  cat > "$HOME/.config/spinclass/sweatfile" <<'EOF'
claude-allow = ["Bash(git *)"]
EOF

  # Repo sweatfile
  cat > "$TEST_REPO/sweatfile" <<'EOF'
claude-allow = ["Bash(nix *)"]
EOF

  cd "$TEST_REPO"
  run_sc new --no-attach test_hierarchy
  assert_success

  local settings="$TEST_REPO/.worktrees/test_hierarchy/.claude/settings.local.json"
  assert [ -f "$settings" ]

  # Both rules should appear (global + repo merged)
  run cat "$settings"
  assert_output --partial '"Bash(git *)"'
  assert_output --partial '"Bash(nix *)"'
}

function apply_writes_envrc_when_flake_exists { # @test
  # Create a flake.nix in the repo
  cat > "$TEST_REPO/flake.nix" <<'EOF'
{ }
EOF
  git -C "$TEST_REPO" add flake.nix
  git -C "$TEST_REPO" commit -m "add flake.nix"

  cd "$TEST_REPO"
  run_sc new --no-attach test_envrc_flake
  assert_success

  local envrc="$TEST_REPO/.worktrees/test_envrc_flake/.envrc"
  assert [ -f "$envrc" ]
  run cat "$envrc"
  assert_output --partial "source_up"
  assert_output --partial "use flake"
}

function apply_skips_use_flake_without_flake_nix { # @test
  cd "$TEST_REPO"
  run_sc new --no-attach test_envrc_no_flake
  assert_success

  local envrc="$TEST_REPO/.worktrees/test_envrc_no_flake/.envrc"
  assert [ -f "$envrc" ]
  run cat "$envrc"
  assert_output --partial "source_up"
  assert_output --partial "PATH_add"
  refute_output --partial "use flake"
}
