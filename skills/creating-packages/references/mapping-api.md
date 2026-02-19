# Tool Mapping API Reference

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

Mappings redirect built-in Claude Code tools (e.g., `Bash`, `Grep`, `Read`) to MCP server tools exposed by the package. The purse-first PreToolUse hook denies matching Bash commands and suggests specific MCP tools instead.

## How Matching Works

`FindMatch` iterates mappings in order and returns the **first** match. This means:
- **Specific mappings must come before general ones** -- `"git log"` before `"git "`
- A general catch-all at the end handles unrecognized subcommands

## Targeted Per-Subcommand Mappings

The recommended pattern is one mapping per subcommand, each suggesting only the relevant tool(s). This gives focused denial messages instead of listing every tool in the package.

### Flag-based CLI Example

```go
reason := "Use the git-mcp MCP tool instead of shelling out. When the command uses git -C <path>, pass that path as the repo_path parameter"

b := purse.NewPluginBuilder("my-mcp").
    Command("my-mcp").
    StdioTransport().
    // Specific mappings first (matched before the catch-all)
    Mapping("Bash").
    CommandPrefixes("git status").
    Tool("status", "checking repository status").
    Reason(reason).
    Done().
    Mapping("Bash").
    CommandPrefixes("git log").
    Tool("log", "viewing commit history").
    Reason(reason).
    Done().
    Mapping("Bash").
    CommandPrefixes("git branch").
    Tool("branch_list", "listing branches").
    Tool("branch_create", "creating a new branch").
    Reason(reason).
    Done().
    // General catch-all last (for unrecognized subcommands)
    Mapping("Bash").
    CommandPrefixes("git ", "git -C ").
    Tool("status", "checking repository status").
    Tool("log", "viewing commit history").
    Tool("branch_list", "listing branches").
    Tool("branch_create", "creating a new branch").
    Reason("Use git-mcp MCP tools for git operations instead of shelling out").
    Done()
```

Key points:
- Each `Mapping("Bash")` creates a separate `MappingEntry` in `mappings.json`
- Use `CommandPrefixes` for Bash commands, `Extensions` for file-based tools (Read, Grep, etc.)
- Multiple prefixes per mapping are supported (e.g., `git checkout` and `git switch` both map to `checkout`)
- Multiple tools per mapping are supported (e.g., `git branch` suggests both `branch_list` and `branch_create`)
- The `Reason` string is shown in the denial message along with the tool suggestions

## Writing Mappings in postInstall

When using mappings, the `generate-plugin` command must also write `mappings.json`. Call `BuildMappings` and `WriteMappings`:

```go
if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
    b := purse.NewPluginBuilder("my-mcp").
        Command("my-mcp").
        StdioTransport().
        Mapping("Bash").
        // ... mappings ...
        Done()

    p := b.Build()
    dir := flag.Arg(1)

    if err := purse.WritePlugin(dir, p); err != nil {
        log.Fatalf("generating plugin: %v", err)
    }

    if mf := b.BuildMappings(); mf != nil {
        if err := purse.WriteMappings(dir, p.Name, mf); err != nil {
            log.Fatalf("generating mappings: %v", err)
        }
    }

    return
}
```

This produces both `$out/share/purse-first/<name>/plugin.json` and `$out/share/purse-first/<name>/mappings.json`. The `postInstall` in `flake.nix` stays the same -- the binary handles both files.

## MappingBuilder API

| Method | Description |
|--------|-------------|
| `Mapping(replaces)` | Start a new mapping that replaces the named tool (`"Bash"`, `"Read"`, `"Grep"`, etc.) |
| `CommandPrefixes(p...)` | Match Bash commands starting with any of these prefixes |
| `Extensions(e...)` | Match file operations on files with these extensions |
| `Tool(name, useWhen)` | Suggest this MCP tool as a replacement |
| `Reason(reason)` | Set the denial message shown to the user |
| `Done()` | Finish this mapping and return to the PluginBuilder |
| `BuildMappings()` | Returns `*MappingFile` (nil if no mappings declared) |
