# claude-plugin-marketplace

default:
    @just --list

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

build-nix-mcp:
    nix build .#nix-mcp-server

build-purse-first:
    nix build .#purse-first

build-gomod2nix:
    nix develop --command gomod2nix

build-go: build-gomod2nix
    nix develop --command go build -o purse-first ./cmd/purse-first

# Install MCP servers and purse-first hook
install:
    nix run .#install

# Update MCP server flake inputs
update-plugins:
    nix flake update grit get-hubbed lux nix-mcp-server

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
    go vet ./...

# Update go dependencies and regenerate gomod2nix.toml
deps:
    nix develop --command go mod tidy
    nix develop --command gomod2nix

# Run integration tests with sandcastle
test-integration:
    nix build
    nix develop --command zz-tests_bats/bin/run-sandcastle-bats.bash \
      bats --tap zz-tests_bats/*.bats

# Run all tests (unit + integration)
test-all: test test-integration

# Clean build artifacts
clean:
    rm -f purse-first
    rm -rf result
