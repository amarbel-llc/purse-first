#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  purse_first="$(purse_first_bin)"
}

function claude_validates_grit { # @test
  run claude plugin validate "$(plugin_share_dir grit)/plugin.json"
  assert_success
}

function claude_validates_get_hubbed { # @test
  run claude plugin validate "$(plugin_share_dir get-hubbed)/plugin.json"
  assert_success
}

function claude_validates_lux { # @test
  run claude plugin validate "$(plugin_share_dir lux)/plugin.json"
  assert_success
}

function claude_validates_chix { # @test
  run claude plugin validate "$(plugin_share_dir chix)/plugin.json"
  assert_success
}

function claude_validates_purse_first { # @test
  run claude plugin validate "$(plugin_share_dir purse-first)/plugin.json"
  assert_success
}

function claude_validates_robin { # @test
  run claude plugin validate "$(plugin_share_dir robin)/plugin.json"
  assert_success
}

function claude_validates_tap_dancer { # @test
  run claude plugin validate "$(plugin_share_dir tap-dancer)/plugin.json"
  assert_success
}

function purse_first_validates_grit { # @test
  run "$purse_first" validate "$(plugin_share_dir grit)"
  assert_success
}

function purse_first_validates_get_hubbed { # @test
  run "$purse_first" validate "$(plugin_share_dir get-hubbed)"
  assert_success
}

function purse_first_validates_lux { # @test
  run "$purse_first" validate "$(plugin_share_dir lux)"
  assert_success
}

function purse_first_validates_chix { # @test
  run "$purse_first" validate "$(plugin_share_dir chix)"
  assert_success
}

function purse_first_validates_purse_first { # @test
  run "$purse_first" validate "$(plugin_share_dir purse-first)"
  assert_success
}

function purse_first_validates_robin { # @test
  run "$purse_first" validate "$(plugin_share_dir robin)"
  assert_success
}

function purse_first_validates_tap_dancer { # @test
  run "$purse_first" validate "$(plugin_share_dir tap-dancer)"
  assert_success
}
