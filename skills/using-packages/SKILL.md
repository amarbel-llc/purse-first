---
name: Using Packages
description: This skill should be used when the user asks "how do purse-first hooks work", "why is my tool being denied", "how to customize mappings", "troubleshoot package discovery", "how does tool routing work", "how do I override a mapping", "debug purse-first", or is experiencing unexpected tool denials, mapping conflicts, or package discovery failures. Also applies when working with `.purse-first/` project-local overrides, `$XDG_STATE_HOME/purse-first/` user overrides, or `purse-first install` and `purse-first hook` commands.
version: 0.1.0
---

# Using Purse-First Packages

This skill covers the consumer side of purse-first: how installed packages work at runtime, how tool routing decisions are made, and how to troubleshoot issues.

For creating new packages, see the **bob:creating-packages** skill.
For framework orientation, see the **bob:overview** skill.

## How Tool Routing Works

When Claude Code invokes a built-in tool (Bash, Read, Grep, etc.), purse-first's PreToolUse hook fires:

1. Hook receives: tool name, tool input (command, file path, etc.), working directory
2. Loads mappings from three sources (lowest to highest priority):
   - **Package-shipped:** `share/purse-first/<name>/mappings.json`
   - **Global user:** `$XDG_STATE_HOME/purse-first/*.json` (default: `~/.local/state/purse-first/`)
   - **Project-local:** `.purse-first/*.json` in the working directory
3. Matches the tool invocation against mapping rules:
   - For Bash: matches `command_prefixes` against the command string
   - For file tools (Read, Grep, Glob): matches `extensions` against the file path
   - Catch-all mappings (no prefixes or extensions) match unconditionally
4. If a match is found: denies the built-in tool and suggests MCP alternatives
5. If no match: allows the tool to proceed

## Mapping Precedence

Higher-priority sources override lower-priority ones for the same `(replaces, server)` pair:

```
Package-shipped (lowest)  →  Global user  →  Project-local (highest)
```

This means you can always override or disable a package's mappings without modifying the package itself.

## Customizing Mappings

### Project-Local Overrides

Create `.purse-first/<name>.json` in your project root to override mappings for that project:

```json
{
  "server": "grit",
  "mappings": [
    {
      "replaces": "Bash",
      "command_prefixes": ["git log"],
      "tools": [
        {"name": "log", "use_when": "viewing commit history"}
      ],
      "reason": "Use grit log for structured commit data"
    }
  ]
}
```

### Global User Overrides

Place mapping files in `$XDG_STATE_HOME/purse-first/` (typically `~/.local/state/purse-first/`) to apply overrides across all projects.

### Disabling a Mapping

To disable a specific package mapping, create an override with an empty `tools` array for the same `replaces`/`server` pair. The higher-priority source will take precedence.

## Troubleshooting

### "Why is my tool being denied?"

When a tool is denied, purse-first prints the mapping's `reason` field and lists suggested MCP tools. To investigate:

1. Check which mappings are loaded: look at `share/purse-first/*/mappings.json` in the marketplace
2. Check for project-local overrides: `ls .purse-first/`
3. Check for user overrides: `ls $XDG_STATE_HOME/purse-first/`

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

Verify hooks are installed in Claude Code settings:

```bash
purse-first install --hooks
```

This registers the `purse-first hook` command as a PreToolUse handler in Claude Code's settings.

## Key Commands

| Command | Purpose |
|---------|---------|
| `purse-first install` | Install marketplace and register hooks |
| `purse-first install --hooks` | Only register hook handlers |
| `purse-first uninstall-hooks` | Remove hook handlers |
| `purse-first hook` | PreToolUse handler (called by Claude Code, not manually) |
| `purse-first validate` | Validate plugin/mapping/marketplace documents |
| `purse-first generate-marketplace` | Build marketplace.json from discovered packages |
| `purse-first generate-local-plugin` | Discover skills and update plugin.json |

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `PURSE_FIRST_PLUGINS_DIR` | `<exe>/../share/purse-first/` | Override package discovery root |
| `XDG_STATE_HOME` | `$HOME/.local/state` | Base for user-level mapping overrides |
| `XDG_CONFIG_HOME` | `$HOME/.config` | Base for user-level configuration |
