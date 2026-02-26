# mgp GraphQL Mux Design

## Problem

mgp currently discovers tools exclusively through plugin.json manifests and MCP
introspection. We want mgp to also support forwarding requests to a local
GraphQL server over stdio, enabling tool discovery and query forwarding through
an external GraphQL service with a paired MCP server for execution.

## Design

### Overview

A new `--graphql-server <command>` flag tells mgp to spawn a GraphQL server as a
child process. mgp communicates with it over stdio using newline-delimited JSON
(standard GraphQL request/response format). At startup, mgp introspects the
remote schema to discover available tools. The remote server's tools supplement
the existing plugin.json-discovered tools in the catalog.

When `--graphql-server` is provided:

- The `query` tool forwards all GraphQL queries to the remote server
- The `exec` tool routes tool calls to the paired MCP server (same binary,
  default mode) for GraphQL-sourced tools, and to plugin-discovered servers for
  plugin-sourced tools

### Components

#### 1. GraphQL Client (`internal/graphqlclient/`)

New package that manages a spawned GraphQL server subprocess.

```go
type Client struct {
    cmd    *exec.Cmd
    stdin  io.Writer      // child's stdin (we write to it)
    stdout *bufio.Scanner // child's stdout (we read from it)
}

func Spawn(ctx context.Context, command string, args ...string) (*Client, error)
func (c *Client) Query(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error)
func (c *Client) Close() error
```

Wire protocol: write one JSON line per request, read one JSON line per response.

Request format:

```json
{"query": "{ tools { name } }", "variables": {}}
```

Response format: standard GraphQL response (data + errors).

#### 2. Catalog Source Tracking

`ServerEntry` gains a `Source` field to distinguish tool origins:

```go
type ServerSource int

const (
    SourcePlugin  ServerSource = iota // discovered via plugin.json
    SourceGraphQL                      // discovered via GraphQL server
)

type ServerEntry struct {
    Name    string
    Command string
    Args    []string
    Source  ServerSource
}
```

Tools from the GraphQL server create `ServerEntry` records with
`Source: SourceGraphQL` and `Command` set to the `--graphql-server` binary (which
serves MCP in its default mode).

#### 3. Discovery Flow (GraphQL Source)

At startup, when `--graphql-server` is provided:

1. Spawn the GraphQL server subprocess
2. Send GraphQL introspection query (`{ __schema { ... } }`) to learn the schema
3. Query for tools (adapting to discovered schema)
4. For each unique package in the results, create a `ServerEntry` with
   `Source: SourceGraphQL`
5. Add tools to the catalog

This runs alongside existing plugin.json discovery. Both sources contribute
tools.

#### 4. Query Tool Changes

When `--graphql-server` is configured, the `query` tool forwards all GraphQL
queries directly to the remote server and returns its response. The local
GraphQL schema (built from the catalog) is still constructed for the catalog
resource but is bypassed by the query tool.

When no `--graphql-server` is configured, the query tool works as before against
the local schema.

#### 5. Exec Tool Changes

The exec tool's routing logic:

1. Look up `ServerEntry` by server name (unchanged)
2. Spawn the entry's command as an MCP subprocess (unchanged)
3. Initialize, call tool, return result, kill process (unchanged)

Since `SourceGraphQL` entries already have the correct command (the
`--graphql-server` binary in its default MCP mode), no branching is needed in
the exec path itself. The differentiation happens at discovery time, not
execution time.

#### 6. Lifecycle

- **GraphQL server:** Spawned once at startup, kept alive for mgp's lifetime,
  closed on shutdown
- **MCP servers (exec):** Spawned per call, killed after result (unchanged)

### Error Handling

| Scenario                              | Behavior                          |
| ------------------------------------- | --------------------------------- |
| `--graphql-server` command not found  | Fatal at startup                  |
| Introspection fails                   | Fatal at startup                  |
| GraphQL server dies mid-session       | `query` returns error             |
| Paired MCP server fails to spawn      | `exec` returns error (unchanged)  |

### CLI Changes

```
mgp [flags]
  --plugins-dir    path to share/purse-first/ directory
  --bin-dir        path to bin/ directory
  --graphql-server command to spawn as GraphQL server (newline-delimited JSON over stdio)
```

### File Changes

| File                                  | Change                                     |
| ------------------------------------- | ------------------------------------------ |
| `internal/graphqlclient/client.go`    | New: GraphQL stdio client                  |
| `internal/catalog/catalog.go`         | Add `ServerSource` type and `Source` field  |
| `internal/catalog/discover.go`        | Add `DiscoverGraphQL()` function           |
| `internal/tools/query.go`             | Forward queries when GraphQL client exists |
| `cmd/mgp/main.go`                     | New `--graphql-server` flag, startup logic |

### Non-Goals

- Schema merging (remote + local into unified schema)
- Connection pooling for MCP servers
- Multiple GraphQL server sources
- Auto-restart of crashed GraphQL server
