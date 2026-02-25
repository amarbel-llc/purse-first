# bob:mcp Skill Design

## Summary

Create a new `bob:mcp` skill that serves as an MCP protocol specification
reference and conformance guide. The skill is purse-first agnostic — it covers
the official MCP spec, not purse-first-specific implementation details.

## Purpose

1. **Spec conformance guide** — help developers building MCP servers ensure
   their implementations conform to the official MCP spec (version negotiation,
   required methods, capabilities, transport requirements).
2. **Spec reference for agents** — give Claude agents quick access to official
   MCP spec details when answering questions about MCP protocol behavior,
   available methods, capability negotiation, schema types.

## Approach

Lean SKILL.md (~1,200–1,500 words) with heavy references. The SKILL.md provides
version comparison tables, capability matrices, transport summaries, and version
negotiation rules. All actual spec content lives in `references/<version>/`
mirroring the upstream repo layout.

## Directory Layout

```
skills/mcp/
├── SKILL.md
└── references/
    ├── versioning.mdx
    ├── 2024-11-05/        # Legacy
    │   ├── schema.json
    │   ├── schema.ts
    │   ├── index.mdx
    │   ├── architecture/
    │   ├── basic/
    │   ├── client/
    │   └── server/
    ├── 2025-03-26/        # Stable
    │   └── (same + changelog.mdx, authorization.mdx)
    ├── 2025-06-18/        # Latest
    │   └── (+ elicitation.mdx, security_best_practices.mdx)
    ├── 2025-11-25/        # Latest
    │   └── (+ tasks.mdx, expanded auth/sampling/elicitation)
    └── draft/             # Next release working draft
```

## SKILL.md Sections

1. **Overview** — What MCP is, what this skill provides (~2 sentences)
2. **Spec Versions** — Table: version, status, date, one-line delta
3. **Version Comparison: Capabilities** — Matrix of capabilities × versions
4. **Version Comparison: Transports** — Transports × versions
5. **Version Negotiation** — How initialize handshake works
6. **Key Protocol Concepts** — Brief pointers: lifecycle, JSON-RPC 2.0,
   capabilities, methods
7. **Finding What You Need** — Navigation guide to reference files

## Registration

Add `"./skills/mcp"` to `.claude-plugin/plugin.json` skills array.

## Non-Goals

- No purse-first-specific implementation guidance (that's `bob:go-cli-framework`
  and `bob:creating-packages`)
- No code examples (those are in the spec docs)
- No opinion on which version to target (the skill presents facts, developers
  decide)
