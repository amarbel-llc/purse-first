#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
}

# RFC-0001 section 2.1: directory mode writes plugin.json
function directory_mode_writes_plugin_json { # @test
  run_package_bin generate-plugin "$BATS_TEST_TMPDIR"
  assert_success

  # Find plugin.json under the output directory (package name varies)
  local plugin_json
  plugin_json=$(find "$BATS_TEST_TMPDIR/share/purse-first" -name plugin.json -type f)
  [[ -n "$plugin_json" ]]
}

# RFC-0001 section 2.1: plugin.name must equal directory name
function plugin_name_equals_dirname { # @test
  run_package_bin generate-plugin "$BATS_TEST_TMPDIR"
  assert_success

  local pkg_dir
  pkg_dir=$(find "$BATS_TEST_TMPDIR/share/purse-first" -mindepth 1 -maxdepth 1 -type d)
  local dirname
  dirname=$(basename "$pkg_dir")

  local name
  name=$(jq -r '.name' "$pkg_dir/.claude-plugin/plugin.json")
  assert_equal "$name" "$dirname"
}

# RFC-0001 section 2.1: mcpServers must have at least one entry
function mcp_servers_has_at_least_one_entry { # @test
  run_package_bin generate-plugin "$BATS_TEST_TMPDIR"
  assert_success

  local plugin_json
  plugin_json=$(find "$BATS_TEST_TMPDIR/share/purse-first" -name plugin.json -type f)

  local count
  count=$(jq '.mcpServers | length' "$plugin_json")
  [[ "$count" -ge 1 ]]
}

# RFC-0001 section 2.1: command field must be a bare name (no /)
function command_field_is_bare_name { # @test
  run_package_bin generate-plugin "$BATS_TEST_TMPDIR"
  assert_success

  local plugin_json
  plugin_json=$(find "$BATS_TEST_TMPDIR/share/purse-first" -name plugin.json -type f)

  # Check every command field contains no /
  local commands_with_slash
  commands_with_slash=$(jq -r '.mcpServers[].command // empty' "$plugin_json" | grep -c '/' || true)
  assert_equal "$commands_with_slash" "0"
}

# RFC-0001 section 2.1: directory mode must be idempotent
function directory_mode_is_idempotent { # @test
  run_package_bin generate-plugin "$BATS_TEST_TMPDIR"
  assert_success

  local plugin_json
  plugin_json=$(find "$BATS_TEST_TMPDIR/share/purse-first" -name plugin.json -type f)
  local first_run
  first_run=$(cat "$plugin_json")

  # Run again
  run_package_bin generate-plugin "$BATS_TEST_TMPDIR"
  assert_success

  local second_run
  second_run=$(cat "$plugin_json")
  assert_equal "$first_run" "$second_run"
}

# RFC-0001 section 2.1: PWD default writes to cwd
function pwd_default_writes_to_cwd { # @test
  local workdir="$BATS_TEST_TMPDIR/pwd-test"
  mkdir -p "$workdir"

  run bash -c "cd '$workdir' && '$PACKAGE_BIN' generate-plugin"
  assert_success

  local plugin_json
  plugin_json=$(find "$workdir/share/purse-first" -name plugin.json -type f)
  [[ -n "$plugin_json" ]]
}

# RFC-0001 section 2.1: stdout mode outputs valid JSON
function stdout_mode_outputs_valid_json { # @test
  run_package_bin generate-plugin -
  assert_success

  echo "$output" | jq -e '.name' >/dev/null
}

# RFC-0001 section 2.1: stdout mode must not write files
function stdout_mode_writes_no_files { # @test
  local workdir="$BATS_TEST_TMPDIR/stdout-test"
  mkdir -p "$workdir"

  run bash -c "cd '$workdir' && '$PACKAGE_BIN' generate-plugin -"
  assert_success

  local file_count
  file_count=$(find "$workdir" -type f | wc -l)
  assert_equal "$file_count" "0"
}
