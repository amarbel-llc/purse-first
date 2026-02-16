# Pull go-lib-mcp and rust-lib-mcp into eat-libs

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Consolidate the MCP library ecosystem into the eat-libs (purse-first) monorepo for unified packaging and faster iteration.

**Architecture:** Move source from `~/eng/repos/go-lib-mcp` and `~/eng/repos/rust-lib-mcp` into `libs/go-mcp/` and `libs/rust-mcp/` respectively. Wire up a Go workspace so purse-first can import go-lib-mcp locally. Rust lib keeps its own sub-flake for standalone builds.

**Tech Stack:** Go workspace (`go.work`), Nix flakes (crane for Rust), just

---

### Task 1: Copy go-lib-mcp source into libs/go-mcp

**Files:**
- Create: `libs/go-mcp/` (directory tree)

**Step 1: Create target directory**

```bash
mkdir -p libs/go-mcp
```

**Step 2: Copy source, excluding .git, flake.lock, result, .direnv**

```bash
rsync -a --exclude='.git' --exclude='flake.lock' --exclude='result' --exclude='.direnv' \
  ~/eng/repos/go-lib-mcp/ libs/go-mcp/
```

**Step 3: Verify the copy**

Run: `ls libs/go-mcp/`
Expected: `examples executor flake.nix go.mod jsonrpc justfile LICENSE output protocol purse README.md server transport` (and .gitignore)

**Step 4: Verify Go module is intact**

Run: `head -2 libs/go-mcp/go.mod`
Expected: `module github.com/amarbel-llc/go-lib-mcp`

**Step 5: Commit**

```bash
git add libs/go-mcp/
git commit -m "Add go-lib-mcp source as libs/go-mcp"
```

---

### Task 2: Copy rust-lib-mcp source into libs/rust-mcp

**Files:**
- Create: `libs/rust-mcp/` (directory tree)

**Step 1: Create target directory**

```bash
mkdir -p libs/rust-mcp
```

**Step 2: Copy source, excluding .git, flake.lock, result, target, .direnv**

```bash
rsync -a --exclude='.git' --exclude='flake.lock' --exclude='result' --exclude='target' --exclude='.direnv' \
  ~/eng/repos/rust-lib-mcp/ libs/rust-mcp/
```

**Step 3: Verify the copy**

Run: `ls libs/rust-mcp/`
Expected: `Cargo.lock Cargo.toml examples flake.nix LICENSE MIGRATION_GUIDE.md README.md src` (and .gitignore)

**Step 4: Verify Cargo.toml is intact**

Run: `head -4 libs/rust-mcp/Cargo.toml`
Expected: `[package]` / `name = "mcp-server"`

**Step 5: Commit**

```bash
git add libs/rust-mcp/
git commit -m "Add rust-lib-mcp source as libs/rust-mcp"
```

---

### Task 3: Add go.work and remove it from .gitignore

**Files:**
- Create: `go.work`
- Modify: `.gitignore` (remove `go.work` and `go.work.sum` lines)

**Step 1: Remove go.work from .gitignore**

In `.gitignore`, remove these two lines:
```
go.work
go.work.sum
```

**Step 2: Create go.work**

```go
go 1.25.6

use (
	.
	./libs/go-mcp
)
```

**Step 3: Verify Go workspace resolves**

Run: `go work sync` (from repo root)
Expected: no errors

**Step 4: Verify both modules build**

Run: `go build ./...` (from repo root)
Expected: no errors — purse-first builds, and go workspace includes libs/go-mcp

Run: `cd libs/go-mcp && go build ./... && cd ../..`
Expected: no errors

**Step 5: Commit**

```bash
git add go.work .gitignore
git commit -m "Add Go workspace for purse-first + go-lib-mcp"
```

---

### Task 4: Add justfile targets for libs

**Files:**
- Modify: `justfile`

**Step 1: Add lib build and test targets**

Append to `justfile`:

```just
# Build go-lib-mcp
build-go-mcp:
    cd libs/go-mcp && go build ./...

# Test go-lib-mcp
test-go-mcp:
    cd libs/go-mcp && go test -v ./...

# Build rust-lib-mcp
build-rust-mcp:
    nix build ./libs/rust-mcp

# Test rust-lib-mcp
test-rust-mcp:
    cd libs/rust-mcp && nix develop --command cargo test
```

**Step 2: Update test-all to include lib tests**

Replace the existing `test-all` line:
```just
# Run all tests (unit + integration + libs)
test-all: test test-go-mcp test-rust-mcp test-integration
```

**Step 3: Run just to verify targets are listed**

Run: `just --list`
Expected: new targets `build-go-mcp`, `test-go-mcp`, `build-rust-mcp`, `test-rust-mcp` appear

**Step 4: Commit**

```bash
git add justfile
git commit -m "Add justfile targets for go-mcp and rust-mcp libs"
```

---

### Task 5: Verify everything builds and tests pass

**Files:** None (verification only)

**Step 1: Run go-mcp tests**

Run: `just test-go-mcp`
Expected: all tests pass

**Step 2: Run purse-first tests**

Run: `just test`
Expected: all tests pass (Go workspace doesn't break anything)

**Step 3: Run nix build for the marketplace**

Run: `nix build`
Expected: builds successfully (top-level flake is unaffected — it doesn't reference libs/)

**Step 4: Build rust-mcp via nix**

Run: `just build-rust-mcp`
Expected: builds successfully via crane sub-flake

**Step 5: Run rust-mcp tests**

Run: `just test-rust-mcp`
Expected: all tests pass

**Step 6: Run integration tests**

Run: `just test-integration`
Expected: all BATS tests pass (TAP-14 output)

---

## Layout After Migration

```
eat-libs/
├── go.work
├── go.mod
├── cmd/purse-first/
├── internal/
├── purse/
├── skills/
├── libs/
│   ├── go-mcp/
│   │   ├── go.mod
│   │   ├── flake.nix
│   │   ├── justfile
│   │   ├── protocol/
│   │   ├── server/
│   │   ├── transport/
│   │   ├── jsonrpc/
│   │   ├── executor/
│   │   ├── output/
│   │   ├── purse/
│   │   └── examples/
│   └── rust-mcp/
│       ├── Cargo.toml
│       ├── Cargo.lock
│       ├── flake.nix
│       └── src/
├── flake.nix
├── justfile
└── zz-tests_bats/
```

## Decisions

- **Monorepo packages** — source code lives in eat-libs, not flake inputs or submodules.
- **Go workspace** — `go.work` at root enables purse-first to import go-lib-mcp locally.
- **Simple copy** — no git subtree/history preservation, clean break.
- **Rust sub-flake** — `libs/rust-mcp/flake.nix` kept for standalone dev/test; not added to top-level flake.
- **No code changes** — diverged `purse` packages coexist, reconciliation is a follow-up.
- **Old repos removed later** once migration is stable.

## Future Work

- Reconcile diverged `purse` packages into one canonical location
- Have purse-first import go-lib-mcp types (jsonrpc, protocol, etc.)
- Update external consumers (lux, grit) to point at new module location
- Remove standalone repos
