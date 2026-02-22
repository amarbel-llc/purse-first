# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

**grit** is an MCP (Model Context Protocol) server that exposes git operations over JSON-RPC via stdin/stdout. It's designed to be launched by MCP clients like Claude Code. Built in Go, packaged with Nix using `gomod2nix`.

## Build & Dev Commands

All commands use the justfile and run inside a Nix dev shell:

```sh
just build              # Nix build -> ./result/bin/grit
just build-go           # Go build via nix develop -> ./grit
just test               # go test ./...
just test-v             # go test -v ./...
just fmt                # go fmt ./...
just lint               # go vet ./...
just deps               # go mod tidy + gomod2nix
just install-claude     # Register as Claude Code MCP server
just clean              # Remove build artifacts
```

## Architecture

Single external dependency: `github.com/amarbel-llc/go-lib-mcp` (MCP server framework providing protocol types, transport, and tool registry).

### Entry Point

`cmd/grit/main.go` — Sets up signal handling, creates a stdio JSON-RPC transport, registers all tools, and runs the MCP server loop.

### Git Execution Layer

`internal/git/exec.go` — Single function `Run(ctx, dir, args...)` that shells out to `git`, captures stdout/stderr, and returns output or a formatted error. Every tool handler calls through this.

### Tool System

`internal/tools/registry.go` — `RegisterAll()` aggregates tool registrations from category files. Each category file (e.g., `status.go`, `branch.go`) defines tool schemas and handler functions following the pattern:

```go
func handleXXX(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error)
```

Tool categories: status/diff, log/show/blame, staging (add/reset), commit, branch (list/create/checkout), remote (fetch/pull/push/list).

### Safety Constraints

- Force push is blocked on `main`/`master` branches
- `git_reset` is soft-only (no working tree modifications)

## Nix Flake

Follows the stable-first nixpkgs convention (`nixpkgs` = stable, `nixpkgs-master` = unstable). Uses devenv flakes from `github:friedenberg/eng` for Go and shell environments. Built with `pkgs.buildGoApplication` via `gomod2nix`.
