
- [ ] fix issues with skills not loading
- [ ] separate libs and marketplace generation into new `bob` repo
- [ ] migrate everything to latest MCP
- [ ] add rust tap-dancer library
- [ ] add PreToolUse hook support to rust-mcp (parity with go-mcp's HandleHook, GenerateHooks, ToolMapping/MapsTools, GeneratePlugin)
- [ ] add PreToolUse hooks to chix (depends on rust-mcp hook support) — map nix/fh/cachix CLI tools to MCP equivalents
- [ ] explore purse-first neovim plugin packaging (ftplugins, lux config fragments, auto-discovery)
- [ ] pivot hook matching to delegation: let packages provide custom matchers (e.g. a `MatchHook` callback on `Command`) instead of the framework owning all matching logic via static `CommandPrefixes`/`Extensions`
- [ ] sandcastle: fix double-escaping of `!` in arguments — `sandcastle --shell bash -- echo '!x'` prints `\\!x` instead of `!x`, which breaks bats `--filter-tags '!tag'` negation (workaround: dodder@ed7c31d69 uses `grep -L` instead)
