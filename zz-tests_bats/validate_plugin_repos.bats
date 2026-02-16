#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  purse_first="$(purse_first_bin)"
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

function chix_repo_validates { # @test
  run claude plugin validate /home/sasha/eng/repos/chix
  assert_success
}

function purse_first_grit_repo_validates { # @test
  run "$purse_first" validate /home/sasha/eng/repos/grit
  assert_success
}

function purse_first_get_hubbed_repo_validates { # @test
  run "$purse_first" validate /home/sasha/eng/repos/get-hubbed
  assert_success
}

function purse_first_lux_repo_validates { # @test
  run "$purse_first" validate /home/sasha/eng/repos/lux
  assert_success
}

function purse_first_chix_repo_validates { # @test
  run "$purse_first" validate /home/sasha/eng/repos/chix
  assert_success
}
