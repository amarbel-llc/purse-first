# grit

default:
    @just --list

# Build the binary
build:
    nix build

build-gomod2nix:
    nix develop --command gomod2nix

build-go: build-gomod2nix
    nix develop --command go build -o grit ./cmd/grit

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

# Install as Claude Code MCP server
install-claude: build
    claude mcp add grit -- ./result/bin/grit

# Clean build artifacts
clean:
    rm -f grit
    rm -rf result

# Run BATS integration tests
test-bats: build
    just zz-tests_bats/test

# Run all tests (Go + BATS)
test-all: test test-bats
