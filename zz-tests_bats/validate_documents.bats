#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  purse_first="$(purse_first_bin)"
}

function empty_json_fails_plugin_validation { # @test
  run "$purse_first" validate --type plugin <(echo '{}')
  assert_failure
}

function bad_name_fails_validation { # @test
  run "$purse_first" validate --type plugin <(echo '{"name":"Bad Name"}')
  assert_failure
}

function minimal_valid_plugin_passes { # @test
  local doc='{"name":"test-plugin","mcpServers":{"test-plugin":{"type":"stdio","command":"test-plugin"}}}'
  run "$purse_first" validate --type plugin <(echo "$doc")
  assert_success
}

function stdin_without_type_fails { # @test
  run sh -c 'echo "{}" | "$1" validate -' -- "$purse_first"
  assert_failure
}

function stdin_with_type_works { # @test
  run sh -c 'echo "{\"name\":\"x\",\"mcpServers\":{\"x\":{\"type\":\"stdio\",\"command\":\"x\"}}}" | "$1" validate --type plugin -' -- "$purse_first"
  assert_success
}

function directory_validation_finds_plugin { # @test
  local dir="$BATS_TEST_TMPDIR/test-dir"
  mkdir -p "$dir/.claude-plugin"
  echo '{"name":"x","mcpServers":{"x":{"type":"stdio","command":"x"}}}' > "$dir/.claude-plugin/plugin.json"
  run "$purse_first" validate "$dir"
  assert_success
}

function directory_without_documents_fails { # @test
  local dir="$BATS_TEST_TMPDIR/empty-dir"
  mkdir -p "$dir"
  run "$purse_first" validate "$dir"
  assert_failure
}

function strict_mode_fails_on_warnings { # @test
  run "$purse_first" validate --strict --type plugin <(echo '{"name":"x"}')
  assert_failure
}
