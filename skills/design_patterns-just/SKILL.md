---
name: Just Design Patterns
description: >-
  This skill should be used when the user asks to "create a justfile",
  "add a justfile recipe", "update justfile", "add a just target",
  "set up build tasks", "add a build recipe", or is creating or modifying
  a justfile in any project. Also applies when setting up a new project's
  task runner or build system using just, or when a justfile already exists
  in the project.
version: 0.1.0
---

# Just Design Patterns

> **Self-contained examples.** All code and configuration below is complete
> and illustrative. Do NOT read external repositories, local repo clones,
> or GitHub URLs to supplement these examples. Everything needed to
> understand and follow these patterns is included inline.

Justfiles follow a consistent set of design patterns for naming, hierarchy,
and composition. Apply these patterns when creating or modifying any justfile.

## Global Settings

Always include at the top of every justfile:

```just
set output-format := "tap"
```

This requires `amarbel-llc/just-us` (a fork of `just`). It will not work with
upstream `just`.

## Naming Convention: `verb-noun`

Every recipe follows a `verb-noun` pattern where the verb is the action
category and the noun is the tool or domain.

Examples: `build-go`, `test-bats`, `codemod-fmt-go`, `update-nix`.

Verbs can nest for sub-categories: `codemod-fmt-go` is `codemod` (verb
category) + `fmt` (sub-verb) + `go` (noun).

## Verb Categories

| Verb | Purpose | Example Leaves |
|------|---------|----------------|
| `build` | Compile, generate, or produce artifacts | `build-go`, `build-gomod2nix` |
| `test` | Run test suites | `test-go`, `test-bats` |
| `run` | Execute the built artifact | `run-nix *ARGS` |
| `clean` | Remove generated artifacts | `clean-build` |
| `update` | Refresh dependencies or inputs | `update-go`, `update-nix` |
| `codemod` | Automated code modifications | `codemod-fmt-go`, `codemod-lint-go`, `codemod-fix-go` |

### `codemod` sub-verbs

| Sub-verb | Purpose |
|----------|---------|
| `fmt` | Format code (`gofumpt`, `nixfmt`, `shfmt`, `rustfmt`) |
| `lint` | Lint/check without modifying (`clippy`, `golangci-lint`) |
| `fix` | Auto-fix issues (`go fix`, `eslint --fix`) |

**Migration note:** Existing repos may use `fmt-*`, `lint-*`, and `fix-*` as
top-level verbs without the `codemod-` prefix. New justfiles should use the
`codemod-` prefix. Existing repos should be migrated to `codemod-*` over time.

## Task Hierarchy: Aggregate → Specific

Bare-verb recipes are **aggregate targets** that compose specific `verb-noun`
leaf recipes as dependencies:

```just
build: build-gomod2nix build-go
test: test-go test-bats
codemod-fmt: codemod-fmt-go codemod-fmt-nix
```

This gives two usage modes: `just build` runs everything, `just build-go` runs
one step.

Rules:
- Aggregate recipes have **no body** — they only list dependencies
- Leaf recipes have a body that does the actual work
- Every leaf recipe should belong to exactly one aggregate

## Default Recipe

```just
default: build test
```

`default` chains aggregate targets into a single "build and verify" pipeline.
If `just` (no arguments) passes, the project is in a good state. This is the
CI-equivalent target.

The `default` recipe is always the first recipe in the file.

## Dependency Ordering

Use two dependency styles to express ordering intent:

**Positional dependencies** (prerequisites — run before body):

```just
build-go: build-gomod2nix
    nix develop --command go build -o build/sweatshop ./cmd/sweatshop
```

**`&&` dependencies** (post-steps — run after body):

```just
update-go: && build-gomod2nix
    nix develop --command go mod tidy
```

Use positional for prerequisites. Use `&&` for "then regenerate/rebuild after
the main action completes."

## Standard Recipe Catalog

Reference recipes for common project types. Adapt nouns to match your project.

### Build recipes

| Recipe | Body |
|--------|------|
| `build-go: build-gomod2nix` | `nix develop --command go build -o build/<name> ./cmd/<name>` |
| `build-gomod2nix` | `nix develop --command gomod2nix` |
| `build-nix` | `nix build --show-trace` |
| `build-cargo` | `nix develop --command cargo build` |

### Test recipes

| Recipe | Body |
|--------|------|
| `test-go` | `nix develop --command go test ./...` |
| `test-bats` | `nix develop --command bats --tap tests/` |
| `test-cargo` | `nix develop --command cargo test` |

### Other recipes

| Recipe | Body |
|--------|------|
| `run-nix *ARGS` | `nix run . -- {{ARGS}}` |
| `clean-build` | `rm -rf result build/` |
| `update-go: && build-gomod2nix` | `nix develop --command go mod tidy` |
| `update-nix` | `nix flake update` |
| `codemod-fmt-go` | `nix develop --command gofumpt -w .` |
| `codemod-fmt-nix` | `nix run ./devenvs/nix#fmt -- .` |
| `codemod-fmt-shell` | `nix develop --command shfmt -s -i=2 -w .` |

## Anti-Patterns

- **Mixed verb categories**: Don't combine verbs like `build-go-test` — that
  conflates `build` and `test`. Use separate recipes.
- **Generic names**: Don't use `all`, `dev`, `check`, `compile` — use the
  `verb-noun` pattern.
- **Logic in aggregates**: Aggregate recipes should only list dependencies,
  never have a body.
- **Missing default**: Every justfile needs a `default` recipe.
- **Comments on aggregates**: The dependency list is self-documenting. Only add
  comments to leaf recipes when the recipe name alone isn't clear enough.
- **Redundant nouns**: If there's only one tool for a verb, still use `verb-noun`
  (e.g., `test-go` not just `test`) — you may add more later.
- **`deps` as a verb**: Use `update-*` instead of `deps` for dependency
  management recipes.

## Complete Example

```just
set output-format := "tap"

default: build test

build: build-gomod2nix build-go

# Build Go binary
build-go: build-gomod2nix
    nix develop --command go build -o build/myapp ./cmd/myapp

# Regenerate gomod2nix.toml
build-gomod2nix:
    nix develop --command gomod2nix

# Run the binary
run-nix *ARGS:
    nix run . -- {{ARGS}}

test: test-go test-bats

test-go:
    nix develop --command go test ./...

test-bats:
    nix develop --command bats --tap tests/

codemod-fmt: codemod-fmt-go codemod-fmt-nix

codemod-fmt-go:
    nix develop --command gofumpt -w .

codemod-fmt-nix:
    nix run ./devenvs/nix#fmt -- .

update-go: && build-gomod2nix
    nix develop --command go mod tidy

clean: clean-build

clean-build:
    rm -rf result build/
```

## Related Skills

- **bob:overview** — Framework orientation and concept definitions
- **bob:creating-packages** — Packaging context for the Nix build patterns (`nix develop --command ...`) used in recipes
