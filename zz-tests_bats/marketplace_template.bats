#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  TEMPLATE_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/templates/marketplace"
  TEST_DIR="$(mktemp -d)"
}

teardown() {
  rm -rf "$TEST_DIR"
}

@test "template flake.nix parses" {
  run nix-instantiate --parse "$TEMPLATE_DIR/flake.nix"
  assert_success
}

@test "template contains required files" {
  [ -f "$TEMPLATE_DIR/flake.nix" ]
  [ -f "$TEMPLATE_DIR/justfile" ]
  [ -f "$TEMPLATE_DIR/.envrc" ]
  [ -f "$TEMPLATE_DIR/.gitignore" ]
  [ -f "$TEMPLATE_DIR/.github/workflows/ci.yml" ]
}

@test "mkMarketplace.nix parses" {
  LIB_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/lib"
  run nix-instantiate --parse "$LIB_DIR/mkMarketplace.nix"
  assert_success
}
