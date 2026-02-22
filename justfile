
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

build-gomod2nix:
    nix develop --command gomod2nix

build-go: build-gomod2nix
    nix develop --command go build -o purse-first ./cmd/purse-first

# Build Homebrew tap formula templates
build-brew:
    nix build .#homebrew-tap

# Build marketplace without hooks
build-no-hooks:
    nix build .#marketplace-no-hooks

# Install MCP servers and purse-first hook
install:
    nix run .#install

# Remove purse-first hooks from Claude Code settings
uninstall-hooks:
    nix run -- .#default -- uninstall-hooks

update: update-nix

update-nix:
    nix flake update

# Test individual Go packages
test-grit:
    nix develop --command go test ./packages/grit/...

test-get-hubbed:
    nix develop --command go test ./packages/get-hubbed/...

test-lux:
    nix develop --command go test ./packages/lux/...

test-tap-dancer-go:
    nix develop --command go test ./packages/tap-dancer/go/...

# Test Rust packages
test-chix:
    cd packages/chix && nix develop ../../ --command cargo test

test-tap-dancer-rust:
    cd packages/tap-dancer/rust && nix develop ../../../ --command cargo test

# Run tests
test-go:
    nix develop --command go test ./...

# Run tests with verbose output
test-v:
    nix develop --command go test -v ./...

# Format code
fmt:
    nix develop --command go fmt ./...

# Lint code
lint:
    nix develop --command go vet ./...

# Update go dependencies and regenerate gomod2nix.toml
deps:
    nix develop --command go mod tidy
    nix develop --command gomod2nix

# Run integration tests
test-integration:
    nix build
    nix develop --command bats --tap \
      zz-tests_bats/validate_marketplace.bats \
      zz-tests_bats/validate_documents.bats \
      zz-tests_bats/validate_plugin_repos.bats

# Validate plugin repos have correct .claude-plugin/plugin.json
test-validate-repos:
    nix develop --command bats --tap zz-tests_bats/validate_plugin_repos.bats

# Run validate-specific BATS tests
test-validate:
    nix build
    nix develop --command bats --tap zz-tests_bats/validate_documents.bats

# Run hook unit tests + BATS hook I/O tests
test-hooks:
    nix develop --command go test -v ./internal/hook/...
    nix build
    nix develop --command bats --tap zz-tests_bats/hook_io.bats

# Run lifecycle tests
test-lifecycle:
    nix build
    nix develop --command bats --tap zz-tests_bats/hook_lifecycle.bats

# Validate own plugin manifest
validate:
    nix develop --command go run ./cmd/purse-first validate .claude-plugin/plugin.json

# Run Homebrew tap BATS tests
test-brew:
    nix build .#homebrew-tap
    nix develop --command bats --tap zz-tests_bats/homebrew_tap.bats

test: \
    test-chix \
    test-get-hubbed \
    test-go \
    test-go-mcp \
    test-grit \
    test-hooks \
    test-integration \
    test-lifecycle \
    test-lux \
    test-rust-mcp \
    test-tap-dancer-go \
    test-tap-dancer-rust \
    test-template

# Build go-lib-mcp
build-go-mcp:
    cd libs/go-mcp && nix develop ../../ --command go build ./...

# Test go-lib-mcp
test-go-mcp:
    cd libs/go-mcp && nix develop ../../ --command go test -v ./...

# Build rust-lib-mcp
build-rust-mcp:
    nix build ./libs/rust-mcp

# Test rust-lib-mcp
test-rust-mcp:
    cd libs/rust-mcp && nix develop --command cargo test

# Test command package specifically
test-command:
    cd libs/go-mcp && nix develop ../../ --command go test -v ./command/

# Verify mkMarketplace.nix parses
check-lib:
    nix-instantiate --parse lib/mkMarketplace.nix

# Verify template parses
check-template:
    nix-instantiate --parse templates/marketplace/flake.nix

# Run template tests
test-template:
    nix develop --command bats --tap zz-tests_bats/marketplace_template.bats

# Clean build artifacts
clean:
    rm -f purse-first
    rm -rf result
