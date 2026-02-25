# mgp (model graph protocol) Design

## Problem

N MCP servers each dump their full tool lists into Claude's context window.
With 50+ tools across packages, this wastes significant context on tool
descriptions that may never be used in a given session.

## Solution

A new MCP package called **mgp** that exposes the entire purse-first tool
ecosystem through two tools: `query` and `exec`. Claude sees 2 tools instead
of 50+ and discovers what it needs on demand via GraphQL.

## Goals

- **Intelligent discovery** — GraphQL lets Claude query tool metadata, filter
  by annotations, search by name, discover what's available
- **Single entry point** — one MCP server replaces N separate ones; Claude
  talks to one server which routes exec calls to the right underlying MCP
- **Minimize context usage** — 2 tool descriptions instead of 50+

## Architecture

```
                   +---------------------------+
                   |  Claude Code (client)     |
                   |  sees 2 tools: query, exec|
                   +-----------+---------------+
                               | MCP (stdio)
                   +-----------v---------------+
                   |  mgp MCP server           |
                   |                           |
                   |  +-------+   +------+     |
                   |  | query |   | exec |     |
                   |  +---+---+   +--+---+     |
                   |      |          |          |
                   |  +---v---+      |          |
                   |  |GraphQL|      |          |
                   |  |engine |      |          |
                   |  +---+---+      |          |
                   |      |          |          |
                   |  +---v----------v------+   |
                   |  |    Tool Catalog      |   |
                   |  | (populated at init)  |   |
                   |  +---------------------+   |
                   +------------+---------------+
                                | spawns on exec
              +-----------------+------------------+
              |                 |                   |
        +-----v-----+   +------v------+   +--------v----+
        |   grit    |   | get-hubbed  |   |    chix     |
        |  (stdio)  |   |   (stdio)   |   |   (stdio)   |
        +-----------+   +-------------+   +-------------+
```

### Startup Sequence

1. mgp starts
2. Discovers installed packages via marketplace discovery (reads plugin.json
   files from Nix store paths)
3. For each MCP package found (excluding itself), spawns the binary, calls
   `initialize` + `tools/list`, captures tool definitions, shuts it down
4. Builds GraphQL schema from collected tool metadata using `graphql-go/graphql`
5. Serves MCP over stdio with two tools: `query` and `exec`

### Runtime

- `query` — evaluates GraphQL against the in-memory tool catalog
- `exec` — spawns the target MCP server, calls `initialize` + `tools/call`,
  returns result, kills process

## GraphQL Schema

Flat tool list with filters. Supports standard introspection (`__schema`,
`__type`) via the graphql-go library.

```graphql
type Tool {
  name: String!
  title: String
  description: String
  package: String!
  inputSchema: String      # JSON Schema as raw JSON string
  readOnly: Boolean
  destructive: Boolean
  idempotent: Boolean
  openWorld: Boolean
}

type Query {
  tools(
    package: String         # filter by package name
    name: String            # filter by tool name (exact or substring)
    readOnly: Boolean       # filter by annotation
    destructive: Boolean
  ): [Tool!]!
}
```

### Example Queries

```graphql
# "What git tools are available?"
{ tools(package: "grit") { name description } }

# "Find all read-only tools"
{ tools(readOnly: true) { name package description } }

# "What does the 'log' tool expect?"
{ tools(name: "log") { name package inputSchema } }

# "List all tools with just names and packages"
{ tools { name package } }
```

## MCP Tools

### query

```json
{
  "name": "query",
  "description": "Query the purse-first tool catalog using GraphQL",
  "inputSchema": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "GraphQL query string"
      }
    },
    "required": ["query"]
  }
}
```

Returns GraphQL result as JSON in the tool response content.

### exec

```json
{
  "name": "exec",
  "description": "Execute a tool on an MCP server",
  "inputSchema": {
    "type": "object",
    "properties": {
      "server": {
        "type": "string",
        "description": "MCP server name (e.g. grit, get-hubbed, chix)"
      },
      "tool": {
        "type": "string",
        "description": "Tool name to call"
      },
      "args": {
        "type": "object",
        "description": "Arguments to pass to the tool"
      }
    },
    "required": ["server", "tool"]
  }
}
```

Exec flow:

1. Look up server command from the catalog
2. Spawn the MCP binary over stdio
3. Send `initialize` request, wait for response
4. Send `tools/call` with tool name and args
5. Return the tool result content
6. Kill the process

## Package Structure

```
packages/mgp/
  cmd/mgp/main.go                    # Entrypoint
  internal/tools/registry.go         # query + exec tool registration
  internal/tools/query.go            # GraphQL query handler
  internal/tools/exec.go             # MCP proxy handler
  internal/catalog/catalog.go        # Tool catalog (in-memory store)
  internal/catalog/introspect.go     # Startup MCP introspection
  internal/mcpclient/client.go       # Minimal MCP JSON-RPC client over stdio
  go.mod
lib/packages/mgp.nix                 # Nix build expression
```

### Dependencies

- `github.com/graphql-go/graphql` — GraphQL engine
- `libs/go-mcp` — command framework + MCP protocol types
- Added to `go.work`

### Marketplace Integration

- Entry in `marketplace-config.json`
- Nix build produces `share/purse-first/mgp/plugin.json`
- Self-excludes from its own catalog to prevent recursion

## Error Handling

- **Startup introspection failures:** Log warning, exclude the failing server
  from catalog. Don't fail the whole server. 10-second timeout per server.
- **Invalid GraphQL:** Return GraphQL error in tool response (not MCP error)
- **Valid query, no results:** Return empty array
- **Unknown server/tool in exec:** MCP error response
- **Server fails to spawn:** MCP error response with stderr context
- **Downstream tool error:** Forward error as-is from the downstream server

## Decisions

- **Approach 1 (static marketplace) + startup introspection** — plugin.json
  doesn't contain per-tool metadata, so we introspect via `tools/list` at
  init. One-time cost.
- **graphql-go/graphql** — runtime schema construction, no codegen. Good for
  prototyping.
- **Spawn per exec call** — no persistent connection pool for v1.
- **Flat tool list** — no package-as-node graph structure for v1. Tools are
  the primary queryable entity with package as a field/filter.
