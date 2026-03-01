
cmd_nix_dev := "nix develop " + justfile_directory() + " --command "

default: && update build test

# Build all packages (default = marketplace bundle)
build:
    nix build

# Build individual packages
build-grit:
    nix build .#grit

build-lux:
    nix build .#lux

build-get-hubbed:
    nix build .#get-hubbed

build-chix:
    nix build .#chix

build-purse-first:
    nix build .#purse-first

build-robin:
    nix build .#robin

build-spinclass:
    nix build .#spinclass

build-go:
    {{cmd_nix_dev}} go build -o build/purse-first ./cmd/purse-first

# Build Homebrew tap formula templates
build-brew:
    nix build .#homebrew-tap

# Build marketplace without hooks
build-no-hooks:
    nix build .#marketplace-no-hooks

# Install MCP servers and packages
install:
    nix run .#install

update: update-nix

update-nix:
    nix flake update

cmd-tap-dancer := join(justfile_directory(), "./packages/tap-dancer/go/cmd/tap-dancer")
tap-dancer-go-test := "go run " + cmd-tap-dancer + " go-test -skip-empty"
tap-dancer-cargo-test := "go run " + cmd-tap-dancer + " cargo-test -skip-empty"

# Test individual Go packages
test-grit:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./packages/grit/...

test-get-hubbed:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./packages/get-hubbed/...

test-lux:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./packages/lux/...

test-spinclass:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./packages/spinclass/...

test-tap-dancer-go:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./packages/tap-dancer/go/...

# Test Rust packages
[working-directory: 'packages/chix']
test-chix:
  {{cmd_nix_dev}} {{tap-dancer-cargo-test}} test

[working-directory: 'packages/tap-dancer/rust']
test-tap-dancer-rust:
    {{cmd_nix_dev}} {{tap-dancer-cargo-test}} test

# Run tests
test-go:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./...

# Run tests with verbose output
test-v:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} -v ./...

# Format code
fmt:
    {{cmd_nix_dev}} go fmt ./...

# Lint code
lint:
    {{cmd_nix_dev}} go vet ./...

# Regenerate workspace vendor directory after dependency changes
vendor:
    {{cmd_nix_dev}} go work vendor

# Update go dependencies, tidy all modules, and re-vendor
deps:
    {{cmd_nix_dev}} go work sync
    {{cmd_nix_dev}} go work vendor

# Recompute goVendorHash in flake.nix from the local vendor directory
vendor-hash:
    #!/usr/bin/env bash
    set -euo pipefail
    # Hash the vendor directory — matches what nix's fixed-output derivation produces
    hash=$(nix hash path vendor/)
    # Update goVendorHash in flake.nix
    sed -i '' -E 's|(goVendorHash = )"sha256-[^"]+";|\1"'"$hash"'";|' flake.nix
    echo "updated goVendorHash to $hash"

# Run integration tests
test-integration:
    nix build
    {{cmd_nix_dev}} bats --tap \
      zz-tests_bats/validate_marketplace.bats \
      zz-tests_bats/validate_documents.bats \
      zz-tests_bats/validate_plugin_repos.bats

# Validate plugin repos have correct .claude-plugin/plugin.json
test-validate-repos:
    {{cmd_nix_dev}} bats --tap zz-tests_bats/validate_plugin_repos.bats

# Run validate-specific BATS tests
test-validate:
    nix build
    {{cmd_nix_dev}} bats --tap zz-tests_bats/validate_documents.bats

# Run lifecycle tests
test-lifecycle:
    nix build
    {{cmd_nix_dev}} bats --tap zz-tests_bats/hook_lifecycle.bats

# Validate own plugin manifest
validate:
    {{cmd_nix_dev}} go run ./cmd/purse-first validate .claude-plugin/plugin.json

# Run Homebrew tap BATS tests
test-brew:
    nix build .#homebrew-tap
    {{cmd_nix_dev}} bats --tap zz-tests_bats/homebrew_tap.bats

test-spinclass-bats:
    nix build .#spinclass
    {{cmd_nix_dev}} just packages/spinclass/zz-tests_bats/test

test: \
    test-chix \
    test-get-hubbed \
    test-go \
    test-go-mcp \
    test-grit \
    test-integration \
    test-lifecycle \
    test-lux \
    test-rust-mcp \
    test-spinclass \
    test-spinclass-bats \
    test-tap-dancer-go \
    test-tap-dancer-rust \
    test-template

# Build go-lib-mcp
build-go-mcp:
    {{cmd_nix_dev}} go build ./...

# Test go-lib-mcp
test-go-mcp:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} -v ./libs/go-mcp/...

# Build rust-lib-mcp
build-rust-mcp:
    nix build ./libs/rust-mcp

# Test rust-lib-mcp
test-rust-mcp:
    cd libs/rust-mcp && {{cmd_nix_dev}} cargo test

# Test command package specifically
test-command:
    cd libs/go-mcp && nix develop ../../ --command {{tap-dancer-go-test}} -v ./command/

# Verify mkMarketplace.nix parses
check-lib:
    nix-instantiate --parse lib/mkMarketplace.nix

# Verify template parses
check-template:
    nix-instantiate --parse templates/marketplace/flake.nix

# Run template tests
test-template:
    {{cmd_nix_dev}} bats --tap zz-tests_bats/marketplace_template.bats

# Build dummy Go MCP servers
build-dummies-go:
    {{cmd_nix_dev}} go build -o build/ ./dummies/go/cmd/...

# Test dummy Go MCP servers
test-dummies-go:
    {{cmd_nix_dev}} go vet ./dummies/go/...

# Clean build artifacts
clean:
    rm -f purse-first
    rm -rf build/
    rm -rf result
