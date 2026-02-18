#!/usr/bin/env bats

setup() {
  load "common.bash"
  TAP_RESULT="${BATS_TEST_TMPDIR}/tap-result"
}

@test "homebrew-tap output builds successfully" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  [[ -d "$TAP_RESULT/Formula" ]]
}

@test "formula files exist for eligible packages" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  for pkg in purse-first grit lux get-hubbed robin tap-dancer; do
    [[ -f "$TAP_RESULT/Formula/${pkg}.rb" ]]
  done
}

@test "excluded packages have no formula" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  [[ ! -f "$TAP_RESULT/Formula/chix.rb" ]]
}

@test "meta formula exists" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  [[ -f "$TAP_RESULT/Formula/purse-first-all.rb" ]]
}

@test "binary formulas have bin.install" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  for pkg in purse-first grit lux get-hubbed; do
    grep -q 'bin.install' "$TAP_RESULT/Formula/${pkg}.rb"
  done
}

@test "skill-only formulas lack bin.install" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  for pkg in robin tap-dancer; do
    ! grep -q 'bin.install' "$TAP_RESULT/Formula/${pkg}.rb"
  done
}

@test "get-hubbed depends on gh" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  grep -q 'depends_on "gh"' "$TAP_RESULT/Formula/get-hubbed.rb"
}

@test "formulas have sha256 placeholders" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  grep -q 'SHA256_PLACEHOLDER_' "$TAP_RESULT/Formula/grit.rb"
}

@test "formula class names are PascalCase" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  grep -q 'class Grit < Formula' "$TAP_RESULT/Formula/grit.rb"
  grep -q 'class GetHubbed < Formula' "$TAP_RESULT/Formula/get-hubbed.rb"
  grep -q 'class TapDancer < Formula' "$TAP_RESULT/Formula/tap-dancer.rb"
  grep -q 'class PurseFirst < Formula' "$TAP_RESULT/Formula/purse-first.rb"
  grep -q 'class PurseFirstAll < Formula' "$TAP_RESULT/Formula/purse-first-all.rb"
}

@test "meta formula depends on all individual formulas" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  for pkg in purse-first grit lux get-hubbed robin tap-dancer; do
    grep -q "\"${pkg}\"" "$TAP_RESULT/Formula/purse-first-all.rb"
  done
}

@test "tap README exists" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  [[ -f "$TAP_RESULT/README.md" ]]
}

@test "formulas are valid Ruby syntax" {
  nix build .#homebrew-tap -o "$TAP_RESULT"
  for f in "$TAP_RESULT"/Formula/*.rb; do
    ruby -c "$f"
  done
}
