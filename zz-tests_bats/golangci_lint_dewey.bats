#!/usr/bin/env bats
#
# Acceptance tests for the nix-built custom golangci-lint binary with
# dewey's gclplugin module plugin linked in (purse-first#134).
#
# Deliberately does not load common.bash: that helper hard-requires
# PURSE_FIRST_BIN at source time and these tests never touch the
# purse-first CLI. The binary under test arrives via
# GOLANGCI_LINT_DEWEY_BIN (see `just test-golangci-dewey`).

setup() {
  bats_load_library "bats-support"
  bats_load_library "bats-assert"

  # `output` is set by bats' `run`; the export satisfies shellcheck
  # (SC2154), matching the other bats files in this directory.
  export output

  if [[ -z ${GOLANGCI_LINT_DEWEY_BIN:-} || ! -x ${GOLANGCI_LINT_DEWEY_BIN:-} ]]; then
    echo "error: GOLANGCI_LINT_DEWEY_BIN is not set or not executable" >&2
    return 1
  fi
  gcl="$GOLANGCI_LINT_DEWEY_BIN"

  # The fixture module lives under the repo's go.work tree but is not a
  # workspace member; force module mode so go/packages loading works.
  export GOWORK=off
  export GOLANGCI_LINT_CACHE="$BATS_TEST_TMPDIR/golangci-cache"

  write_fixture
}

# A minimal module tripping defererr: the deferred Close discards its
# error return value.
write_fixture() {
  fixture="$BATS_TEST_TMPDIR/fixture"
  mkdir -p "$fixture"

  cat >"$fixture/go.mod" <<'EOF'
module example.test/fixture

go 1.26
EOF

  cat >"$fixture/main.go" <<'EOF'
package fixture

import "os"

func leak() {
	f, _ := os.Open("x")
	defer f.Close()
}
EOF

  cat >"$fixture/.golangci.yml" <<'EOF'
version: "2"
linters:
  default: none
  enable:
    - dewey
  settings:
    custom:
      dewey:
        type: module
        description: dewey static analyzers
        settings:
          # Pin the fixture to defererr only: the other default-on
          # analyzers (seqerror, repool) are explicitly disabled so a
          # future fixture edit can't silently change which analyzer
          # the run assertion exercises.
          defererr: true
          seqerror: false
          repool: false
EOF
}

function version_reports_dewey_custom_build { # @test
  run "$gcl" version
  assert_success
  assert_output --partial "golangci-lint has version"
  if [[ $output == *"has version unknown"* ]]; then
    # Dev-loop binary (just explore-test-golangci-dewey-dev): the dewey
    # version suffix is injected by nix ldflags only.
    skip "dev build: nix ldflags not injected"
  fi
  assert_output --partial "dewey"
}

function linters_lists_dewey_module_plugin { # @test
  cd "$fixture" || return 1
  run "$gcl" linters
  assert_success
  assert_output --partial "dewey"
}

function run_flags_defererr_violation { # @test
  cd "$fixture" || return 1
  run "$gcl" run
  assert_failure
  assert_output --partial "discards its error return value"
  assert_output --partial "(dewey)"
}
