
default: build test

# Build the binary
build: build-gomod2nix build-go build-nix

build-nix:
    nix build

build-gomod2nix:
    nix develop --command gomod2nix

build-go: build-gomod2nix
    nix develop --command go build -o lux ./cmd/lux

test: test-go

test-go:
    nix develop --command go test -v ./...

update: update-nix

update-nix:
  nix flake update

fmt: fmt-go

fmt-go:
    nix develop --command go fmt ./...
    shfmt -w -i 2 -ci ./*.sh 2>/dev/null || true

lint: lint-go

lint-go:
    go vet ./...

run-serve:
    go run ./cmd/lux serve

# Show configured LSPs
run-list:
    go run ./cmd/lux list

# Check LSP status
run-status:
    go run ./cmd/lux status

# Add a new LSP from a flake
run-add flake:
    go run ./cmd/lux add "{{flake}}"

# Run MCP server over stdio
run-mcp-stdio:
    go run ./cmd/lux mcp stdio

# Run MCP server over SSE
run-mcp-sse addr=":8080":
    go run ./cmd/lux mcp sse --addr "{{addr}}"

# Run MCP server over HTTP
run-mcp-http addr=":8081":
    go run ./cmd/lux mcp http --addr "{{addr}}"

# Clean build artifacts
clean:
    rm -f lux
    rm -rf result

# Update go dependencies and regenerate gomod2nix.toml
deps:
    nix develop --command go mod tidy
    nix develop --command gomod2nix

# Regenerate gomod2nix.toml (run after changing go.mod)
