#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  BATS_TMPDIR="${BATS_TMPDIR:-/tmp}"
  TEST_TMPDIR="$(mktemp -d "${BATS_TMPDIR}/bats-wrapper-XXXXXX")"

  require_bin BATS_WRAPPER
  export BATS_WRAPPER
}

teardown() {
  rm -rf "$TEST_TMPDIR"
}

function bats_wrapper_runs_tests { # @test
  cat >"${TEST_TMPDIR}/truth.bats" <<'EOF'
#! /usr/bin/env bats
function truth { # @test
  true
}
EOF
  run "$BATS_WRAPPER" --tap "${TEST_TMPDIR}/truth.bats"
  assert_success
  assert_output --partial "ok 1"
}

function bats_wrapper_denies_config_read { # @test
  # Verify sandcastle replaces $HOME/.config with empty tmpfs.
  # The inner test asserts the directory is empty or missing.
  cat >"${TEST_TMPDIR}/read_config.bats" <<'INNER'
#! /usr/bin/env bats
function config_dir_is_empty_or_missing { # @test
  if [[ -d "$HOME/.config" ]]; then
    contents="$(ls "$HOME/.config")"
    [ -z "$contents" ]
  fi
}
INNER
  run "$BATS_WRAPPER" --tap "${TEST_TMPDIR}/read_config.bats"
  assert_success
  assert_output --partial "ok 1"
}

function bats_wrapper_allows_tmp_write { # @test
  cat >"${TEST_TMPDIR}/write_tmp.bats" <<'EOF'
#! /usr/bin/env bats
function write_tmp { # @test
  echo "test" > /tmp/bats-wrapper-test-$$
  rm -f /tmp/bats-wrapper-test-$$
}
EOF
  run "$BATS_WRAPPER" --tap "${TEST_TMPDIR}/write_tmp.bats"
  assert_success
}

function bats_wrapper_prepends_bin_dir_to_path { # @test
  mkdir -p "${TEST_TMPDIR}/fake-bin"
  cat >"${TEST_TMPDIR}/fake-bin/my-tool" <<'EOF'
#!/usr/bin/env bash
echo "fake-tool-output"
EOF
  chmod +x "${TEST_TMPDIR}/fake-bin/my-tool"

  cat >"${TEST_TMPDIR}/bin_dir.bats" <<'INNER'
#! /usr/bin/env bats
function finds_tool_on_path { # @test
  run my-tool
  [ "$status" -eq 0 ]
  [ "$output" = "fake-tool-output" ]
}
INNER
  run "$BATS_WRAPPER" --bin-dir "${TEST_TMPDIR}/fake-bin" --no-sandbox "${TEST_TMPDIR}/bin_dir.bats"
  assert_success
  assert_output --partial "ok 1"
}

function bats_wrapper_supports_multiple_bin_dirs { # @test
  mkdir -p "${TEST_TMPDIR}/bin-a" "${TEST_TMPDIR}/bin-b"
  cat >"${TEST_TMPDIR}/bin-a/tool-a" <<'EOF'
#!/usr/bin/env bash
echo "from-a"
EOF
  cat >"${TEST_TMPDIR}/bin-b/tool-b" <<'EOF'
#!/usr/bin/env bash
echo "from-b"
EOF
  chmod +x "${TEST_TMPDIR}/bin-a/tool-a" "${TEST_TMPDIR}/bin-b/tool-b"

  cat >"${TEST_TMPDIR}/multi.bats" <<'INNER'
#! /usr/bin/env bats
function finds_both_tools { # @test
  run tool-a
  [ "$status" -eq 0 ]
  [ "$output" = "from-a" ]
  run tool-b
  [ "$status" -eq 0 ]
  [ "$output" = "from-b" ]
}
INNER
  run "$BATS_WRAPPER" --bin-dir "${TEST_TMPDIR}/bin-a" --bin-dir "${TEST_TMPDIR}/bin-b" --no-sandbox "${TEST_TMPDIR}/multi.bats"
  assert_success
  assert_output --partial "ok 1"
}

function bats_wrapper_defaults_to_tap_output { # @test
  cat >"${TEST_TMPDIR}/tap_default.bats" <<'EOF'
#! /usr/bin/env bats
function truth { # @test
  true
}
EOF
  run "$BATS_WRAPPER" --no-sandbox "${TEST_TMPDIR}/tap_default.bats"
  assert_success
  # TAP output starts with version or plan line
  assert_line --index 0 --regexp "^(TAP version|1\.\.)"
}

function bats_wrapper_no_sandbox_skips_sandcastle { # @test
  cat >"${TEST_TMPDIR}/no_sandbox.bats" <<'EOF'
#! /usr/bin/env bats
function can_read_home_config { # @test
  [[ -d "$HOME/.config" ]] || skip "no .config dir"
  ls "$HOME/.config" >/dev/null
}
EOF
  run "$BATS_WRAPPER" --no-sandbox "${TEST_TMPDIR}/no_sandbox.bats"
  assert_success
}

function bats_wrapper_no_tempdir_cleanup_preserves_tmpdir { # @test
  cat >"${TEST_TMPDIR}/preserve.bats" <<'EOF'
#! /usr/bin/env bats
function creates_file_in_tmpdir { # @test
  echo "marker" > "${BATS_TEST_TMPDIR}/marker.txt"
}
EOF
  run "$BATS_WRAPPER" --no-tempdir-cleanup "${TEST_TMPDIR}/preserve.bats"
  assert_success
  assert_output --partial "ok 1"
  # Extract BATS_RUN_TMPDIR from output (printed by --no-tempdir-cleanup)
  bats_run_dir="$(echo "$output" | grep "BATS_RUN_TMPDIR" | cut -d' ' -f2)"
  [[ -n "$bats_run_dir" ]]
  # Verify the temp dir survived sandcastle cleanup
  [[ -d "$bats_run_dir" ]]
  [[ -f "$bats_run_dir/test/1/marker.txt" ]]
  # Clean up manually
  rm -rf "$bats_run_dir"
}

function bats_wrapper_hide_passing_filters_ok_lines { # @test
  cat >"${TEST_TMPDIR}/mixed.bats" <<'EOF'
#! /usr/bin/env bats
function passing_one { # @test
  true
}
function failing_one { # @test
  false
}
function passing_two { # @test
  true
}
EOF
  run "$BATS_WRAPPER" --hide-passing --no-sandbox "${TEST_TMPDIR}/mixed.bats"
  # bats returns non-zero when tests fail
  assert_failure
  assert_output --partial "not ok 2"
  refute_line --regexp "^ok 1 "
  refute_line --regexp "^ok 3 "
}

function bats_wrapper_hide_passing_preserves_skip_and_todo { # @test
  cat >"${TEST_TMPDIR}/directives.bats" <<'EOF'
#! /usr/bin/env bats
function skipped_test { # @test
  skip "not ready"
}
function passing_test { # @test
  true
}
EOF
  run "$BATS_WRAPPER" --hide-passing --no-sandbox "${TEST_TMPDIR}/directives.bats"
  assert_success
  assert_line --regexp "^ok 1.* # [Ss][Kk][Ii][Pp]"
  refute_line --regexp "^ok 2 "
}

function bats_wrapper_hide_passing_preserves_yaml_on_failure { # @test
  cat >"${TEST_TMPDIR}/yaml_fail.bats" <<'EOF'
#! /usr/bin/env bats
function passing_test { # @test
  true
}
function failing_with_output { # @test
  run echo "some diagnostic"
  false
}
EOF
  run "$BATS_WRAPPER" --hide-passing --no-sandbox "${TEST_TMPDIR}/yaml_fail.bats"
  assert_failure
  assert_output --partial "not ok 2"
  refute_line --regexp "^ok 1 "
}

function bats_wrapper_hide_passing_preserves_plan_and_version { # @test
  cat >"${TEST_TMPDIR}/plan.bats" <<'EOF'
#! /usr/bin/env bats
function passing_one { # @test
  true
}
function passing_two { # @test
  true
}
EOF
  run "$BATS_WRAPPER" --hide-passing --no-sandbox "${TEST_TMPDIR}/plan.bats"
  assert_success
  assert_output --partial "1..2"
  assert_line --index 0 --regexp "^(TAP version|1\.\.)"
}
