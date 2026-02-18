#!/usr/bin/env bats

setup() {
  load "common.bash"
  OUTDIR="${BATS_TEST_TMPDIR}/tarballs"
  mkdir -p "$OUTDIR"
}

@test "tarball for binary package contains bin and share" {
  nix build .#grit -o "${BATS_TEST_TMPDIR}/result-grit"
  bin/brew-build-tarball.bash \
    "${BATS_TEST_TMPDIR}/result-grit" grit 0.1.0 darwin-arm64 "$OUTDIR"

  [[ -f "$OUTDIR/grit-0.1.0-darwin-arm64.tar.gz" ]]

  tar -tzf "$OUTDIR/grit-0.1.0-darwin-arm64.tar.gz" | grep -q 'bin/grit'
  tar -tzf "$OUTDIR/grit-0.1.0-darwin-arm64.tar.gz" | grep -q 'share/purse-first/grit/plugin.json'
}

@test "tarball for skill-only package contains only share" {
  nix build .#robin -o "${BATS_TEST_TMPDIR}/result-robin"
  bin/brew-build-tarball.bash \
    "${BATS_TEST_TMPDIR}/result-robin" robin 0.1.0 darwin-arm64 "$OUTDIR"

  [[ -f "$OUTDIR/robin-0.1.0-darwin-arm64.tar.gz" ]]

  ! tar -tzf "$OUTDIR/robin-0.1.0-darwin-arm64.tar.gz" | grep -q '^./bin/'
  tar -tzf "$OUTDIR/robin-0.1.0-darwin-arm64.tar.gz" | grep -q 'share/purse-first/robin/plugin.json'
}
