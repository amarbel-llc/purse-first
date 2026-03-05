#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  bats_load_library bats-island
}

function set_xdg_creates_directories { # @test
  set_xdg "$BATS_TEST_TMPDIR"
  local resolved
  resolved="$(realpath "$BATS_TEST_TMPDIR")"
  [[ -d "$XDG_DATA_HOME" ]]
  [[ -d "$XDG_CONFIG_HOME" ]]
  [[ -d "$XDG_STATE_HOME" ]]
  [[ -d "$XDG_CACHE_HOME" ]]
  [[ -d "$XDG_RUNTIME_HOME" ]]
  [[ "$XDG_DATA_HOME" == "$resolved/.xdg/data" ]]
  [[ "$XDG_CONFIG_HOME" == "$resolved/.xdg/config" ]]
  [[ "$XDG_STATE_HOME" == "$resolved/.xdg/state" ]]
  [[ "$XDG_CACHE_HOME" == "$resolved/.xdg/cache" ]]
  [[ "$XDG_RUNTIME_HOME" == "$resolved/.xdg/runtime" ]]
}

function set_xdg_fails_on_empty_arg { # @test
  run set_xdg ""
  assert_failure
  assert_output --partial "base directory argument required"
}

function set_xdg_fails_on_missing_arg { # @test
  run set_xdg
  assert_failure
  assert_output --partial "base directory argument required"
}

function setup_test_home_isolates_home { # @test
  local original_home="$HOME"
  setup_test_home
  [[ "$HOME" != "$original_home" ]]
  [[ "$HOME" == "$BATS_TEST_TMPDIR/home" ]]
  [[ "$REAL_HOME" == "$original_home" ]]
}

function setup_test_home_sets_git_config { # @test
  setup_test_home
  [[ -n "$GIT_CONFIG_GLOBAL" ]]
  [[ "$GIT_CONFIG_SYSTEM" == "/dev/null" ]]
}

function setup_test_home_sets_ceiling_directories { # @test
  setup_test_home
  [[ "$GIT_CEILING_DIRECTORIES" == "$BATS_TEST_TMPDIR" ]]
}

function setup_test_repo_creates_git_repo { # @test
  setup_test_repo
  [[ -d "$TEST_REPO/.git" ]]
  run git -C "$TEST_REPO" log --oneline
  assert_success
  assert_output --partial "initial commit"
}

function setup_test_repo_calls_setup_test_home { # @test
  setup_test_repo
  [[ -n "$REAL_HOME" ]]
}

function setup_test_repo_accepts_custom_dir { # @test
  setup_test_repo "$BATS_TEST_TMPDIR/custom"
  [[ "$TEST_REPO" == "$BATS_TEST_TMPDIR/custom" ]]
  [[ -d "$TEST_REPO/.git" ]]
}

function chflags_nouchg_clears_flags { # @test
  echo "marker" > "$BATS_TEST_TMPDIR/marker.txt"
  chflags_nouchg
  [[ -f "$BATS_TEST_TMPDIR/marker.txt" ]]
}
