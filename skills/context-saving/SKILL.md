---
name: MCP Context-Saving
description: This skill should be used when the user asks to "add context-saving", "add pagination", "add truncation", "limit output size", "add offset and limit", "reduce token usage", "add head and tail parameters", "limit results", or mentions unbounded output, large responses, PaginationInfo, TruncationInfo, output size management, or token waste in MCP server tools. Also applies when an MCP tool's output is too long, agent token usage is too high, or MCP responses need size limits.
version: 0.1.0
---

# MCP Context-Saving

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

MCP tools that return unbounded output waste context window tokens and degrade agent performance. Context-saving adds output-limiting parameters so callers can request only the data they need. Every MCP tool that can produce large output should implement one of two patterns.

For Go implementations of these patterns, see the **bob:go-cli-framework** skill (Context-Saving in Go section).
For packaging your MCP server after adding context-saving, see the **bob:creating-packages** skill.

## When to Apply

Audit each tool by asking: "Can this tool's output grow without bound?" If yes, apply context-saving. Common indicators:

- **Array results** (directory listings, search results, diagnostics, completions) -- use Pagination
- **Text/JSON blob results** (build logs, flake metadata, eval output, file contents) -- use Truncation
- **Naturally bounded results** (hash computation, single status check, definition lookup) -- skip

## Pattern 1: Pagination (Array Results)

For tools returning lists or arrays of items. Add `offset` and `limit` parameters plus `PaginationInfo` metadata.

### Parameters to Add

| Parameter | Type | Description |
|-----------|------|-------------|
| `offset` | `Option<usize>` | Skip first N items. Defaults to 0. |
| `limit` | `Option<usize>` | Maximum items to return. Defaults to all. |

### Result Metadata

Include `PaginationInfo` in the response (skip serialization when `None`):

```text
PaginationInfo { offset, limit, total, has_more }
```

- `total`: count of all items before pagination
- `has_more`: whether `offset + kept_count < total`
- Only populate when caller specifies `offset` or `limit`; omit otherwise to avoid noise

### Implementation Steps

1. Collect all items into a `Vec`
2. Record `total = items.len()`
3. Apply `items.into_iter().skip(offset).take(limit).collect()`
4. Compute `has_more = offset + kept_count < total`
5. Return `PaginationInfo` only when `offset.is_some() || limit.is_some()`

### Where to Apply

- Directory listings (store_ls, file browsers)
- Search results (package search, FlakeHub search)
- Diagnostic arrays (LSP diagnostics, completions)
- Closure entries (dependency closures)
- Any API returning JSON arrays

## Pattern 2: Truncation (Text/JSON Blob Results)

For tools returning text or serialized JSON. Add `head`, `tail`, and `max_bytes` parameters plus `TruncationInfo` metadata.

### Parameters to Add

| Parameter | Type | Description |
|-----------|------|-------------|
| `head` | `Option<usize>` | Return only the first N lines. |
| `tail` | `Option<usize>` | Return only the last N lines. |
| `max_bytes` | `Option<usize>` | Maximum byte size of output. |

`head` and `tail` are mutually exclusive; `head` takes priority. `max_bytes` is applied after line-based limiting and truncates at line boundaries when possible.

### Result Metadata

Include `TruncationInfo` when truncation occurs:

```text
TruncationInfo { original_bytes, original_lines, kept_bytes, kept_lines, position }
```

- `position`: `"head"`, `"tail"`, or `None`
- Only populate when content was actually truncated

### Implementation Steps

1. Capture the raw output as a string
2. Apply `limit_text_output(raw, &limits)` (or equivalent)
3. If truncated, parse what remains (may produce a string fallback for truncated JSON)
4. Return `truncated: Some(true)` and `truncation_info` only when truncation occurred

### JSON Output Consideration

When truncating JSON output (eval, flake_show, flake_metadata), truncation may break JSON validity. Handle this by attempting to parse the truncated content and falling back to a raw string value:

```rust
let value = serde_json::from_str(&limited.content)
    .unwrap_or(serde_json::Value::String(limited.content));
```

This preserves structured JSON when no truncation occurs and degrades gracefully to a string when it does.

### Where to Apply

- Build logs
- Eval output (arbitrary Nix expressions)
- Flake show/metadata (JSON objects)
- Check output (stdout/stderr)
- Any tool producing text blobs

## Pattern 3: Stderr Truncation (Command Output)

Every tool that executes an external command captures stderr. This stderr is
**never caller-controllable** — unlike stdout, there are no parameters the
caller can set to limit it. Commands like `nix search --json` emit per-package
evaluation progress on stderr (~8.5M characters for nixpkgs), inflating tool
responses far beyond token limits.

Stderr requires **automatic, default-on truncation** via a convenience function.
This is fundamentally different from Patterns 1 and 2, which are opt-in via
caller parameters.

### When to Apply

**Every tool that shells out to an external command.** This includes tools whose
primary output is "naturally bounded" (hash computation, status checks, single
results). The primary output may be small, but stderr is always unbounded.

### Implementation

Use the `LimitStderr` convenience function from the `output` package:

```go
result, err := exec.RunCommand(ctx, args...)
limited := output.LimitStderr(result.Stderr)

return &ToolResult{
    Output:         result.Stdout,
    Stderr:         limited.Content,
    Truncated:      limited.Truncated,
    TruncationInfo: limited.TruncationInfo,
}, nil
```

For Rust, use `limit_stderr()` from the `output` module (same 100KB default).

### Important: Inspect Before Truncating

When a tool needs to inspect stderr before truncation (e.g., checking for
authentication status or specific error patterns), read it first:

```go
isAuthed := strings.Contains(result.Stderr, "authenticated")
limited := output.LimitStderr(result.Stderr)
```

### Combined Truncation

When both stdout and stderr are independently truncated, combine the signals:

```go
limitedStdout := output.LimitText(result.Stdout, limits)
limitedStderr := output.LimitStderr(result.Stderr)
truncated := limitedStdout.Truncated || limitedStderr.Truncated
```

## Decision Checklist

For each tool, determine context-saving applicability:

| Output Type | Primary Output | Stderr | Example Tools |
|-------------|---------------|--------|---------------|
| `Vec<T>` or JSON array | Pagination | Truncate | store_ls, search, diagnostics, completions, list APIs |
| Text or JSON object/blob | Truncation | Truncate | build logs, eval, flake show/metadata, check output |
| Single scalar value | None needed | Truncate | hash, status, resolve |
| User-initiated output | None needed | Truncate | run, develop_run (user controls the command) |

**Note:** The "Stderr" column applies to every tool that executes an external
command. Even tools with naturally bounded primary output can produce unbounded
stderr.

## Implementation Checklist

When adding context-saving to a tool:

1. Add parameters to the params struct (offset/limit or head/tail/max_bytes)
2. Add parameters to the tool schema in list_tools()
3. Add metadata field to the result struct (PaginationInfo or TruncationInfo) with `skip_serializing_if`
4. Implement the limiting logic in the tool function
5. Update server dispatch if the function signature changed
6. Build and run tests to verify
7. Apply `LimitStderr()` to stderr from any external command before including it in the result

## Reference Files

For detailed implementation examples from nix-mcp (a Rust MCP server), consult:
- **`references/implementation-patterns.md`** -- Side-by-side before/after code examples, schema updates, and a complete tool audit showing which tools got which pattern and why
- **`examples/pagination.rs`** -- Minimal standalone pagination implementation
- **`examples/truncation.rs`** -- Minimal standalone truncation implementation with JSON fallback

## Related Skills

- **bob:go-cli-framework** — Go implementations of pagination and truncation via the `output` package
- **bob:creating-packages** — Packaging your MCP server for distribution via purse-first
- **bob:overview** — Framework orientation and concept definitions
