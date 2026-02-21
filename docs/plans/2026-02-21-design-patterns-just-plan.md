# `design_patterns-just` Skill Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a bob skill that codifies justfile design patterns so agents produce consistent, well-structured justfiles across all projects.

**Architecture:** Single `SKILL.md` file at `skills/design_patterns-just/SKILL.md`, registered in `.claude-plugin/plugin.json`.

**Tech Stack:** Markdown with YAML frontmatter (purse-first skill format)

---

### Task 1: Create the skill directory and SKILL.md

**Files:**
- Create: `skills/design_patterns-just/SKILL.md`

**Step 1: Create the skill file**

```markdown
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

Justfiles follow a consistent set of design patterns for naming, hierarchy,
and composition. Apply these patterns when creating or modifying any justfile.

## Global Settings

Always include at the top of every justfile:

\`\`\`just
set output-format := "tap"
\`\`\`

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
| `run` | Execute the built artifact | `run *ARGS` |
| `clean` | Remove generated artifacts | `clean` |
| `update` | Refresh dependencies or inputs | `update-go`, `update-nix` |
| `codemod` | Automated code modifications | `codemod-fmt-go`, `codemod-lint-go`, `codemod-fix-go` |

### `codemod` sub-verbs

| Sub-verb | Purpose |
|----------|---------|
| `fmt` | Format code (`gofumpt`, `nixfmt`, `shfmt`, `rustfmt`) |
| `lint` | Lint/check without modifying (`clippy`, `golangci-lint`) |
| `fix` | Auto-fix issues (`go fix`, `eslint --fix`) |

## Task Hierarchy: Aggregate → Specific

Bare-verb recipes are **aggregate targets** that compose specific `verb-noun`
leaf recipes as dependencies:

\`\`\`just
build: build-gomod2nix build-go
test: test-go test-bats
codemod-fmt: codemod-fmt-go codemod-fmt-nix
\`\`\`

This gives two usage modes: `just build` runs everything, `just build-go` runs
one step.

Rules:
- Aggregate recipes have **no body** — they only list dependencies
- Leaf recipes have a body that does the actual work
- Every leaf recipe should belong to exactly one aggregate

## Default Recipe

\`\`\`just
default: build test
\`\`\`

`default` chains aggregate targets into a single "build and verify" pipeline.
If `just` (no arguments) passes, the project is in a good state. This is the
CI-equivalent target.

The `default` recipe is always the first recipe in the file.

## Dependency Ordering

Use two dependency styles to express ordering intent:

**Positional dependencies** (prerequisites — run before body):

\`\`\`just
build-go: build-gomod2nix
    nix develop --command go build -o build/sweatshop ./cmd/sweatshop
\`\`\`

**`&&` dependencies** (post-steps — run after body):

\`\`\`just
update-go: && build-gomod2nix
    nix develop --command go mod tidy
\`\`\`

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
| `run *ARGS` | `nix run . -- {{ARGS}}` |
| `clean` | `rm -rf result build/` |
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

## Complete Example

\`\`\`just
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
run *ARGS:
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

clean:
    rm -rf result build/
\`\`\`
```

**Step 2: Verify no syntax issues in the frontmatter**

Run: `head -5 skills/design_patterns-just/SKILL.md`
Expected: YAML frontmatter with `---` delimiters, `name`, `description`, `version`

**Step 3: Commit**

```bash
git add skills/design_patterns-just/SKILL.md
git commit -m "feat: add design_patterns-just skill for justfile conventions"
```

---

### Task 2: Register the skill in plugin.json

**Files:**
- Modify: `.claude-plugin/plugin.json`

**Step 1: Add the skill path to the skills array**

Add `"./skills/design_patterns-just"` to the `skills` array in `.claude-plugin/plugin.json`:

```json
{
  "author": {
    "name": "friedenberg"
  },
  "description": "Package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages for humans and agents like Claude Code",
  "name": "bob",
  "skills": [
    "./skills/overview",
    "./skills/creating-packages",
    "./skills/using-packages",
    "./skills/context-saving",
    "./skills/go-cli-framework",
    "./skills/voldemort",
    "./skills/design_patterns-just"
  ]
}
```

**Step 2: Commit**

```bash
git add .claude-plugin/plugin.json
git commit -m "feat: register design_patterns-just skill in plugin manifest"
```

---

### Task 3: Build and verify

**Step 1: Build the flake to verify skill discovery**

Run: `just build` (or `nix build` in purse-first)
Expected: Build succeeds, skill is included in output

**Step 2: Verify skill is present in build output**

Run: `ls result/share/purse-first/bob/skills/design_patterns-just/SKILL.md`
Expected: File exists

**Step 3: Verify generated plugin.json includes the skill**

Run: `cat result/share/purse-first/bob/plugin.json | jq '.skills'`
Expected: Array includes `"./skills/design_patterns-just"`

**Step 4: Commit any fixups if needed**
