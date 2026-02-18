#!/usr/bin/env bats

setup() {
  load "common.bash"
  FORMULA_DIR="${BATS_TEST_TMPDIR}/Formula"
  TARBALL_DIR="${BATS_TEST_TMPDIR}/tarballs"
  mkdir -p "$FORMULA_DIR" "$TARBALL_DIR"

  # Create a mock formula with placeholders.
  cat > "$FORMULA_DIR/grit.rb" <<'EOF'
class Grit < Formula
  on_macos do
    if Hardware::CPU.arm?
      sha256 "SHA256_PLACEHOLDER_DARWIN_ARM64"
    else
      sha256 "SHA256_PLACEHOLDER_DARWIN_AMD64"
    end
  end
  on_linux do
    if Hardware::CPU.arm?
      sha256 "SHA256_PLACEHOLDER_LINUX_ARM64"
    else
      sha256 "SHA256_PLACEHOLDER_LINUX_AMD64"
    end
  end
end
EOF

  # Create mock tarballs.
  echo "arm64-content" | gzip > "$TARBALL_DIR/grit-0.1.0-darwin-arm64.tar.gz"
  echo "amd64-content" | gzip > "$TARBALL_DIR/grit-0.1.0-darwin-amd64.tar.gz"
  echo "linux-arm64" | gzip > "$TARBALL_DIR/grit-0.1.0-linux-arm64.tar.gz"
  echo "linux-amd64" | gzip > "$TARBALL_DIR/grit-0.1.0-linux-amd64.tar.gz"
}

@test "placeholders are replaced with real hashes" {
  bin/brew-update-hashes.bash "$FORMULA_DIR" "$TARBALL_DIR" 0.1.0

  ! grep -q 'SHA256_PLACEHOLDER_' "$FORMULA_DIR/grit.rb"
  grep -qE 'sha256 "[a-f0-9]{64}"' "$FORMULA_DIR/grit.rb"
}

@test "missing tarballs cause a non-zero exit" {
  rm "$TARBALL_DIR/grit-0.1.0-darwin-arm64.tar.gz"
  run bin/brew-update-hashes.bash "$FORMULA_DIR" "$TARBALL_DIR" 0.1.0
  [[ "$status" -ne 0 ]]
}
