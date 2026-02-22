#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output

  # Isolate XDG directories
  set_xdg "$BATS_TEST_TMPDIR"

  # Copy fixtures or initialize state as needed
  # copy_from_version "$DIR"
}

teardown() {
  # Runs after each test regardless of success/failure. BATS_TEST_TMPDIR is
  # cleaned up automatically by bats — use teardown for resources created
  # outside managed temp dirs (e.g. background processes, external files).
}

function init_creates_directory { # @test
  run_cmd init
  assert_success
  assert [ -d "$BATS_TEST_TMPDIR/.app" ]
}

function add_produces_expected_output { # @test
  run_cmd init
  assert_success

  run_cmd add "item-name"
  assert_success
  assert_output --partial "added: item-name"
}

function list_shows_all_items_regardless_of_order { # @test
  run_cmd init
  assert_success

  run_cmd add "charlie"
  run_cmd add "alice"
  run_cmd add "bob"

  run_cmd list
  assert_success
  assert_output_unsorted - <<-EOM
    alice
    bob
    charlie
  EOM
}

function invalid_command_fails { # @test
  run_cmd nonexistent
  assert_failure
  assert_output --partial "unknown command"
}

# bats test_tags=slow
function large_import_completes { # @test
  run_cmd init
  assert_success

  run_cmd import --source /path/to/fixtures
  assert_success
  assert_line --index 0 "import complete"
}
