# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Shell-agnostic git worktree session manager. Manages worktree lifecycles: creating worktrees with config inheritance, attaching via `zmx` sessions, rebasing/merging back to main, and cleaning up. Aliased as `sc`. Part of the purse-first monorepo.

## Build & Test Commands

All commands run from `packages/spinclass/`:

```sh
just build          # nix build the spinclass package
just test           # run Go tests with TAP-14 output via tap-dancer
just fmt            # go fmt
just lint           # go vet
just clean          # remove result symlink
```

Tests run through the monorepo root's devShell and use `tap-dancer` for TAP-14 formatted output. The test command is:

```sh
nix develop <root> --command go run packages/tap-dancer/ go-test ./packages/spinclass/...
```

## Architecture

**CLI layer** (`cmd/spinclass/main.go`): Cobra commands mapping to internal packages. Global flags: `--format` (tap/table), `--verbose`.

**Core workflow** (`internal/shop/`): Orchestrates create, attach, and fork — the main user-facing operations. `Create()` sets up worktree + sweatfile + Claude trust. `Attach()` calls Create then delegates to an `Executor`. `Fork()` branches from current worktree.

**Executor abstraction** (`internal/executor/`): Interface for session attachment. `ZmxExecutor` (production, uses `zmx` sessions) and `ShellExecutor` (used by merge). Tests use a `mockExecutor`.

**Git operations** (`internal/git/`): Thin wrapper — all commands use `git -C <dir>`. Two modes: `Run()` captures output, `RunPassthrough()` streams to console. Supports env injection (e.g. `GIT_SEQUENCE_EDITOR=true` for non-interactive rebase).

**Worktree resolution** (`internal/worktree/`): Resolves targets to `ResolvedPath` (branch, abs path, repo path, session key). Bare name → `<repo>/.worktrees/<branch>`, relative path → resolved from repo root, absolute → used directly.

**Sweatfile config** (`internal/sweatfile/`): TOML-based hierarchical configuration. Merges global (`~/.config/spinclass/sweatfile`) → intermediate parent dirs → repo-level. Supports `git-excludes`, `claude-allow`, and `envrc-directives` arrays (nil = inherit, empty = clear, non-empty = append), `[env]` table (map merge, repo keys override base), and `[hooks]` table with `create` and `stop` lifecycle hooks (scalar override). The create hook runs after worktree config is applied and receives `$WORKTREE` as an env var.

**Merge/Pull/Clean** (`internal/merge/`, `internal/pull/`, `internal/clean/`): Post-session workflows. Merge rebases onto default branch then ff-only merges. Pull scans repos and rebases clean worktrees. Clean removes fully-merged worktree branches.

**Permission tiers** (`internal/perms/`): Claude Code hook integration. Tier-based permission rules stored as JSON (`global.json` + `repos/<repo>.json`). Implements `PermissionRequest` protocol for Claude's pre-tool-use hooks.

**Claude integration** (`internal/claude/`): Updates `~/.claude.json` to trust worktree paths. Applies `claude-allow` rules from sweatfile to `.claude/settings.local.json`.

**TAP output** (`internal/tap/`): Local TAP-14 writer used across commands for structured output with YAML diagnostic blocks.

## Key Patterns

- **TAP-14 everywhere**: Most commands default to `--format tap`. Diagnostics include git stderr and exit codes in YAML blocks. Verbose mode adds git output to passing tests.
- **Path resolution**: `worktree.ResolvePath()` is the single entry point for target → absolute path conversion. Session keys follow `<repo-dirname>/<branch>` format.
- **Sweatfile merging**: Config files walk from `$HOME` down to repo root, merging at each level. This is the mechanism for project-specific git excludes and Claude permissions.
- **External tool deps**: `git`, `zmx` (session manager), `gum` (interactive selection in merge).

## Nix Build

Built via `lib/packages/spinclass.nix` using `mkGoWorkspaceModule` from the monorepo. Shares `goWorkspaceSrc` and `goVendorHash` with other Go packages — local code changes don't invalidate the vendor hash. Shell completions (bash + fish) are packaged separately and joined via `symlinkJoin`. The `sc` alias is a symlink created in `postBuild`.

## Go Workspace

Module: `github.com/amarbel-llc/spinclass`. Uses `replace` directive for local `tap-dancer/go` dependency. Part of the purse-first `go.work` workspace.
