#!/usr/bin/env bats
#
# RFC 0003 (List-Table NDJSON Protocol) conformance tests for the
# `dewey table` renderer.
#
# Deliberately does not load common.bash: that helper hard-requires
# PURSE_FIRST_BIN at source time, and this lane only needs the dewey
# binary. Binary injection (RFC 0003 Conformance Testing): resolve the
# binary from DEWEY_BIN, falling back to build/dewey (`just
# build-dewey-cli`), so a future re-implementation runs the same suite.

setup() {
  bats_load_library "bats-support"
  bats_load_library "bats-assert"

  # `output` is set by bats' `run`; the export satisfies shellcheck
  # (SC2154), matching the other bats files in this directory.
  export output

  DEWEY_BIN="${DEWEY_BIN:-$BATS_TEST_DIRNAME/../build/dewey}"
  if [[ ! -x $DEWEY_BIN ]]; then
    echo "error: $DEWEY_BIN is not built (run \`just build-dewey-cli\` or set DEWEY_BIN)" >&2
    return 1
  fi

  infile="$BATS_TEST_TMPDIR/in.ndjson"
}

# mk writes each argument as one NDJSON record line to $infile.
mk() {
  printf '%s\n' "$@" >"$infile"
}

ESC=$'\e'
TAB=$'\t'

@test "§8: a first record lacking columns is a protocol error" {
  mk '{"cells":["x"]}'
  run "$DEWEY_BIN" table --plain <"$infile"
  assert_failure
}

@test "§8: an empty columns list is a protocol error" {
  mk '{"columns":[]}'
  run "$DEWEY_BIN" table --plain <"$infile"
  assert_failure
}

@test "§3: a role that is not pin/flex is a protocol error" {
  mk '{"columns":[{"name":"A","role":"bogus"}]}'
  run "$DEWEY_BIN" table --plain <"$infile"
  assert_failure
}

@test "§4: a row whose cell count differs from the column count is a protocol error" {
  mk '{"columns":[{"name":"A","role":"pin"}]}' '{"cells":["a","b"]}'
  run "$DEWEY_BIN" table --plain <"$infile"
  assert_failure
}

@test "§7.1/§7.3: piped output is plain — TAB-separated, no border, no ANSI" {
  mk '{"columns":[{"name":"ID","role":"pin"},{"name":"AGE","role":"pin"}]}' \
    '{"cells":["api","2m"]}'
  # No flag: stdout is a pipe under `run`, so mode selection picks plain.
  run "$DEWEY_BIN" table <"$infile"
  assert_success
  assert_line --index 0 "ID${TAB}AGE"
  assert_line --index 1 "api${TAB}2m"
  refute_output --partial "╭"
  refute_output --partial "$ESC["
}

@test "§7.1/§7.2: --force-style emits ANSI and a border to a pipe" {
  mk '{"columns":[{"name":"ID","role":"pin"}]}' '{"cells":["api"]}'
  run "$DEWEY_BIN" table --force-style <"$infile"
  assert_success
  assert_output --partial "$ESC["
  assert_output --partial "╭"
}

@test "§7.4: an empty table prints its empty text and exits 0" {
  mk '{"columns":[{"name":"ID","role":"pin"}],"empty":"no sessions"}'
  run "$DEWEY_BIN" table --plain <"$infile"
  assert_success
  assert_output "no sessions"
}

@test "§5: an unknown severity degrades to neutral without aborting" {
  mk '{"columns":[{"name":"A","role":"pin"}]}' \
    '{"cells":[{"spans":[{"text":"hi","sev":"wat"}]}]}'
  run "$DEWEY_BIN" table --plain <"$infile"
  assert_success
  assert_output --partial "hi"
}

@test "Security: an embedded escape in cell text is stripped from plain output" {
  # \u001b[31m is an ESC + SGR sequence encoded in the JSON string.
  mk '{"columns":[{"name":"A","role":"pin"}]}' '{"cells":["a\u001b[31mb"]}'
  run "$DEWEY_BIN" table --plain <"$infile"
  assert_success
  refute_output --partial "$ESC"
  assert_output --partial "a[31mb"
}
