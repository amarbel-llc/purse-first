
cmd_nix_dev := "nix develop " + justfile_directory() + " --command "

default: build test

# Build all packages (default = marketplace bundle)
build:
    nix build

build-purse-first:
    nix build .#purse-first

build-purse-first-cli:
    nix build .#purse-first -o result-cli

# Build marketplace without hooks
build-no-hooks:
    nix build .#marketplace-no-hooks

build-go:
    {{cmd_nix_dev}} go build -o build/purse-first ./cmd/purse-first

# Install this marketplace's packages into Claude Code
install: build-purse-first-cli
    nix build
    {{justfile_directory()}}/result-cli/bin/purse-first install {{justfile_directory()}}/result

update: update-nix

update-nix:
    nix flake update

# Run Go tests
test-go:
    {{cmd_nix_dev}} go test ./...

# Run Go tests with verbose output
test-v:
    {{cmd_nix_dev}} go test -v ./...

# Test go-mcp library
test-go-mcp:
    {{cmd_nix_dev}} go test -v ./libs/go-mcp/...

# Test rust-mcp library
test-rust-mcp:
    cd libs/rust-mcp && {{cmd_nix_dev}} cargo test

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

# Run BATS integration tests
test-integration: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap --jobs {{num_cpus()}} \
      zz-tests_bats/validate_marketplace.bats \
      zz-tests_bats/validate_documents.bats \
      zz-tests_bats/validate_mcp.bats

# Run validate-specific BATS tests
test-validate: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap --jobs {{num_cpus()}} zz-tests_bats/validate_documents.bats

# Run lifecycle tests
test-lifecycle: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap --jobs {{num_cpus()}} zz-tests_bats/hook_lifecycle.bats

# Run MCP validation tests
test-validate-mcp: build-purse-first-cli
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap zz-tests_bats/validate_mcp.bats

# Run package brew tests
test-package-brew: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap --jobs {{num_cpus()}} zz-tests_bats/package_brew.bats

# Validate own plugin manifest
validate:
    {{cmd_nix_dev}} go run ./cmd/purse-first validate .claude-plugin/plugin.json

# Run template tests
test-template:
    {{cmd_nix_dev}} bats --tap --jobs {{num_cpus()}} zz-tests_bats/marketplace_template.bats

# Verify mkMarketplace.nix parses
check-lib:
    nix-instantiate --parse lib/mkMarketplace.nix

# Verify template parses
check-template:
    nix-instantiate --parse templates/marketplace/flake.nix

test: \
    test-go \
    test-go-mcp \
    test-rust-mcp \
    test-integration \
    test-lifecycle \
    test-package-brew \
    test-template

# Bump version for a package. Usage: just bump-version grit 0.2.0
bump-version package version:
  #!/usr/bin/env bash
  set -euo pipefail
  # Update marketplace-config.json (source of truth)
  jq --arg pkg "{{package}}" --arg ver "{{version}}" \
    '.plugins[$pkg].version = $ver' marketplace-config.json > marketplace-config.json.tmp
  mv marketplace-config.json.tmp marketplace-config.json
  gum log --level info "{{package}}: version bumped to {{version}}"
  gum log --level warn "Remember to update Cargo.toml and SKILL.md frontmatter if applicable"

# Clean build artifacts
clean:
    rm -f purse-first
    rm -rf build/
    rm -rf result result-cli
