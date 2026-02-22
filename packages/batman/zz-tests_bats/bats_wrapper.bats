#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  BATS_TMPDIR="${BATS_TMPDIR:-/tmp}"
  TEST_TMPDIR="$(mktemp -d "${BATS_TMPDIR}/bats-wrapper-XXXXXX")"

  # Resolve our wrapper binary (the one under test).
  # BATS_WRAPPER can be set externally; otherwise derive from the result symlink.
  if [[ -z "${BATS_WRAPPER:-}" ]]; then
    BATS_WRAPPER="$(cd "$BATS_TEST_DIRNAME/.." && pwd)/result/bin/bats"
  fi
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
