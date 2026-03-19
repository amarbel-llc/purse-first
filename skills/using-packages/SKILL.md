---
name: Using Packages
description: This skill should be used when the user asks "how do purse-first hooks work", "why is my tool being denied", "how to customize mappings", "troubleshoot package discovery", "how does tool routing work", "how do I override a mapping", "debug purse-first", or is experiencing unexpected tool denials, mapping conflicts, or package discovery failures. Also applies when working with `.purse-first/` project-local overrides, `$XDG_STATE_HOME/purse-first/` user overrides, or `purse-first install` and `purse-first hook` commands.
version: 0.1.0
---

# Using Purse-First Packages

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

This skill covers the consumer side of purse-first: how installed packages work at runtime, how tool routing decisions are made, and how to troubleshoot issues.

For creating new packages, see the **bob:creating-packages** skill.
For framework orientation, see the **bob:overview** skill.

## How Tool Routing Works

Each package with tool mappings ships its own PreToolUse hook. There is no central purse-first hook — each package binary handles tool routing independently.

When Claude Code invokes a built-in tool (Bash, Read, Grep, etc.), per-package PreToolUse hooks fire in parallel:

1. Claude Code calls the package's `hooks/pre-tool-use` wrapper script
2. The wrapper delegates to `<binary> hook`, which reads hook input from stdin
3. The binary matches the tool invocation against its registered `ToolMapping` declarations:
   - For Bash: matches `CommandPrefixes` against the command string
   - For file tools (Read, Grep, Glob): matches `Extensions` against the file path
   - Catch-all mappings (no prefixes or extensions) match unconditionally
4. If a match is found: writes a deny response with the MCP tool name (e.g., `mcp__plugin_grit_grit__status`)
5. If no match: writes nothing (implicit allow)

Multiple packages can each have their own hooks — they run in parallel and each only knows about its own tool mappings.

## Package Hook Architecture

Each package that declares tool mappings ships a `hooks/` directory:

```
share/purse-first/<name>/hooks/
├── hooks.json      # PreToolUse matcher (auto-generated)
└── pre-tool-use    # Wrapper script calling `<binary> hook`
```

The `hooks.json` uses `${CLAUDE_PLUGIN_ROOT}` (without shell quoting) for portable path resolution. The `pre-tool-use` script uses an absolute Nix store path to the package binary (resolved at build time via `os.Executable()`).

Hooks are registered with Claude Code automatically when the package is installed — no manual `purse-first install --hooks` step is needed.

## Troubleshooting

### "Why is my tool being denied?"

When a tool is denied, the package's hook prints the MCP tool name and usage hint. To investigate:

1. Check which packages have hooks: `ls share/purse-first/*/hooks/hooks.json`
2. Check the matcher in each hooks.json to see which tools are intercepted
3. Check the package's mappings: `cat share/purse-first/<name>/mappings.json`
4. The deny message includes the full MCP tool name (e.g., `mcp__plugin_grit_grit__status`)

### "Package not discovered"

Verify the package layout:

```bash
# Check the package directory exists
ls $PURSE_FIRST_PLUGINS_DIR/<package-name>/plugin.json

# Verify plugin.name matches directory name
cat $PURSE_FIRST_PLUGINS_DIR/<package-name>/plugin.json | jq .name

# Check the binary exists
ls $(dirname $PURSE_FIRST_PLUGINS_DIR)/../bin/<command>
```

### "Skills not loading"

Skills are discovered by globbing `skills/*/SKILL.md` under each package directory:

```bash
# Check skills exist in the package
ls $PURSE_FIRST_PLUGINS_DIR/<package-name>/skills/*/SKILL.md

# Verify YAML frontmatter has name and description
head -5 $PURSE_FIRST_PLUGINS_DIR/<package-name>/skills/<skill>/SKILL.md
```

### "Hook not firing"

Per-package hooks are registered automatically via the package's `hooks/hooks.json`. Check:

1. The package has `hooks/hooks.json`: `ls share/purse-first/<name>/hooks/`
2. The `pre-tool-use` script is executable: `ls -la share/purse-first/<name>/hooks/pre-tool-use`
3. The binary responds to `hook` subcommand: `echo '{"tool_name":"Bash","tool_input":{"command":"git status"}}' | <binary> hook`
4. Claude Code loaded the hooks (restart Claude Code to pick up new hooks)

## Key Commands

| Command | Purpose |
|---------|---------|
| `purse-first install` | Install marketplace |
| `purse-first validate` | Validate plugin/mapping/marketplace documents |
| `purse-first validate-mcp <binary>` | Validate MCP server over stdio (initialize, tools/list, resources) |
| `purse-first generate-marketplace` | Build marketplace.json from discovered packages |
| `purse-first generate-local-plugin` | Discover skills and update plugin.json |
| `<package> hook` | Per-package PreToolUse handler (called by hooks/pre-tool-use, not manually) |

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `PURSE_FIRST_PLUGINS_DIR` | `<exe>/../share/purse-first/` | Override package discovery root |
| `XDG_STATE_HOME` | `$HOME/.local/state` | Base for user-level mapping overrides |
| `XDG_CONFIG_HOME` | `$HOME/.config` | Base for user-level configuration |
