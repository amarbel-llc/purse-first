---
name: Claude Code Plugin Specification
description: >-
  Use when the user asks about "Claude Code plugin", "plugin.json schema",
  "plugin manifest", "SKILL.md format", "agent frontmatter", "hook events",
  "plugin marketplace", "marketplace.json", "plugin structure", "LSP plugin",
  "plugin hooks", "plugin distribution", or needs to verify plugin conformance,
  understand plugin component schemas, check hook event behavior, or look up
  Claude Code plugin specification details. Also applies when building or
  debugging Claude Code plugins and needing authoritative spec details.
disable-model-invocation: true
version: 0.1.0
---

# Claude Code Plugin Specification

A plugin is a self-contained directory that extends Claude Code with skills,
agents, hooks, MCP servers, LSP servers, and settings. This skill is the
authoritative reference for the plugin format. For the MCP protocol itself, see
**bob:mcp**.

## Plugin Components

| Component   | Location                     | Purpose                              |
|:------------|:-----------------------------|:-------------------------------------|
| Manifest    | `.claude-plugin/plugin.json` | Plugin metadata and configuration    |
| Skills      | `skills/<name>/SKILL.md`     | Agent skills (slash commands)        |
| Agents      | `agents/<name>.md`           | Specialized subagent definitions     |
| Hooks       | `hooks/hooks.json`           | Lifecycle event handlers             |
| MCP servers | `.mcp.json`                  | MCP server configurations            |
| LSP servers | `.lsp.json`                  | Language server configurations       |
| Settings    | `settings.json`              | Default settings (currently `agent`) |

Components live at the plugin root. Only `plugin.json` goes inside
`.claude-plugin/`. The manifest is optional — Claude Code auto-discovers
components in default locations.

## Plugin Manifest (`plugin.json`)

| Field          | Required | Type          | Description                       |
|:---------------|:---------|:--------------|:----------------------------------|
| `name`         | Yes      | string        | Unique identifier (kebab-case)    |
| `version`      | No       | string        | Semantic version                  |
| `description`  | No       | string        | Brief explanation                 |
| `author`       | No       | object        | `name`, `email`, `url`            |
| `homepage`     | No       | string        | Documentation URL                 |
| `repository`   | No       | string        | Source code URL                   |
| `license`      | No       | string        | SPDX identifier                   |
| `keywords`     | No       | array         | Discovery tags                    |
| `commands`     | No       | string\|array | Additional command paths          |
| `agents`       | No       | string\|array | Additional agent paths            |
| `skills`       | No       | string\|array | Additional skill paths            |
| `hooks`        | No       | string\|object| Hook config paths or inline       |
| `mcpServers`   | No       | string\|object| MCP config paths or inline        |
| `lspServers`   | No       | string\|object| LSP config paths or inline        |
| `outputStyles` | No       | string\|array | Output style paths                |

Custom paths supplement defaults — they don't replace them. All paths relative
to plugin root, starting with `./`. Use `${CLAUDE_PLUGIN_ROOT}` in hooks and
MCP/LSP configs for absolute resolution.

## SKILL.md Frontmatter

| Field                      | Required    | Description                                          |
|:---------------------------|:------------|:-----------------------------------------------------|
| `name`                     | No          | Display name (lowercase, hyphens, max 64 chars)      |
| `description`              | Recommended | When to use (drives auto-invocation)                 |
| `argument-hint`            | No          | Autocomplete hint (e.g. `[issue-number]`)            |
| `disable-model-invocation` | No          | `true` = user-invocable only. Default: `false`       |
| `user-invocable`           | No          | `false` = hidden from `/` menu. Default: `true`      |
| `allowed-tools`            | No          | Tools allowed without permission prompts             |
| `model`                    | No          | Model override when skill is active                  |
| `context`                  | No          | `fork` to run in a subagent context                  |
| `agent`                    | No          | Subagent type when `context: fork`                   |
| `hooks`                    | No          | Hooks scoped to skill lifecycle                      |

## Agent Frontmatter

| Field             | Required | Description                                     |
|:------------------|:---------|:------------------------------------------------|
| `name`            | Yes      | Unique identifier (lowercase, hyphens)          |
| `description`     | Yes      | When Claude should delegate here                |
| `tools`           | No       | Allowed tools (inherits all if omitted)         |
| `disallowedTools` | No       | Tools to deny                                   |
| `model`           | No       | `sonnet`, `opus`, `haiku`, `inherit`            |
| `permissionMode`  | No       | `default`, `acceptEdits`, `dontAsk`, etc.       |
| `maxTurns`        | No       | Max agentic turns                               |
| `skills`          | No       | Skills preloaded at startup                     |
| `hooks`           | No       | Scoped lifecycle hooks                          |
| `memory`          | No       | `user`, `project`, or `local`                   |
| `background`      | No       | `true` = always background. Default: `false`    |
| `isolation`       | No       | `worktree` for git worktree isolation           |

## Hook Events

| Event              | Fires when                        | Can block? |
|:-------------------|:----------------------------------|:-----------|
| `SessionStart`     | Session begins or resumes         | No         |
| `UserPromptSubmit` | User submits prompt               | Yes        |
| `PreToolUse`       | Before tool call executes         | Yes        |
| `PermissionRequest`| Permission dialog appears         | Yes        |
| `PostToolUse`      | After tool call succeeds          | No         |
| `PostToolUseFailure`| After tool call fails            | No         |
| `Notification`     | Notification sent                 | No         |
| `SubagentStart`    | Subagent spawned                  | No         |
| `SubagentStop`     | Subagent finishes                 | Yes        |
| `Stop`             | Claude finishes responding        | Yes        |
| `TeammateIdle`     | Teammate about to go idle         | Yes        |
| `TaskCompleted`    | Task marked completed             | Yes        |
| `ConfigChange`     | Config file changes               | Yes        |
| `WorktreeCreate`   | Worktree being created            | Yes        |
| `WorktreeRemove`   | Worktree being removed            | No         |
| `PreCompact`       | Before context compaction         | No         |
| `SessionEnd`       | Session terminates                | No         |

Hook handler types: `command`, `prompt`, `agent`. Exit 0 = success, exit 2 =
blocking error, other = non-blocking error.

## Marketplace Schema

| Field              | Required | Type   | Description                      |
|:-------------------|:---------|:-------|:---------------------------------|
| `name`             | Yes      | string | Marketplace identifier           |
| `owner.name`       | Yes      | string | Maintainer name                  |
| `owner.email`      | No       | string | Maintainer email                 |
| `plugins`          | Yes      | array  | Available plugins                |
| `metadata.version` | No       | string | Marketplace version              |

Plugin source types: relative path (`"./path"`), `github` (`repo`, `ref?`,
`sha?`), `url` (`.git` URL), `npm` (`package`, `version?`, `registry?`), `pip`
(`package`, `version?`, `registry?`).

## Finding What You Need

For full details on any topic, consult the reference document:

| Topic                              | Section in reference                          |
|:-----------------------------------|:----------------------------------------------|
| Plugin directory layout            | `references/claude-plugin-specification.md` § Plugin Structure Overview |
| Complete manifest schema           | § Plugin Manifest Schema                      |
| SKILL.md format & substitutions    | § Skills                                      |
| Agent file format & built-ins      | § Agents (Subagents)                          |
| Hook config, matchers, I/O schemas | § Hooks                                       |
| MCP server configuration           | § MCP Servers                                 |
| LSP server configuration           | § LSP Servers                                 |
| Marketplace distribution           | § Plugin Marketplaces                         |
| Installation scopes                | § Plugin Installation Scopes                  |
| CLI commands                       | § CLI Commands                                |
| Debugging common issues            | § Debugging                                   |

## Related Skills

- **bob:mcp** — MCP protocol specification (versions, capabilities, transports)
- **bob:creating-packages** — Packaging MCP servers and skills for purse-first
- **bob:using-packages** — How installed packages work at runtime
