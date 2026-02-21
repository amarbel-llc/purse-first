
default: && update build-all test-all

# Build all packages (default = marketplace bundle)
build-all:
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

# Run tests
test:
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

# Run all tests (unit + integration + libs + framework)
test-all: test test-go-mcp test-rust-mcp test-integration test-hooks test-lifecycle test-template

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
