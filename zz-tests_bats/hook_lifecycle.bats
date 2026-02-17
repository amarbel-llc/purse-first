#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  purse_first="$(purse_first_bin)"
  result_path="${PURSE_FIRST_RESULT:-$BATS_CWD/result}"
}

function install_produces_tap_output { # @test
  # Install requires a configured claude CLI for plugin install steps.
  # In sandcastle, verify it produces TAP output and gets past marketplace add.
  run "$purse_first" install
  # May fail at plugin install step if claude isn't configured
  assert_output --partial "TAP version 14"
  assert_output --partial "add marketplace"
}

function installed_hooks_reference_valid_binary { # @test
  skip "TODO: hooks are not installed by purse-first install (issues #1, #10)"

  "$purse_first" install

  local settings="$HOME/.claude/settings.json"
  [[ -f "$settings" ]]

  # Extract hook binary path and verify it exists
  local hook_bin
  hook_bin=$(jq -r '.hooks.PreToolUse[0].hooks[0].command' "$settings" | awk '{print $1}')
  [[ -x "$hook_bin" ]]
}

function pretooluse_hook_is_blocking { # @test
  skip "TODO: hook.Install is dead code; PreToolUse should be blocking (issues #4, #5)"

  "$purse_first" install

  local settings="$HOME/.claude/settings.json"
  [[ -f "$settings" ]]

  local blocking
  blocking=$(jq -r '.hooks.PreToolUse[0].hooks[0].blocking' "$settings")
  [[ "$blocking" == "true" ]]
}

function marketplace_json_validates { # @test
  local marketplace
  marketplace="$(marketplace_result)"

  run "$purse_first" validate --type marketplace "$marketplace"
  assert_success
}

function chix_has_mappings { # @test
  skip "TODO: chix does not ship mappings.json yet (issue #7)"

  local mappings="$result_path/share/purse-first/chix/mappings.json"
  [[ -f "$mappings" ]]

  run jq -e '.server == "chix"' "$mappings"
  assert_success
}

function get_hubbed_has_mappings { # @test
  skip "TODO: get-hubbed does not ship mappings.json yet (issue #7)"

  local mappings="$result_path/share/purse-first/get-hubbed/mappings.json"
  [[ -f "$mappings" ]]

  run jq -e '.server == "get-hubbed"' "$mappings"
  assert_success
}
