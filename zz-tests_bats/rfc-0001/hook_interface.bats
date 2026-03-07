#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
}

# RFC-0001 section 2.2: must exit 0 on malformed stdin
function exit_0_on_malformed_stdin { # @test
  run bash -c 'echo "not valid json at all" | "$1" hook' -- "$PACKAGE_BIN"
  assert_success
}

# RFC-0001 section 2.2: must not deny on malformed stdin
function no_deny_on_malformed_stdin { # @test
  run bash -c 'echo "not valid json at all" | "$1" hook' -- "$PACKAGE_BIN"
  assert_success
  refute_output --partial '"deny"'
}

# RFC-0001 section 2.2: must exit 0 on empty stdin
function exit_0_on_empty_stdin { # @test
  run bash -c 'echo "" | "$1" hook' -- "$PACKAGE_BIN"
  assert_success
}
