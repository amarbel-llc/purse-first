# CLAUDE.md

## Overview

TAP-14 writer library (Go + Rust) and purse-first skill plugin. Consolidates hand-rolled TAP writers from sweatshop, just-us, and purse-first into shared libraries.

## Build & Test

```sh
just build          # nix build
just test           # Run all tests (Go + Rust)
just test-go        # Go unit tests only
just test-rust      # Rust unit tests only
just fmt            # Format all code
just deps           # Update Go dependencies (go mod tidy + gomod2nix)
```

## Code Style

- Go: `gofumpt`, package name `tap`, module `github.com/amarbel-llc/tap-dancer/go`
- Rust: `cargo fmt` + `cargo clippy`, crate name `tap-dancer`
- Nix: `nixfmt-rfc-style`

## Testing

Both language implementations verify the same TAP-14 spec compliance: version line, plan, test points (ok/not ok), YAML diagnostics, directives (SKIP/TODO), bail out, comments.
