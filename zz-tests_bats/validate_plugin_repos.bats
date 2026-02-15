#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
}

function grit_repo_validates { # @test
  run claude plugin validate /home/sasha/eng/repos/grit
  assert_success
}

function get_hubbed_repo_validates { # @test
  run claude plugin validate /home/sasha/eng/repos/get-hubbed
  assert_success
}

function lux_repo_validates { # @test
  run claude plugin validate /home/sasha/eng/repos/lux
  assert_success
}

function nix_mcp_server_repo_validates { # @test
  run claude plugin validate /home/sasha/eng/repos/nix-mcp-server
  assert_success
}
