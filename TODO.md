
## Feature Design Records

- [x] FDR: Sweatfile Configuration & Apply — `docs/features/0002-sweatfile-configuration.md`
- [ ] FDR: Worktree Boundary Enforcement — deny→notify design reversal, boundary rules undocumented
- [ ] FDR: Per-Package Hook Architecture — central→per-package pivot, fail-open model, MapsTools/CommandPrefixes matching
- [ ] FDR: Merge & Close-Shop Lifecycle — complex state machine, silent error swallowing, --merge-on-close semantics
- [ ] FDR: Lux Service Daemon — session/workspace/pool hierarchy, socket activation, notification broadcasting

## Other

- [ ] verify install-local skill path resolution (./skills/<name> in .claude-plugin/plugin.json may not resolve correctly — needs manual test)
- [ ] separate libs and marketplace generation into new `bob` repo
- [ ] migrate everything to latest MCP
- [ ] add rust tap-dancer library
- [ ] add PreToolUse hook support to rust-mcp (parity with go-mcp's HandleHook, GenerateHooks, ToolMapping/MapsTools, GeneratePlugin)
- [ ] add PreToolUse hooks to chix (depends on rust-mcp hook support) — map nix/fh/cachix CLI tools to MCP equivalents
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
- [ ] fix sandcastle/batman socket permission failures: use `--allow-unix-sockets` in bats wrapper tests and sandcastle tests
