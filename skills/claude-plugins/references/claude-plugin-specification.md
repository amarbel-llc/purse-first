# Claude Code Plugin Specification

> Downloaded from the official Claude Code documentation on 2026-02-25.
> Sources:
> - https://code.claude.com/docs/en/plugins
> - https://code.claude.com/docs/en/plugins-reference
> - https://code.claude.com/docs/en/skills
> - https://code.claude.com/docs/en/sub-agents
> - https://code.claude.com/docs/en/hooks
> - https://code.claude.com/docs/en/plugin-marketplaces

---

## Table of Contents

1. [Plugin Structure Overview](#plugin-structure-overview)
2. [Plugin Manifest Schema](#plugin-manifest-schema)
3. [Skills](#skills)
4. [Agents (Subagents)](#agents-subagents)
5. [Hooks](#hooks)
6. [MCP Servers](#mcp-servers)
7. [LSP Servers](#lsp-servers)
8. [Settings](#settings)
9. [Plugin Marketplaces](#plugin-marketplaces)
10. [Plugin Installation Scopes](#plugin-installation-scopes)
11. [CLI Commands](#cli-commands)
12. [Debugging](#debugging)

---

## Plugin Structure Overview

A plugin is a self-contained directory that extends Claude Code with custom
functionality. Components are placed at the plugin root, **not** inside
`.claude-plugin/`. Only `plugin.json` goes inside `.claude-plugin/`.

```
plugin-name/
├── .claude-plugin/           # Metadata directory (optional)
│   └── plugin.json             # plugin manifest
├── commands/                 # Slash commands (legacy; use skills/)
├── agents/                   # Specialized subagent definitions
├── skills/                   # Agent Skills with SKILL.md files
├── hooks/                    # Event handlers
│   └── hooks.json
├── settings.json             # Default settings applied when enabled
├── .mcp.json                 # MCP server configurations
├── .lsp.json                 # LSP server configurations
├── scripts/                  # Hook and utility scripts
├── LICENSE
├── CHANGELOG.md
└── README.md
```

### File Locations Reference

| Component       | Default Location             | Purpose                                                       |
|:----------------|:-----------------------------|:--------------------------------------------------------------|
| **Manifest**    | `.claude-plugin/plugin.json` | Plugin metadata and configuration (optional)                  |
| **Commands**    | `commands/`                  | Skill Markdown files (legacy; use `skills/` for new skills)   |
| **Agents**      | `agents/`                    | Subagent Markdown files                                       |
| **Skills**      | `skills/`                    | Skills with `<name>/SKILL.md` structure                       |
| **Hooks**       | `hooks/hooks.json`           | Hook configuration                                            |
| **MCP servers** | `.mcp.json`                  | MCP server definitions                                        |
| **LSP servers** | `.lsp.json`                  | Language server configurations                                |
| **Settings**    | `settings.json`              | Default configuration (currently only `agent` key supported)  |

---

## Plugin Manifest Schema

The `.claude-plugin/plugin.json` file defines plugin metadata and configuration.
The manifest is **optional**. If omitted, Claude Code auto-discovers components
in default locations and derives the plugin name from the directory name.

### Complete Schema

```json
{
  "name": "plugin-name",
  "version": "1.2.0",
  "description": "Brief plugin description",
  "author": {
    "name": "Author Name",
    "email": "author@example.com",
    "url": "https://github.com/author"
  },
  "homepage": "https://docs.example.com/plugin",
  "repository": "https://github.com/author/plugin",
  "license": "MIT",
  "keywords": ["keyword1", "keyword2"],
  "commands": ["./custom/commands/special.md"],
  "agents": "./custom/agents/",
  "skills": "./custom/skills/",
  "hooks": "./config/hooks.json",
  "mcpServers": "./mcp-config.json",
  "outputStyles": "./styles/",
  "lspServers": "./.lsp.json"
}
```

### Required Fields

If you include a manifest, `name` is the only required field.

| Field  | Type   | Description                               | Example              |
|:-------|:-------|:------------------------------------------|:---------------------|
| `name` | string | Unique identifier (kebab-case, no spaces) | `"deployment-tools"` |

### Metadata Fields

| Field         | Type   | Description                                | Example                                            |
|:--------------|:-------|:-------------------------------------------|:---------------------------------------------------|
| `version`     | string | Semantic version                           | `"2.1.0"`                                          |
| `description` | string | Brief explanation of plugin purpose        | `"Deployment automation tools"`                    |
| `author`      | object | Author information                         | `{"name": "Dev Team", "email": "dev@company.com"}` |
| `homepage`    | string | Documentation URL                          | `"https://docs.example.com"`                       |
| `repository`  | string | Source code URL                            | `"https://github.com/user/plugin"`                 |
| `license`     | string | License identifier                         | `"MIT"`, `"Apache-2.0"`                            |
| `keywords`    | array  | Discovery tags                             | `["deployment", "ci-cd"]`                          |

### Component Path Fields

| Field          | Type                  | Description                         | Example                                |
|:---------------|:----------------------|:------------------------------------|:---------------------------------------|
| `commands`     | string\|array         | Additional command files/directories| `"./custom/cmd.md"` or `["./cmd1.md"]` |
| `agents`       | string\|array         | Additional agent files              | `"./custom/agents/reviewer.md"`        |
| `skills`       | string\|array         | Additional skill directories        | `"./custom/skills/"`                   |
| `hooks`        | string\|array\|object | Hook config paths or inline config  | `"./my-extra-hooks.json"`              |
| `mcpServers`   | string\|array\|object | MCP config paths or inline config   | `"./my-extra-mcp-config.json"`         |
| `outputStyles` | string\|array         | Output style files/directories      | `"./styles/"`                          |
| `lspServers`   | string\|array\|object | LSP configs                         | `"./.lsp.json"`                        |

### Path Behavior Rules

- Custom paths **supplement** default directories — they don't replace them.
- All paths must be relative to plugin root and start with `./`.
- Multiple paths can be specified as arrays.

### Environment Variables

- **`${CLAUDE_PLUGIN_ROOT}`**: Absolute path to the plugin directory. Use in
  hooks, MCP servers, and scripts for correct paths regardless of installation
  location.

---

## Skills

Skills extend Claude's capabilities. A `SKILL.md` file with instructions creates
a `/skill-name` shortcut. Claude uses skills when relevant, or users invoke them
directly.

### Skill Directory Structure

```
skills/
├── code-review/
│   ├── SKILL.md           # Main instructions (required)
│   ├── reference.md       # Optional supporting file
│   ├── examples/
│   │   └── sample.md
│   └── scripts/
│       └── validate.sh
└── deploy/
    └── SKILL.md
```

Plugin skills are namespaced: `/<plugin-name>:<skill-name>`.

### SKILL.md Format

```yaml
---
name: my-skill
description: What this skill does and when to use it
argument-hint: "[issue-number]"
disable-model-invocation: true
user-invocable: true
allowed-tools: Read, Grep, Glob
model: sonnet
context: fork
agent: Explore
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "./scripts/security-check.sh"
---

Your skill instructions in markdown...
```

### Frontmatter Reference

All fields are optional. Only `description` is recommended.

| Field                      | Required    | Description                                                                                                                        |
|:---------------------------|:------------|:-----------------------------------------------------------------------------------------------------------------------------------|
| `name`                     | No          | Display name. If omitted, uses directory name. Lowercase letters, numbers, hyphens only (max 64 chars).                            |
| `description`              | Recommended | What the skill does and when to use it. Claude uses this for auto-invocation decisions.                                            |
| `argument-hint`            | No          | Hint shown during autocomplete. Example: `[issue-number]`                                                                          |
| `disable-model-invocation` | No          | `true` prevents Claude from automatically loading this skill. Default: `false`.                                                    |
| `user-invocable`           | No          | `false` hides from `/` menu. Default: `true`.                                                                                      |
| `allowed-tools`            | No          | Tools Claude can use without asking permission when this skill is active.                                                          |
| `model`                    | No          | Model to use when this skill is active.                                                                                            |
| `context`                  | No          | `fork` to run in a forked subagent context.                                                                                        |
| `agent`                    | No          | Which subagent type to use when `context: fork` is set. Options: `Explore`, `Plan`, `general-purpose`, or custom agent names.      |
| `hooks`                    | No          | Hooks scoped to this skill's lifecycle.                                                                                            |

### String Substitutions

| Variable               | Description                                          |
|:------------------------|:-----------------------------------------------------|
| `$ARGUMENTS`           | All arguments passed when invoking the skill          |
| `$ARGUMENTS[N]`        | Specific argument by 0-based index                   |
| `$N`                   | Shorthand for `$ARGUMENTS[N]`                         |
| `${CLAUDE_SESSION_ID}` | Current session ID                                    |

### Dynamic Context Injection

The `` !`command` `` syntax runs shell commands before skill content is sent to
Claude. Command output replaces the placeholder.

```yaml
---
name: pr-summary
description: Summarize changes in a pull request
context: fork
agent: Explore
---

## Pull request context
- PR diff: !`gh pr diff`
- Changed files: !`gh pr diff --name-only`
```

### Invocation Control

| Frontmatter                      | User can invoke | Claude can invoke |
|:---------------------------------|:----------------|:------------------|
| (default)                        | Yes             | Yes               |
| `disable-model-invocation: true` | Yes             | No                |
| `user-invocable: false`          | No              | Yes               |

---

## Agents (Subagents)

Subagents are specialized AI assistants that handle specific task types. Each
runs in its own context window with a custom system prompt, specific tool access,
and independent permissions.

### Agent File Format

```markdown
---
name: code-reviewer
description: Reviews code for quality and best practices. Use proactively after code changes.
tools: Read, Glob, Grep, Bash
disallowedTools: Write, Edit
model: sonnet
permissionMode: default
maxTurns: 50
skills:
  - api-conventions
  - error-handling-patterns
memory: user
background: false
isolation: worktree
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "./scripts/validate-command.sh"
---

You are a code reviewer. Analyze code and provide specific, actionable feedback.
```

### Agent Frontmatter Fields

| Field             | Required | Description                                                                                          |
|:------------------|:---------|:-----------------------------------------------------------------------------------------------------|
| `name`            | Yes      | Unique identifier (lowercase, hyphens)                                                               |
| `description`     | Yes      | When Claude should delegate to this subagent                                                         |
| `tools`           | No       | Tools the subagent can use. Inherits all if omitted. Use `Task(worker, researcher)` to restrict.     |
| `disallowedTools` | No       | Tools to deny, removed from inherited or specified list                                              |
| `model`           | No       | `sonnet`, `opus`, `haiku`, or `inherit`. Default: `inherit`                                          |
| `permissionMode`  | No       | `default`, `acceptEdits`, `dontAsk`, `bypassPermissions`, or `plan`                                  |
| `maxTurns`        | No       | Maximum agentic turns before stopping                                                                |
| `skills`          | No       | Skills to preload into context at startup                                                            |
| `mcpServers`      | No       | MCP servers available to this subagent                                                               |
| `hooks`           | No       | Lifecycle hooks scoped to this subagent                                                              |
| `memory`          | No       | Persistent memory scope: `user`, `project`, or `local`                                               |
| `background`      | No       | `true` to always run as background task. Default: `false`                                            |
| `isolation`       | No       | `worktree` to run in a temporary git worktree                                                        |

### Agent Scope (Priority Order)

| Location                     | Scope                   | Priority    |
|:-----------------------------|:------------------------|:------------|
| `--agents` CLI flag          | Current session         | 1 (highest) |
| `.claude/agents/`            | Current project         | 2           |
| `~/.claude/agents/`          | All your projects       | 3           |
| Plugin's `agents/` directory | Where plugin is enabled | 4 (lowest)  |

### Built-in Subagents

| Agent            | Model    | Tools      | Purpose                                 |
|:-----------------|:---------|:-----------|:----------------------------------------|
| Explore          | Haiku    | Read-only  | File discovery, code search             |
| Plan             | Inherit  | Read-only  | Codebase research for planning          |
| general-purpose  | Inherit  | All        | Complex multi-step tasks                |
| Bash             | Inherit  | Terminal   | Running terminal commands               |
| statusline-setup | Sonnet   | Setup      | Configure status line                   |
| Claude Code Guide| Haiku    | Info       | Questions about Claude Code features    |

---

## Hooks

Hooks are user-defined shell commands or LLM prompts that execute automatically
at specific points in Claude Code's lifecycle.

### Hook Configuration Format

Hooks are defined in `hooks/hooks.json` in the plugin root, or inline in
`plugin.json`:

```json
{
  "description": "Optional description",
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/scripts/format-code.sh",
            "timeout": 30,
            "statusMessage": "Formatting code...",
            "async": false
          }
        ]
      }
    ]
  }
}
```

### Hook Events

| Event                | When it fires                                        | Can block? |
|:---------------------|:-----------------------------------------------------|:-----------|
| `SessionStart`       | Session begins or resumes                            | No         |
| `UserPromptSubmit`   | User submits a prompt                                | Yes        |
| `PreToolUse`         | Before a tool call executes                          | Yes        |
| `PermissionRequest`  | Permission dialog appears                            | Yes        |
| `PostToolUse`        | After a tool call succeeds                           | No         |
| `PostToolUseFailure` | After a tool call fails                              | No         |
| `Notification`       | Claude Code sends a notification                     | No         |
| `SubagentStart`      | Subagent spawned                                     | No         |
| `SubagentStop`       | Subagent finishes                                    | Yes        |
| `Stop`               | Claude finishes responding                           | Yes        |
| `TeammateIdle`       | Agent team teammate about to go idle                 | Yes        |
| `TaskCompleted`      | Task being marked completed                          | Yes        |
| `ConfigChange`       | Configuration file changes                           | Yes        |
| `WorktreeCreate`     | Worktree being created                               | Yes        |
| `WorktreeRemove`     | Worktree being removed                               | No         |
| `PreCompact`         | Before context compaction                            | No         |
| `SessionEnd`         | Session terminates                                   | No         |

### Matcher Patterns

The `matcher` field is a regex string that filters when hooks fire. Omit or use
`"*"` to match all.

| Event                                                              | Matches on        | Examples                                     |
|:-------------------------------------------------------------------|:------------------|:---------------------------------------------|
| `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `PermissionReq` | tool name         | `Bash`, `Edit\|Write`, `mcp__.*`             |
| `SessionStart`                                                     | session source    | `startup`, `resume`, `clear`, `compact`      |
| `SessionEnd`                                                       | exit reason       | `clear`, `logout`, `prompt_input_exit`       |
| `Notification`                                                     | notification type | `permission_prompt`, `idle_prompt`           |
| `SubagentStart`, `SubagentStop`                                    | agent type        | `Bash`, `Explore`, `Plan`, custom names      |
| `PreCompact`                                                       | trigger type      | `manual`, `auto`                             |
| `ConfigChange`                                                     | config source     | `user_settings`, `project_settings`, `skills`|

### Hook Handler Types

#### Command Hooks

```json
{
  "type": "command",
  "command": "${CLAUDE_PLUGIN_ROOT}/scripts/check.sh",
  "timeout": 600,
  "statusMessage": "Checking...",
  "async": false
}
```

#### Prompt Hooks

```json
{
  "type": "prompt",
  "prompt": "Evaluate if Claude should stop: $ARGUMENTS. Check if all tasks are complete.",
  "model": "haiku",
  "timeout": 30
}
```

Prompt hooks return: `{ "ok": true }` or `{ "ok": false, "reason": "..." }`

#### Agent Hooks

```json
{
  "type": "agent",
  "prompt": "Verify all unit tests pass. $ARGUMENTS",
  "model": "haiku",
  "timeout": 60
}
```

### Common Hook Handler Fields

| Field           | Required | Description                                              |
|:----------------|:---------|:---------------------------------------------------------|
| `type`          | Yes      | `"command"`, `"prompt"`, or `"agent"`                    |
| `timeout`       | No       | Seconds before canceling. Defaults: 600/30/60            |
| `statusMessage` | No       | Custom spinner message                                   |
| `once`          | No       | If `true`, runs only once per session (skills only)      |

### Command Hook Additional Fields

| Field     | Required | Description                                              |
|:----------|:---------|:---------------------------------------------------------|
| `command` | Yes      | Shell command to execute                                 |
| `async`   | No       | `true` to run in background without blocking             |

### Hook Input (Common Fields via stdin JSON)

| Field             | Description                        |
|:------------------|:-----------------------------------|
| `session_id`      | Current session identifier         |
| `transcript_path` | Path to conversation JSON          |
| `cwd`             | Current working directory          |
| `permission_mode` | Current permission mode            |
| `hook_event_name` | Name of the event that fired       |

### Exit Code Behavior

- **Exit 0**: Success. Claude Code parses stdout for JSON output.
- **Exit 2**: Blocking error. stderr fed back to Claude as error.
- **Other**: Non-blocking error. stderr shown in verbose mode.

### PreToolUse Decision Control

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow|deny|ask",
    "permissionDecisionReason": "Reason string",
    "updatedInput": { "field": "new value" },
    "additionalContext": "Extra context for Claude"
  }
}
```

### PermissionRequest Decision Control

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PermissionRequest",
    "decision": {
      "behavior": "allow|deny",
      "updatedInput": { "command": "npm run lint" },
      "updatedPermissions": { ... },
      "message": "Reason for deny",
      "interrupt": false
    }
  }
}
```

### Stop / SubagentStop Decision Control

```json
{
  "decision": "block",
  "reason": "Must complete all tasks first"
}
```

### Global JSON Output Fields

| Field            | Default | Description                                          |
|:-----------------|:--------|:-----------------------------------------------------|
| `continue`       | `true`  | `false` stops Claude entirely                        |
| `stopReason`     | none    | Message shown to user when `continue` is `false`     |
| `suppressOutput` | `false` | `true` hides stdout from verbose mode                |
| `systemMessage`  | none    | Warning message shown to the user                    |

---

## MCP Servers

Plugins can bundle Model Context Protocol (MCP) servers. Configured in
`.mcp.json` at the plugin root or inline in `plugin.json`.

```json
{
  "mcpServers": {
    "plugin-database": {
      "command": "${CLAUDE_PLUGIN_ROOT}/servers/db-server",
      "args": ["--config", "${CLAUDE_PLUGIN_ROOT}/config.json"],
      "env": {
        "DB_PATH": "${CLAUDE_PLUGIN_ROOT}/data"
      }
    },
    "plugin-api-client": {
      "command": "npx",
      "args": ["@company/mcp-server", "--plugin-mode"],
      "cwd": "${CLAUDE_PLUGIN_ROOT}"
    }
  }
}
```

- Plugin MCP servers start automatically when the plugin is enabled.
- Servers appear as standard MCP tools in Claude's toolkit.
- MCP tool names follow pattern: `mcp__<server>__<tool>`.

---

## LSP Servers

Plugins can provide Language Server Protocol servers for code intelligence.
Configured in `.lsp.json` at the plugin root or inline in `plugin.json`.

```json
{
  "go": {
    "command": "gopls",
    "args": ["serve"],
    "extensionToLanguage": {
      ".go": "go"
    }
  }
}
```

### Required Fields

| Field                 | Description                                  |
|:----------------------|:---------------------------------------------|
| `command`             | The LSP binary to execute (must be in PATH)  |
| `extensionToLanguage` | Maps file extensions to language identifiers  |

### Optional Fields

| Field                   | Description                                               |
|:------------------------|:----------------------------------------------------------|
| `args`                  | Command-line arguments                                    |
| `transport`             | `stdio` (default) or `socket`                             |
| `env`                   | Environment variables                                     |
| `initializationOptions` | Options passed during initialization                      |
| `settings`              | Settings via `workspace/didChangeConfiguration`           |
| `workspaceFolder`       | Workspace folder path                                     |
| `startupTimeout`        | Max startup wait (milliseconds)                           |
| `shutdownTimeout`       | Max graceful shutdown wait (milliseconds)                 |
| `restartOnCrash`        | Auto-restart on crash                                     |
| `maxRestarts`           | Max restart attempts                                      |

---

## Settings

Plugins can include `settings.json` at the plugin root. Currently only the
`agent` key is supported, which activates a custom agent as the main thread.

```json
{
  "agent": "security-reviewer"
}
```

---

## Plugin Marketplaces

A marketplace is a catalog that distributes plugins. Defined in
`.claude-plugin/marketplace.json`.

### Marketplace Schema

```json
{
  "name": "company-tools",
  "owner": {
    "name": "DevTools Team",
    "email": "devtools@example.com"
  },
  "metadata": {
    "description": "Brief marketplace description",
    "version": "1.0.0",
    "pluginRoot": "./plugins"
  },
  "plugins": [
    {
      "name": "code-formatter",
      "source": "./plugins/formatter",
      "description": "Automatic code formatting",
      "version": "2.1.0",
      "author": { "name": "DevTools Team" },
      "homepage": "https://docs.example.com",
      "repository": "https://github.com/user/plugin",
      "license": "MIT",
      "keywords": ["formatting"],
      "category": "productivity",
      "tags": ["lint", "style"],
      "strict": true
    }
  ]
}
```

### Required Marketplace Fields

| Field     | Type   | Description                 |
|:----------|:-------|:----------------------------|
| `name`    | string | Marketplace identifier      |
| `owner`   | object | Maintainer info (`name` required, `email` optional) |
| `plugins` | array  | List of available plugins   |

### Plugin Source Types

| Source        | Type                              | Fields                                |
|:--------------|:----------------------------------|:--------------------------------------|
| Relative path | `string` (e.g. `"./my-plugin"`)  | —                                     |
| `github`      | object                           | `repo`, `ref?`, `sha?`               |
| `url`         | object                           | `url` (must end .git), `ref?`, `sha?` |
| `npm`         | object                           | `package`, `version?`, `registry?`    |
| `pip`         | object                           | `package`, `version?`, `registry?`    |

### Strict Mode

| Value            | Behavior                                                       |
|:-----------------|:---------------------------------------------------------------|
| `true` (default) | `plugin.json` is the authority; marketplace supplements it     |
| `false`          | Marketplace entry is the entire definition                     |

---

## Plugin Installation Scopes

| Scope     | Settings file                    | Use case                                  |
|:----------|:---------------------------------|:------------------------------------------|
| `user`    | `~/.claude/settings.json`       | Personal, all projects (default)          |
| `project` | `.claude/settings.json`         | Team plugins, version controlled          |
| `local`   | `.claude/settings.local.json`   | Project-specific, gitignored              |
| `managed` | Managed settings                 | Managed plugins (read-only)               |

---

## CLI Commands

```bash
claude plugin install <plugin> [--scope user|project|local]
claude plugin uninstall <plugin> [--scope user|project|local]  # aliases: remove, rm
claude plugin enable <plugin> [--scope user|project|local]
claude plugin disable <plugin> [--scope user|project|local]
claude plugin update <plugin> [--scope user|project|local|managed]
claude plugin validate .
```

### Development / Testing

```bash
claude --plugin-dir ./my-plugin          # Load plugin for testing
claude --plugin-dir ./plugin-one --plugin-dir ./plugin-two  # Multiple plugins
claude --debug                           # See plugin loading details
```

---

## Debugging

### Common Issues

| Issue                               | Cause                           | Solution                                              |
|:------------------------------------|:--------------------------------|:------------------------------------------------------|
| Plugin not loading                  | Invalid `plugin.json`           | Validate JSON syntax with `claude plugin validate`    |
| Commands not appearing              | Wrong directory structure       | Components at root, not in `.claude-plugin/`          |
| Hooks not firing                    | Script not executable           | `chmod +x script.sh`                                  |
| MCP server fails                    | Missing `${CLAUDE_PLUGIN_ROOT}` | Use variable for all plugin paths                     |
| Path errors                         | Absolute paths used             | All paths relative, start with `./`                   |
| LSP executable not found            | Binary not installed            | Install the language server separately                |

### Plugin Caching

Marketplace plugins are copied to `~/.claude/plugins/cache/` rather than used
in-place. Plugins cannot reference files outside their directory. Use symlinks
for shared files.

### Version Management

Follow semantic versioning: `MAJOR.MINOR.PATCH`

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward-compatible)
- **PATCH**: Bug fixes (backward-compatible)

If code changes but version doesn't bump, users won't see changes due to
caching.
