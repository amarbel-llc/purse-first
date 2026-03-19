#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  purse_first="$(purse_first_bin)"
}

function validate_mcp_nonexistent_binary_fails { # @test
  run "$purse_first" validate-mcp /nonexistent/binary
  assert_failure
}

function validate_mcp_grit_passes { # @test
  if ! command -v grit &>/dev/null; then
    skip "grit not on PATH"
  fi
  run "$purse_first" validate-mcp "$(command -v grit)"
  assert_success
  assert_output --partial "tools/list"
}

function validate_type_mcp_grit_passes { # @test
  if ! command -v grit &>/dev/null; then
    skip "grit not on PATH"
  fi
  run "$purse_first" validate --type mcp "$(command -v grit)"
  assert_success
  assert_output --partial "tools/list"
}
