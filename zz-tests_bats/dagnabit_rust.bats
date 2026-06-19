#!/usr/bin/env bats
#
# End-to-end CLI tests for dagnabit's rust mode: reposition, export,
# and move/rename against fixture cargo workspaces (the Task 12 lane of
# docs/plans/2026-06-06-dagnabit-rust-plan.md).
#
# Deliberately does not load common.bash: that helper hard-requires
# PURSE_FIRST_BIN at source time and these tests only touch the
# dagnabit binary built into build/ by the `dagnabit-build` justfile
# recipe (see `just test-dagnabit-rust`). Tests that shell cargo or
# ast-grep skip when the tool is not on PATH, so the lane stays green
# outside the rust devshell.

setup() {
  bats_load_library "bats-support"
  bats_load_library "bats-assert"

  # `output` is set by bats' `run`; the export satisfies shellcheck
  # (SC2154), matching the other bats files in this directory.
  export output

  dagnabit="$BATS_TEST_DIRNAME/../build/dagnabit"
  if [[ ! -x $dagnabit ]]; then
    echo "error: $dagnabit is not built (run \`just dagnabit-build\`)" >&2
    return 1
  fi

  # Keep cargo's cache/registry writes inside the test sandbox.
  export CARGO_HOME="$BATS_TEST_TMPDIR/cargo-home"
}

require_cargo() {
  command -v cargo >/dev/null || skip "cargo not on PATH"
}

require_ast_grep() {
  command -v ast-grep >/dev/null || skip "ast-grep not on PATH"
}

# Mirrors writeFixtureWorkspaceTree in
# libs/dewey/internal/echo/dagnabit_rust/fixtures_test.go: a virtual
# workspace with blob_store_id_internal at tier 0 and store_internal at
# alfa path-depending on it. store's lib.rs deliberately uses BOTH a
# use-decl and a qualified path so the renamer's ast-grep pattern set
# is exercised on each. The members array is deliberately multiline
# (cargo_manifest.AddMember rejects single-line arrays by design).
write_rust_fixture() {
  fixture="$BATS_TEST_TMPDIR/ws"
  mkdir -p "$fixture/internal/0/blob_store_id/src" \
    "$fixture/internal/alfa/store/src"

  cat >"$fixture/Cargo.toml" <<'EOF'
[workspace]
members = [
  "internal/0/blob_store_id",
  "internal/alfa/store",
]
resolver = "2"
EOF

  cat >"$fixture/internal/0/blob_store_id/Cargo.toml" <<'EOF'
[package]
name = "blob_store_id_internal"
version = "0.1.0"
edition = "2021"
EOF

  echo 'pub fn make_id() -> u32 { 7 }' \
    >"$fixture/internal/0/blob_store_id/src/lib.rs"

  cat >"$fixture/internal/alfa/store/Cargo.toml" <<'EOF'
[package]
name = "store_internal"
version = "0.1.0"
edition = "2021"

[dependencies]
blob_store_id_internal = { path = "../../0/blob_store_id" }
EOF

  cat >"$fixture/internal/alfa/store/src/lib.rs" <<'EOF'
use blob_store_id_internal::make_id as mk;

pub fn make() -> u32 { blob_store_id_internal::make_id() }

pub fn make_via_use() -> u32 { mk() }
EOF
}

# Variant with store misplaced at tier 0: it path-depends on a tier-0
# crate, so its required level is alfa — reposition must move it.
write_misplaced_rust_fixture() {
  fixture="$BATS_TEST_TMPDIR/ws-misplaced"
  mkdir -p "$fixture/internal/0/blob_store_id/src" \
    "$fixture/internal/0/store/src"

  cat >"$fixture/Cargo.toml" <<'EOF'
[workspace]
members = [
  "internal/0/blob_store_id",
  "internal/0/store",
]
resolver = "2"
EOF

  cat >"$fixture/internal/0/blob_store_id/Cargo.toml" <<'EOF'
[package]
name = "blob_store_id_internal"
version = "0.1.0"
edition = "2021"
EOF

  echo 'pub fn make_id() -> u32 { 7 }' \
    >"$fixture/internal/0/blob_store_id/src/lib.rs"

  cat >"$fixture/internal/0/store/Cargo.toml" <<'EOF'
[package]
name = "store_internal"
version = "0.1.0"
edition = "2021"

[dependencies]
blob_store_id_internal = { path = "../blob_store_id" }
EOF

  cat >"$fixture/internal/0/store/src/lib.rs" <<'EOF'
pub fn make() -> u32 { blob_store_id_internal::make_id() }
EOF
}

# A correctly-tiered two-package go module: the go-mode canary fixture.
write_go_fixture() {
  fixture="$BATS_TEST_TMPDIR/gomod"
  mkdir -p "$fixture/internal/0/util" "$fixture/internal/alfa/app"

  cat >"$fixture/go.mod" <<'EOF'
module example.test/fixture

go 1.26
EOF

  cat >"$fixture/internal/0/util/util.go" <<'EOF'
package util

func Value() int { return 7 }
EOF

  cat >"$fixture/internal/alfa/app/app.go" <<'EOF'
package app

import "example.test/fixture/internal/0/util"

func App() int { return util.Value() }
EOF
}

# git init + commit the fixture: moves go through `git mv`. Signing is
# disabled in the fixture repo so the lane never depends on the user's
# gpg agent.
git_init_fixture() {
  git -C "$fixture" init -q
  git -C "$fixture" config user.email "test@test.com"
  git -C "$fixture" config user.name "Test"
  git -C "$fixture" config commit.gpgSign false
  git -C "$fixture" add -A
  git -C "$fixture" commit -q -m "fixture"
}

function rust_reposition_dry_run_reports_planned_moves { # @test
  require_cargo
  write_misplaced_rust_fixture
  cd "$fixture" || return 1

  run "$dagnabit" -n internal
  assert_success
  assert_output '{"dst":"internal/alfa/store","event":"would-move","src":"internal/0/store"}'
}

function rust_reposition_moves_crate_and_workspace_still_resolves { # @test
  require_cargo
  write_misplaced_rust_fixture
  git_init_fixture
  cd "$fixture" || return 1

  run "$dagnabit" internal
  assert_success
  assert_output '{"dst":"internal/alfa/store","event":"move","src":"internal/0/store"}'

  [[ -f internal/alfa/store/Cargo.toml ]]
  [[ ! -e internal/0/store ]]

  run cargo metadata --format-version 1 --no-deps
  assert_success
}

function rust_export_library_generates_facades_cargo_check_accepts { # @test
  require_cargo
  write_rust_fixture
  cd "$fixture" || return 1

  run "$dagnabit" export --library
  assert_success

  run cat pkgs/blob_store_id/src/lib.rs
  assert_output - <<'EOF'
// Code generated by dagnabit (dev+unknown); DO NOT EDIT.

pub use blob_store_id_internal::*;
EOF

  run cargo check --workspace
  assert_success
}

function rust_export_check_detects_drift { # @test
  # Pure-file: generation and the --check comparison never shell cargo,
  # so this runs even outside the rust devshell.
  write_rust_fixture
  cd "$fixture" || return 1

  run "$dagnabit" export --library
  assert_success

  echo '// drift' >>pkgs/blob_store_id/src/lib.rs

  run "$dagnabit" export --check --library
  assert_failure
  # --check prints `generated:` temp-dir lines before the drift report
  # (pre-existing behavior); assert on the drift error line only.
  assert_output --partial 'pkgs/blob_store_id/src/lib.rs (out of date)'
}

function rust_rename_rewrites_dependents_and_cargo_check_passes { # @test
  require_cargo
  require_ast_grep
  write_rust_fixture
  git_init_fixture
  cd "$fixture" || return 1

  run "$dagnabit" move internal/0/blob_store_id internal/0/blob_id
  assert_success

  run cat internal/0/blob_id/Cargo.toml
  assert_output - <<'EOF'
[package]
name = "blob_id_internal"
version = "0.1.0"
edition = "2021"
EOF

  run cat internal/alfa/store/Cargo.toml
  assert_output - <<'EOF'
[package]
name = "store_internal"
version = "0.1.0"
edition = "2021"

[dependencies]
blob_id_internal = { path = "../../0/blob_id" }
EOF

  run cat internal/alfa/store/src/lib.rs
  assert_output - <<'EOF'
use blob_id_internal::make_id as mk;

pub fn make() -> u32 { blob_id_internal::make_id() }

pub fn make_via_use() -> u32 { mk() }
EOF

  run cargo check --workspace
  assert_success
}

function go_mode_is_unaffected { # @test
  # Cheapest meaningful go-mode invocation: a dry-run reposition over a
  # correctly-tiered module exercises language detection (go.mod wins),
  # the go_list reader, and the level math end-to-end, and must produce
  # zero events — proving the rust lane left the go path intact.
  command -v go >/dev/null || skip "go not on PATH"
  write_go_fixture
  cd "$fixture" || return 1

  export GOWORK=off

  run "$dagnabit" -n internal
  assert_success
  assert_output ""
}

function go_mode_works_from_module_subdirectory { # @test
  # purse-first#142: language detection walks up and finds the module
  # root, so go mode must operate on it from any subdirectory — same as
  # rust mode — with prefixes interpreted relative to the root.
  command -v go >/dev/null || skip "go not on PATH"
  write_go_fixture
  cd "$fixture/internal" || return 1

  export GOWORK=off

  run "$dagnabit" -n internal
  assert_success
  assert_output ""
}
