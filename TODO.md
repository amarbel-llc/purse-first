## Feature Design Records

- [x] FDR: Sweatfile Configuration & Apply ---
  `docs/features/0002-sweatfile-configuration.md`
- [x] FDR: Worktree Boundary Enforcement ---
  `docs/features/0004-worktree-boundary-enforcement.md`
- [x] FDR: Per-Package Hook Architecture ---
  `docs/features/0005-per-package-hook-architecture.md`
- [x] FDR: Merge & Close-Shop Lifecycle ---
  `docs/features/0006-merge-close-shop-lifecycle.md`
- [x] FDR: Lux Service Daemon --- `docs/features/0007-lux-service-daemon.md`

## P0

- [x] P0: Move `plugin.json` into `.claude-plugin/` subdirectory (1d2357a)
- [x] P0: Hook execution error after discovery fix --- root cause:
  `hookSpecificOutput` was missing required `hookEventName: "PreToolUse"` field.
  Claude Code validates hook output against a schema that requires this
  discriminator. Fixed in go-mcp (`hook.go`).

## RFC-0001: Package Binary Interface

### go-mcp library

- [x] add built-in `generate-plugin` command to `command.App` with three modes:
  directory, PWD default, stdout (`-`)
- [x] wire `--skills-dir` flag on the built-in `generate-plugin` command
- [x] `HandleHook` must swallow decode errors (return nil, log to stderr) ---
  RFC-0001 section 2.2 requires exit 0 on any error

### Per-package Go migrations

- [x] grit: remove manual `generate-plugin` dispatch from main.go (library
  command handles it)
- [x] get-hubbed: remove manual `generate-plugin` dispatch from main.go
- [x] mgp: remove manual `generate-plugin` dispatch from main.go
- [x] lux: remove `_generate` command, update `lux.nix` to call
  `generate-plugin`

### purse-first CLI

- [x] implement `purse-first install-dev-mcp <binary>` --- calls
  `<binary> generate-plugin -`, writes `.mcp.json` to PWD

### Conformance tests

- [x] `zz-tests_bats/rfc-0001/generate_plugin_interface.bats` --- covers all
  RFC-0001 section 2.1 requirements
- [x] `zz-tests_bats/rfc-0001/hook_interface.bats` --- covers RFC-0001 section
  2.2

## Other

- [ ] `purse-first validate` for hook output: validate hookSpecificOutput
  against Claude Code's expected schema (hookEventName required,
  permissionDecision enum, etc.) --- could run as part of
  `just test-integration` or as a standalone
  `purse-first validate --hook-output` mode

- [ ] skill: downstream marketplace consumer workflow (how to consume
  mkMarketplace outputs in a parent flake, handle collisions, install
  separately)

- [ ] FDR: eliminate .claude-plugin/marketplace.json collision so downstream
  consumers can symlinkJoin multiple marketplaces without infraInputs workaround

- [x] grit: add `--amend` support to `commit` tool (git commit --amend)

- [x] grit: add soft reset mode to `reset` tool (git reset --soft via `soft` +
  `ref` params) --- needed for amend workaround and squash flows

- [x] sandcastle BATS: migrate to bats-emo conformance pattern (require_bin
  SANDCASTLE_BIN sandcastle)

- [x] update tap-dancer with latest tap amendments

- [ ] verify install-local skill path resolution (./skills/`<name>`{=html} in
  .claude-plugin/plugin.json may not resolve correctly --- needs manual test)

- [x] add HTTP/SSE transport support to InstallMCP (MCPURL, MCPHeaders on App)

- [ ] FDR: single-package local test flow --- `purse-first test-local .#chix` or
  similar that installs one package's hooks+MCP into an isolated Claude Code
  project config without touching global state

- [ ] explore purse-first neovim plugin packaging (ftplugins, lux config
  fragments, auto-discovery)

- [ ] pivot hook matching to delegation: let packages provide custom matchers
  (e.g. a `MatchHook` callback on `Command`) instead of the framework owning all
  matching logic via static `CommandPrefixes`/`Extensions`

- [x] migrate grit to `bats_load_library bats-island`

- [x] migrate dodder to `bats_load_library bats-island` (moved to
  \~/eng/repos/dodder/TODO.md)

- [x] migrate pivy to `bats_load_library bats-island` (moved to
  \~/eng/repos/pivy/TODO.md)

- [x] migrate purse-first root tests to `bats_load_library bats-island`

- [x] update batman bats-testing skill examples/references to use bats-island

- [x] promote FDR 0001-bats-island from `exploring` to `proposed` after tests
  pass

- [x] fix grit rebase_mcp.bats: tests expect `packages/grit/result/bin/grit`
  symlink but no build step creates it

- [x] add ANSI color output to `tap-dancer *-test` for TTY's

- [x] fix sandcastle/batman socket permission failures: use
  `--allow-unix-sockets` in bats wrapper tests and sandcastle tests

- [x] sandcastle BATS: tests fail with
  `socat socket(AF_UNIX): Operation not permitted` --- sandcastle's own bwrap
  bridge setup can't create unix sockets on Linux (seccomp blocks
  `socket(AF_UNIX)` even for the outer socat processes). Fixed by skipping
  network infrastructure (proxy servers + socat bridge) in `initialize()` when
  `isInsideBwrap()` detects a bwrap ancestor.

- [x] P0: flaky timeout in
  `bats_wrapper_hide_passing_preserves_plan_and_version` (test 12) --- 10s
  BATS_TEST_TIMEOUT too tight under parallel load

- [x] fix flaky `TestServiceDocumentManager_OpenAlreadyOpenSendsDidChange` ---
  notifications arrive in reversed order (didChange before didOpen) due to race
  in pipe-based test harness; `Notify` is async so recorder observes
  nondeterministic ordering



- [x] spinclass: add test for remote branch detection in ResolvePath (requires
  setting up local remote in test)



- [x] update go-cli-framework skill: import paths reference
  `amarbel-llc/go-lib-mcp` but library moved to
  `amarbel-llc/purse-first/libs/go-mcp`; API reference and examples are stale


## CLAUDE.md improvements (from transcript analysis)

- [ ] add instruction: use `/tmp/lux-test-*` for socket paths, not `t.TempDir()`
  --- worktree paths exceed 108-byte `sun_path` limit
- [ ] add instruction: BATS tests need `--allow-unix-sockets` for daemon tests
  in sandcastle
- [ ] add instruction: `tools -> service` import OK; `service -> tools` creates
  cycle; use func types to break
- [ ] add instruction: use polling-with-timeout for async test assertions, not
  `time.Sleep`
- [ ] add instruction: use `-run TestName` or `just test-lux`, not full-package
  `nix develop` runs
- [ ] default `log_tail: 50` on `chix build` calls to avoid token overflow

## develop_run improvements (from transcript analysis)

- [ ] chix `develop_run`: false positive on Go test `-run` regex patterns ---
  `|` and `()` in args like `-run "TestA|TestB"` are Go regex, not shell
  operators. Metacharacter validation should only reject metacharacters in
  shell-interpreted positions, not inside arbitrary arg values
- [ ] chix `develop_run`: add `env` command examples to error message --- when
  rejecting metacharacters, show the `env` pattern:
  `command: "env", args: ["VAR=val", "actual-cmd", "arg1"]`
- [ ] chix hook: when agents use `Bash` with
  `nix develop -c`/`nix develop --command`, redirect to `develop_run` with a
  corrected invocation rather than just blocking --- agents fall back to Bash
  3/4 times after `develop_run` metacharacter rejections, bypassing the tool
  entirely
- [ ] chix hook: when agents use `Bash` with `nix search` against a remote
  nixpkgs SHA (2+ min eval), redirect to `chix search` tool instead

## Skill improvements (from transcript analysis)

- [ ] sub-agent exploration: instruct "use Glob/Grep tools, never bash
  grep/ls/find; use Glob before Read on directories, never Read a directory
  path"
- [ ] sub-agent delegation: include explicit stop conditions for error recovery
  ("if X fails, STOP and report back")
- [ ] sub-agent-driven-development skill: add guidance to keep sequential
  dependent tasks in main context rather than spawning sub-agents

# OTHER

- [ ] enable conformance tests to be exposed as commands that can be run in
  recipes / pipelines in downstream projects (like the rfc-0001 conformance
  tests)

- [ ] add claude-mcp-tool annotation modes to sweatfile (so read-only mode means
  all mcp's that support sweatfiles operate in read-only mode, etc)

- [ ] remove brew tap infrastructure from `.github/workflows/release.yml`
  (tarball packaging, update-tap job, brew-build-tarball/brew-update-hashes
  references)

- [ ] `package brew`: add optional top-level `version` field to
  `brew-config.json` for explicit meta-formula version (currently derived from
  first alphabetically-sorted package)

- [ ] `package brew --release`: add `--release` flag that creates a GitHub
  release on `releaseRepo` and uploads all tarballs via `gh release create`,
  eliminating the need for a separate release step
