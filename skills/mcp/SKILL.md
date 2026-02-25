---
name: MCP Specification Reference
description: >-
  Use when the user asks about "MCP protocol version", "MCP spec", "protocol
  negotiation", "MCP capabilities", "MCP transport", "MCP lifecycle", "MCP
  schema", "initialize handshake", or needs to verify MCP server conformance,
  understand version differences, check required methods, or look up MCP
  protocol behavior. Also applies when building or debugging MCP servers and
  needing authoritative spec details.
disable-model-invocation: true
---

# MCP Specification Reference

## Overview

The Model Context Protocol (MCP) is an open protocol for connecting AI
applications to external tools, data sources, and services. This skill provides
quick access to the official spec across all released versions and a guide to
navigating the reference documents.

## Spec Versions

| Version | Status | Key Changes |
|---------|--------|-------------|
| `2024-11-05` | Legacy | Original spec: tools, resources, prompts, sampling, roots, stdio + HTTP/SSE transports |
| `2025-03-26` | Stable | OAuth 2.1 authorization, streamable HTTP transport (replaces SSE), JSON-RPC batching, tool annotations, audio content, completions capability |
| `2025-06-18` | Released | Elicitation, structured tool output, resource links, security best practices, `MCP-Protocol-Version` header, removed JSON-RPC batching |
| `2025-11-25` | Latest | Tasks (experimental), URL-mode elicitation, tool calling in sampling, icons on tools/resources/prompts, OAuth enhancements, JSON Schema 2020-12 default |
| `draft` | Draft | Working draft for next release |

For the official versioning policy, see `references/versioning.mdx`.

## Capability Matrix

Capabilities declared during the initialize handshake. Rows show when each was
introduced:

| Capability | 2024-11-05 | 2025-03-26 | 2025-06-18 | 2025-11-25 |
|------------|:---:|:---:|:---:|:---:|
| **Server: tools** | Yes | Yes | Yes | Yes |
| **Server: resources** | Yes | Yes | Yes | Yes |
| **Server: prompts** | Yes | Yes | Yes | Yes |
| **Server: logging** | Yes | Yes | Yes | Yes |
| **Server: completions** | — | Yes | Yes | Yes |
| **Client: roots** | Yes | Yes | Yes | Yes |
| **Client: sampling** | Yes | Yes | Yes | Yes |
| **Client: elicitation** | — | — | Yes | Yes |

## Transport Matrix

| Transport | 2024-11-05 | 2025-03-26 | 2025-06-18 | 2025-11-25 |
|-----------|:---:|:---:|:---:|:---:|
| **stdio** (NDJSON) | Yes | Yes | Yes | Yes |
| **HTTP + SSE** (legacy) | Yes | Replaced | — | — |
| **Streamable HTTP** | — | Yes | Yes | Yes |

Streamable HTTP uses POST for client-to-server, GET for SSE streams, DELETE for
session teardown, with `Mcp-Session-Id` header for session management.

Starting in `2025-06-18`, the negotiated protocol version must be sent via the
`MCP-Protocol-Version` header on subsequent HTTP requests.

## Version Negotiation

The client and server agree on a protocol version during the `initialize`
handshake:

1. Client sends `initialize` request with `protocolVersion` set to the latest
   version it supports.
2. Server responds with `protocolVersion` set to the version it will use, which
   **must** be one it supports and **should** be the latest version both sides
   share.
3. If the server does not support the client's requested version, it **should**
   respond with the latest version it does support. The client may then decide
   whether to continue or disconnect.
4. Client sends `initialized` notification to confirm.

The server **must not** send requests or notifications (other than `ping`) until
after the client sends `initialized`.

## Key Protocol Concepts

**Base protocol:** All messages use JSON-RPC 2.0 with three message types:
requests (require response), responses, and notifications (fire-and-forget).

**Lifecycle:** Initialize → operate → shutdown. During operation, both sides may
send requests/notifications according to negotiated capabilities.

**Capabilities:** Declared in the `initialize` response. A capability's presence
means the server/client supports that feature. Absent capabilities must not be
used.

**Server features:** Tools (model-invoked functions), resources (data exposed to
the model), and prompts (templates for common interactions).

**Client features:** Roots (filesystem entry points), sampling (LLM access for
servers), and elicitation (user input requests from servers).

**Authorization:** Starting in `2025-03-26`, OAuth 2.1 with PKCE for HTTP
transport. Servers act as OAuth Resource Servers. See
`references/<version>/basic/authorization.mdx`.

## Finding What You Need

Each version directory mirrors the official spec structure:

| What you need | File path |
|--------------|-----------|
| Spec overview | `references/<version>/index.mdx` |
| Architecture (client-host-server) | `references/<version>/architecture/index.mdx` |
| JSON-RPC base protocol | `references/<version>/basic/index.mdx` |
| Lifecycle & initialization | `references/<version>/basic/lifecycle.mdx` |
| Transports (stdio, HTTP) | `references/<version>/basic/transports.mdx` |
| Authorization (OAuth 2.1) | `references/<version>/basic/authorization.mdx` (2025-03-26+) |
| Security best practices | `references/<version>/basic/security_best_practices.mdx` (2025-06-18+) |
| Tools | `references/<version>/server/tools.mdx` |
| Resources | `references/<version>/server/resources.mdx` |
| Prompts | `references/<version>/server/prompts.mdx` |
| Logging | `references/<version>/server/utilities/logging.mdx` |
| Completion | `references/<version>/server/utilities/completion.mdx` |
| Pagination | `references/<version>/server/utilities/pagination.mdx` |
| Cancellation | `references/<version>/basic/utilities/cancellation.mdx` |
| Progress | `references/<version>/basic/utilities/progress.mdx` |
| Ping | `references/<version>/basic/utilities/ping.mdx` |
| Tasks (experimental) | `references/<version>/basic/utilities/tasks.mdx` (2025-11-25+) |
| Roots | `references/<version>/client/roots.mdx` |
| Sampling | `references/<version>/client/sampling.mdx` |
| Elicitation | `references/<version>/client/elicitation.mdx` (2025-06-18+) |
| Changes from prior version | `references/<version>/changelog.mdx` (2025-03-26+) |
| JSON Schema | `references/<version>/schema.json` |
| TypeScript types | `references/<version>/schema.ts` |
