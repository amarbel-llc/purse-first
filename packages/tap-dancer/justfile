default: build test

build: build-nix build-cli

build-nix:
    nix build

build-cli:
    nix build .#cli

test: test-go test-rust

test-go:
    nix develop --command bash -c "cd go && go test ./..."

test-rust:
    nix develop --command bash -c "cd rust && cargo test"

fmt: fmt-go fmt-rust fmt-nix

fmt-go:
    nix develop --command bash -c "cd go && gofumpt -w ."

fmt-rust:
    nix develop --command bash -c "cd rust && cargo fmt"

fmt-nix:
    nix run ~/eng/devenvs/nix#fmt -- flake.nix

deps:
    nix develop --command bash -c "cd go && go mod tidy && gomod2nix"

test-validate:
    nix develop --command bash -c 'echo -e "TAP version 14\n1..2\nok 1 - pass\nok 2 - pass" | go run ./go/cmd/tap-dancer validate'

test-go-test:
    nix develop --command bash -c "cd go && go run ./cmd/tap-dancer go-test ./... | go run ./cmd/tap-dancer validate"

clean:
    rm -rf result
