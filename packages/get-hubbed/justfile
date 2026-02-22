# get-hubbed

default:
    @just --list

# Build the binary
build:
    nix build

build-gomod2nix:
    nix develop --command gomod2nix

build-go: build-gomod2nix
    nix develop --command go build -o get-hubbed ./cmd/get-hubbed

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

# Clean build artifacts
clean:
    rm -f get-hubbed
    rm -rf result
