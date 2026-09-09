#!/usr/bin/env bats
#
# RFC 0003 (List-Table NDJSON Protocol) conformance tests for the `mesa`
# renderer CLI.
#
# Deliberately does not load common.bash: that helper hard-requires
# PURSE_FIRST_BIN at source time, and this lane only needs the mesa
# binary. Binary injection (RFC 0003 Conformance Testing): resolve the
# binary from MESA_BIN, falling back to build/mesa (`just
# build-mesa-cli`), so a future re-implementation runs the same suite.

setup() {
  bats_load_library "bats-support"
  bats_load_library "bats-assert"

  # `output` is set by bats' `run`; the export satisfies shellcheck
  # (SC2154), matching the other bats files in this directory.
  export output

  MESA_BIN="${MESA_BIN:-$BATS_TEST_DIRNAME/../build/mesa}"
  if [[ ! -x $MESA_BIN ]]; then
    echo "error: $MESA_BIN is not built (run \`just build-mesa-cli\` or set MESA_BIN)" >&2
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
  run "$MESA_BIN" --plain <"$infile"
  assert_failure
}

@test "§8: an empty columns list is a protocol error" {
  mk '{"columns":[]}'
  run "$MESA_BIN" --plain <"$infile"
  assert_failure
}

@test "§3: a role that is not pin/flex is a protocol error" {
  mk '{"columns":[{"name":"A","role":"bogus"}]}'
  run "$MESA_BIN" --plain <"$infile"
  assert_failure
}

@test "§4: a row whose cell count differs from the column count is a protocol error" {
  mk '{"columns":[{"name":"A","role":"pin"}]}' '{"cells":["a","b"]}'
  run "$MESA_BIN" --plain <"$infile"
  assert_failure
}

@test "§7.1/§7.3: piped output is plain — TAB-separated, no border, no ANSI" {
  mk '{"columns":[{"name":"ID","role":"pin"},{"name":"AGE","role":"pin"}]}' \
    '{"cells":["api","2m"]}'
  # No flag: stdout is a pipe under `run`, so mode selection picks plain.
  run "$MESA_BIN" <"$infile"
  assert_success
  assert_line --index 0 "ID${TAB}AGE"
  assert_line --index 1 "api${TAB}2m"
  refute_output --partial "╭"
  refute_output --partial "$ESC["
}

@test "§7.1/§7.2: --force-style emits ANSI and a border to a pipe" {
  mk '{"columns":[{"name":"ID","role":"pin"}]}' '{"cells":["api"]}'
  run "$MESA_BIN" --force-style <"$infile"
  assert_success
  assert_output --partial "$ESC["
  assert_output --partial "╭"
}

@test "§7.4: an empty table prints its empty text and exits 0" {
  mk '{"columns":[{"name":"ID","role":"pin"}],"empty":"no sessions"}'
  run "$MESA_BIN" --plain <"$infile"
  assert_success
  assert_output "no sessions"
}

@test "§6.1/§7.3: footer lines follow the rows verbatim on a pipe" {
  # Two columns, so the rows carry a TAB the footer lines must not.
  mk '{"columns":[{"name":"ID","role":"pin"},{"name":"AGE","role":"pin"}],"footer":["self= this daemon'"'"'s build",{"spans":[{"text":"●","sev":"warn"},{"text":" stale"}]}]}' \
    '{"cells":["api","2m"]}'
  run "$MESA_BIN" --plain <"$infile"
  assert_success
  assert_line --index 0 "ID${TAB}AGE"
  assert_line --index 1 "api${TAB}2m"
  # assert_line matches the whole line, so these also pin the absence of a
  # trailing TAB and of any styling on the footer.
  assert_line --index 2 "self= this daemon's build"
  assert_line --index 3 "● stale"
  refute_output --partial "$ESC["
}

@test "§6.1: a footer renders below the legend on a TTY" {
  mk '{"columns":[{"name":"ID","role":"pin"}],"legend":[{"sev":"ok","glyph":"●","label":"attached"}],"footer":["self= this daemon'"'"'s build"]}' \
    '{"cells":["api"]}'
  run "$MESA_BIN" --force-style <"$infile"
  assert_success
  # Grid, then legend, then footer.
  local grid legend footer
  grid=$(printf '%s\n' "$output" | grep -n '╰' | head -1 | cut -d: -f1)
  legend=$(printf '%s\n' "$output" | grep -n 'attached' | head -1 | cut -d: -f1)
  footer=$(printf '%s\n' "$output" | grep -n 'self=' | head -1 | cut -d: -f1)
  [[ -n $grid && -n $legend && -n $footer ]]
  ((grid < legend))
  ((legend < footer))
}

# footer_line renders a one-span footer at the given severity and echoes just
# the footer line, so a test can compare two severities without pinning the
# renderer's exact SGR codes. $1 is the "sev" JSON fragment ("" for none).
footer_line() {
  local sev="$1"
  printf '%s\n' \
    "{\"columns\":[{\"name\":\"ID\",\"role\":\"pin\"}],\"footer\":[{\"spans\":[{\"text\":\"marker\"$sev}]}]}" \
    '{"cells":["api"]}' >"$infile"
  "$MESA_BIN" --force-style <"$infile" | grep 'marker'
}

@test "§6.1: a footer span declaring no severity is dimmed" {
  # The footer's only span is neutral, so the sole thing that can put an
  # escape on this line is the neutral -> muted rule.
  local neutral muted
  neutral=$(footer_line "")
  muted=$(footer_line ',"sev":"muted"')
  [[ $neutral == *"$ESC["* ]]
  [[ $neutral == "$muted" ]]
}

@test "§6.1: a footer span keeps its own severity color" {
  local neutral warn
  neutral=$(footer_line "")
  warn=$(footer_line ',"sev":"warn"')
  [[ $warn == *"marker"* ]]
  [[ $warn != "$neutral" ]]
}

@test "§7.4: an empty table renders no footer" {
  mk '{"columns":[{"name":"ID","role":"pin"}],"empty":"no sessions","footer":["a key"]}'
  run "$MESA_BIN" --plain <"$infile"
  assert_success
  assert_output "no sessions"
}

@test "§6.1: a footer is not wrapped or truncated to the table width" {
  # The footer far exceeds --width 30; the lone pin column cannot shrink,
  # so any ellipsis or line break in the output would be the footer's.
  local long='self= this daemon build · remote= the endpoint build · no self= means stale'
  mk "{\"columns\":[{\"name\":\"ID\",\"role\":\"pin\"}],\"footer\":[\"$long\"]}" \
    '{"cells":["x"]}'
  run "$MESA_BIN" --force-style --width 30 <"$infile"
  assert_success
  refute_output --partial "…"
  # One neutral span renders as one contiguous run, so the whole text must
  # sit on a single line.
  printf '%s\n' "$output" | grep -qF "$long"
}

@test "§2: an unrecognized header field is ignored, not an error" {
  # Additive header growth depends on this: a renderer that rejected an
  # unknown field would make every future field a breaking change.
  mk '{"columns":[{"name":"ID","role":"pin"}],"unheardOf":123,"alsoNew":{"deep":["x"]}}' \
    '{"cells":["api"]}'
  run "$MESA_BIN" --plain <"$infile"
  assert_success
  assert_output "ID
api"
}

@test "§2: a stream with no footer field renders exactly as before" {
  mk '{"columns":[{"name":"ID","role":"pin"}]}' '{"cells":["api"]}'
  run "$MESA_BIN" --plain <"$infile"
  assert_success
  assert_output "ID
api"
}

@test "§5: an unknown severity degrades to neutral without aborting" {
  mk '{"columns":[{"name":"A","role":"pin"}]}' \
    '{"cells":[{"spans":[{"text":"hi","sev":"wat"}]}]}'
  run "$MESA_BIN" --plain <"$infile"
  assert_success
  assert_output --partial "hi"
}

@test "Security: an embedded escape in cell text is stripped from plain output" {
  # \u001b[31m is an ESC + SGR sequence encoded in the JSON string.
  mk '{"columns":[{"name":"A","role":"pin"}]}' '{"cells":["a\u001b[31mb"]}'
  run "$MESA_BIN" --plain <"$infile"
  assert_success
  refute_output --partial "$ESC"
  assert_output --partial "a[31mb"
}

@test "§7.2: a flex column with wrap:true wraps overflow instead of ellipsizing" {
  mk '{"columns":[{"name":"A","role":"pin"},{"name":"B","role":"flex","wrap":true}]}' \
    '{"cells":["x","alpha bravo charlie delta echo foxtrot golf hotel"]}'
  run "$MESA_BIN" --force-style --width 30 <"$infile"
  assert_success
  # the tail word survives (not truncated) and no ellipsis is emitted
  assert_output --partial "hotel"
  refute_output --partial "…"
}
