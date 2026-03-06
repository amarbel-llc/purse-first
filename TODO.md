
## Feature Design Records

- [x] FDR: Sweatfile Configuration & Apply — `docs/features/0002-sweatfile-configuration.md`
- [x] FDR: Worktree Boundary Enforcement — `docs/features/0004-worktree-boundary-enforcement.md`
- [x] FDR: Per-Package Hook Architecture — `docs/features/0005-per-package-hook-architecture.md`
- [x] FDR: Merge & Close-Shop Lifecycle — `docs/features/0006-merge-close-shop-lifecycle.md`
- [x] FDR: Lux Service Daemon — `docs/features/0007-lux-service-daemon.md`

## P0

- [ ] P0: PreToolUse hooks not firing — hooks exist on disk and binary works correctly, but Claude Code doesn't enforce them after `purse-first install`. Neither grit nor chix hooks deny Bash commands. Investigate plugin hook loading/registration.

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
- [ ] lux service tests: add notification broadcast test — multiple sessions per workspace, verify LSP notification fans out to all clients
- [ ] lux service tests: add handleLSPNotification unit test
- [ ] lux service tests: add handlePoolStart / handlePoolStop unit tests
- [ ] lux service tests: add pool failure mode tests — build error, execute error, init error transitions
- [ ] lux service tests: add concurrent GetOrStart stress test for pool state machine races
- [ ] lux service tests: add config loading error path tests — missing files, invalid TOML, multiple LSPs
- [ ] lux service tests: add LSPClient connection failure / reconnection tests
- [ ] lux service BATS: add service-stop and service-start CLI tests
- [ ] bats-assert: show trailing whitespace in `--output differs--` / `--regular expression does not match output--` blocks (e.g. render spaces as `·` or `␣` at EOL, or show `$` line terminators like `cat -A`). Invisible trailing spaces cause regex mismatches that look identical in TAP output.
- [ ] FDR: versioned conformance test suites — mechanism for pairing a test suite version with the SUT version it targets, so version bumps surface which assertions need updating rather than requiring forensic debugging. Consider: version-tagged expected-output fixtures, SUT version gates in test setup, or a manifest mapping SUT version ranges to assertion variants.
