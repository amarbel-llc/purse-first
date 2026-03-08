
## Feature Design Records

- [x] FDR: Sweatfile Configuration & Apply — `docs/features/0002-sweatfile-configuration.md`
- [x] FDR: Worktree Boundary Enforcement — `docs/features/0004-worktree-boundary-enforcement.md`
- [x] FDR: Per-Package Hook Architecture — `docs/features/0005-per-package-hook-architecture.md`
- [x] FDR: Merge & Close-Shop Lifecycle — `docs/features/0006-merge-close-shop-lifecycle.md`
- [x] FDR: Lux Service Daemon — `docs/features/0007-lux-service-daemon.md`

## P0

- [ ] P0: PreToolUse hooks not firing — hooks exist on disk and binary works correctly, but Claude Code doesn't enforce them after `purse-first install`. Neither grit nor chix hooks deny Bash commands. Investigate plugin hook loading/registration.

## RFC-0001: Package Binary Interface

### go-mcp library
- [x] add built-in `generate-plugin` command to `command.App` with three modes: directory, PWD default, stdout (`-`)
- [x] wire `--skills-dir` flag on the built-in `generate-plugin` command
- [x] `HandleHook` must swallow decode errors (return nil, log to stderr) — RFC-0001 section 2.2 requires exit 0 on any error

### Per-package Go migrations
- [x] grit: remove manual `generate-plugin` dispatch from main.go (library command handles it)
- [x] get-hubbed: remove manual `generate-plugin` dispatch from main.go
- [x] mgp: remove manual `generate-plugin` dispatch from main.go
- [x] lux: remove `_generate` command, update `lux.nix` to call `generate-plugin`

### rust-mcp library
- [x] add `generate-plugin` subcommand support to rust-mcp (plugin.json, mappings.json, hooks; all three output modes)

### chix
- [ ] add `generate-plugin` to chix using rust-mcp support, replacing `purse-first generate-plugin` + `chix generate-hooks` split
- [ ] update `chix.nix` to use single `$out/bin/chix generate-plugin $out` call

### purse-first CLI
- [x] implement `purse-first install-dev-mcp <binary>` — calls `<binary> generate-plugin -`, writes `.mcp.json` to PWD

### Conformance tests
- [x] `zz-tests_bats/rfc-0001/generate_plugin_interface.bats` — covers all RFC-0001 section 2.1 requirements
- [x] `zz-tests_bats/rfc-0001/hook_interface.bats` — covers RFC-0001 section 2.2

## Other

- [x] tap-dancer rust: add `write_pragma` support (needed for `pragma +streamed-output`)
- [x] tap-dancer rust: add comment/description directive field to `TestResult` (`ok 1 - name # comment`)
- [x] tap-dancer rust: add carriage return stripping for YAML output fields
- [x] tap-dancer rust: add ANSI escape code stripping in YAML output
- [x] bootstrap root Cargo workspace — add root Cargo.toml with members [packages/chix, packages/tap-dancer/rust, libs/rust-mcp], delete per-crate Cargo.lock files, remove chix.nix vendor workaround, verify: `nix build .#chix`, `nix build .#tap-dancer`, `cargo test --workspace`
- [ ] update tap-dancer with latest tap amendments
- [ ] verify install-local skill path resolution (./skills/<name> in .claude-plugin/plugin.json may not resolve correctly — needs manual test)
- [ ] separate libs and marketplace generation into new `bob` repo
- [ ] migrate everything to latest MCP
- [x] tap-dancer rust: add quiet/suppress-YAML-block mode for test points
- [x] add PreToolUse hook support to rust-mcp (parity with go-mcp's HandleHook, GenerateHooks, ToolMapping/MapsTools, GeneratePlugin)
- [x] add PreToolUse hooks to chix (depends on rust-mcp hook support) — map nix/fh/cachix CLI tools to MCP equivalents
- [ ] FDR: single-package local test flow — `purse-first test-local .#chix` or similar that installs one package's hooks+MCP into an isolated Claude Code project config without touching global state
- [ ] explore purse-first neovim plugin packaging (ftplugins, lux config fragments, auto-discovery)
- [ ] lux: address the tension between having to define lux.lua with explicit filetypes against lux's own config declaration (lux already knows its filetypes via filetype/*.toml but neovim requires a static list in lsp/lux.lua)
- [ ] pivot hook matching to delegation: let packages provide custom matchers (e.g. a `MatchHook` callback on `Command`) instead of the framework owning all matching logic via static `CommandPrefixes`/`Extensions`
- [x] migrate grit to `bats_load_library bats-island`
- [x] migrate dodder to `bats_load_library bats-island` (moved to ~/eng/repos/dodder/TODO.md)
- [x] migrate pivy to `bats_load_library bats-island` (moved to ~/eng/repos/pivy/TODO.md)
- [x] migrate purse-first root tests to `bats_load_library bats-island`
- [x] update batman bats-testing skill examples/references to use bats-island
- [x] promote FDR 0001-bats-island from `exploring` to `proposed` after tests pass
- [x] fix grit rebase_mcp.bats: tests expect `packages/grit/result/bin/grit` symlink but no build step creates it
- [x] add ANSI color output to `tap-dancer *-test` for TTY's
- [x] fix sandcastle/batman socket permission failures: use `--allow-unix-sockets` in bats wrapper tests and sandcastle tests
- [x] P0: flaky timeout in `bats_wrapper_hide_passing_preserves_plan_and_version` (test 12) — 10s BATS_TEST_TIMEOUT too tight under parallel load
- [ ] lux service daemon: add SIGTERM/SIGINT signal handler to cancel context for graceful shutdown (socket cleanup on kill)
- [ ] ADR: assembly trampoline for launchd socket activation (go:cgo_import_dynamic pattern, why not cgo/dependency)
- [ ] lux service tests: add notification broadcast test — multiple sessions per workspace, verify LSP notification fans out to all clients
- [ ] lux service tests: add handleLSPNotification unit test
- [ ] lux service tests: add handlePoolStart / handlePoolStop unit tests
- [ ] lux service tests: add pool failure mode tests — build error, execute error, init error transitions
- [ ] lux service tests: add concurrent GetOrStart stress test for pool state machine races
- [ ] lux service tests: add config loading error path tests — missing files, invalid TOML, multiple LSPs
- [ ] lux service tests: add LSPClient connection failure / reconnection tests
- [x] fix flaky `TestServiceDocumentManager_OpenAlreadyOpenSendsDidChange` — notifications arrive in reversed order (didChange before didOpen) due to race in pipe-based test harness; `Notify` is async so recorder observes nondeterministic ordering
- [ ] lux service BATS: add service-stop and service-start CLI tests
- [ ] bats-assert: show trailing whitespace in `--output differs--` / `--regular expression does not match output--` blocks (e.g. render spaces as `·` or `␣` at EOL, or show `$` line terminators like `cat -A`). Invisible trailing spaces cause regex mismatches that look identical in TAP output.
- [ ] FDR: versioned conformance test suites — mechanism for pairing a test suite version with the SUT version it targets, so version bumps surface which assertions need updating rather than requiring forensic debugging. Consider: version-tagged expected-output fixtures, SUT version gates in test setup, or a manifest mapping SUT version ranges to assertion variants.
- [ ] spinclass: `list-agent-sessions` command — parse `~/.claude/projects/` to find session IDs for the current repo; if running inside a worktree, scope results to that worktree only
- [ ] add new `/commit` skill that includes the prompt transcript up until that
  point
- [ ] update go-cli-framework skill: import paths reference `amarbel-llc/go-lib-mcp` but library moved to `amarbel-llc/purse-first/libs/go-mcp`; API reference and examples are stale
- [ ] FDR: skill/docs freshness verification skill — a skill that audits all skills and docs against the current codebase, flags stale references (import paths, API signatures, examples), and produces a report of what needs updating

## CLAUDE.md improvements (from transcript analysis)

- [ ] add instruction: use `/tmp/lux-test-*` for socket paths, not `t.TempDir()` — worktree paths exceed 108-byte `sun_path` limit
- [ ] add instruction: BATS tests need `--allow-unix-sockets` for daemon tests in sandcastle
- [ ] add instruction: `tools -> service` import OK; `service -> tools` creates cycle; use func types to break
- [ ] add instruction: use polling-with-timeout for async test assertions, not `time.Sleep`
- [ ] add instruction: use `-run TestName` or `just test-lux`, not full-package `nix develop` runs
- [ ] default `log_tail: 50` on `chix build` calls to avoid token overflow

## develop_run improvements (from transcript analysis)

- [ ] chix `develop_run`: false positive on Go test `-run` regex patterns — `|` and `()` in args like `-run "TestA|TestB"` are Go regex, not shell operators. Metacharacter validation should only reject metacharacters in shell-interpreted positions, not inside arbitrary arg values
- [ ] chix `develop_run`: add `env` command examples to error message — when rejecting metacharacters, show the `env` pattern: `command: "env", args: ["VAR=val", "actual-cmd", "arg1"]`
- [ ] chix hook: when agents use `Bash` with `nix develop -c`/`nix develop --command`, redirect to `develop_run` with a corrected invocation rather than just blocking — agents fall back to Bash 3/4 times after `develop_run` metacharacter rejections, bypassing the tool entirely
- [ ] chix hook: when agents use `Bash` with `nix search` against a remote nixpkgs SHA (2+ min eval), redirect to `chix search` tool instead

## Skill improvements (from transcript analysis)

- [ ] sub-agent exploration: instruct "use Glob/Grep tools, never bash grep/ls/find; use Glob before Read on directories, never Read a directory path"
- [ ] sub-agent delegation: include explicit stop conditions for error recovery ("if X fails, STOP and report back")
- [ ] sub-agent-driven-development skill: add guidance to keep sequential dependent tasks in main context rather than spawning sub-agents

# OTHER

- [ ] enable conformance tests to be exposed as commands that can be run in
  recipes / pipelines in downstream projects (like the rfc-0001 conformance
  tests)
