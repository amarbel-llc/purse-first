# chix

default:
    @just --list

# Build with nix
build:
    nix build .#default --show-trace

# Build with cargo (dev mode)
dev:
    cargo build

# Watch for changes and rebuild
watch:
    cargo watch -x build

# Run tests
test:
    nix develop --command cargo test

# Format code
fmt:
    cargo fmt

# Run cargo check and clippy
check:
    cargo check
    cargo clippy -- -D warnings

# Clean build artifacts
clean:
    cargo clean
    rm -rf result
