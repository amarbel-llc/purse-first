
- [ ] fix issues with skills not loading
- [ ] separate libs and marketplace generation into new `bob` repo
- [ ] migrate everything to latest MCP
- [ ] add rust tap-dancer library
- [ ] add plugin generation to rust-mcp (parity with go-mcp's GeneratePlugin, GenerateMappings, GenerateHooks, skill discovery)
- [ ] explore purse-first neovim plugin packaging (ftplugins, lux config fragments, auto-discovery)
- [ ] pivot hook matching to delegation: let packages provide custom matchers (e.g. a `MatchHook` callback on `Command`) instead of the framework owning all matching logic via static `CommandPrefixes`/`Extensions`
