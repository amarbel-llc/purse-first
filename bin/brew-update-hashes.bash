#!/usr/bin/env bash
set -euo pipefail

# Usage: brew-update-hashes.bash <formula-dir> <tarball-dir> <version>
#
# For each .rb file, substitutes SHA256_PLACEHOLDER_* with real hashes
# computed from matching tarballs.

formula_dir="$1"
tarball_dir="$2"
version="$3"

declare -A platform_map=(
  [DARWIN_ARM64]="darwin-arm64"
  [DARWIN_AMD64]="darwin-amd64"
  [LINUX_ARM64]="linux-arm64"
  [LINUX_AMD64]="linux-amd64"
)

for formula in "$formula_dir"/*.rb; do
  name="$(basename "$formula" .rb)"

  for placeholder in "${!platform_map[@]}"; do
    platform="${platform_map[$placeholder]}"
    tarball="$tarball_dir/${name}-${version}-${platform}.tar.gz"

    if grep -q "SHA256_PLACEHOLDER_${placeholder}" "$formula"; then
      if [[ ! -f "$tarball" ]]; then
        echo "error: missing tarball: $tarball" >&2
        exit 1
      fi

      hash="$(shasum -a 256 "$tarball" | cut -d' ' -f1)"
      sed -i.bak "s/SHA256_PLACEHOLDER_${placeholder}/${hash}/" "$formula"
      rm -f "${formula}.bak"
    fi
  done
done
