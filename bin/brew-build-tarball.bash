#!/usr/bin/env bash
set -euo pipefail

# Usage: brew-build-tarball.bash <nix-result> <name> <version> <platform> <output-dir>
#
# Packages a Nix build output into a Homebrew-compatible tarball.

nix_result="$1"
name="$2"
version="$3"
platform="$4"
output_dir="$5"

staging="$(mktemp -d)"
trap 'chmod -R u+w "$staging" && rm -rf "$staging"' EXIT

# Copy binary if present (follow symlinks).
if [[ -f "$nix_result/bin/$name" ]]; then
  mkdir -p "$staging/bin"
  cp -L "$nix_result/bin/$name" "$staging/bin/$name"
  chmod +x "$staging/bin/$name"
fi

# Copy share directory if present (follow symlinks).
if [[ -d "$nix_result/share/purse-first/$name" ]]; then
  mkdir -p "$staging/share/purse-first/$name"
  cp -rL "$nix_result/share/purse-first/$name/." "$staging/share/purse-first/$name/"
  chmod -R u+w "$staging/share"
fi

mkdir -p "$output_dir"
tar -czf "$output_dir/${name}-${version}-${platform}.tar.gz" \
  -C "$staging" .
