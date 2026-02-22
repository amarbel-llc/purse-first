# gh api Tools Design

## Motivation

The existing get-hubbed MCP tools (repo, issue, PR) cover common GitHub
operations but can't reach the full API surface. Adding `gh api` support
provides a generic escape hatch for arbitrary REST endpoints and GraphQL
queries, while maintaining a read/write safety boundary.

## Approach

Three new MCP tools with clear read/write separation:

1. **`api_get`** — REST GET requests (read-only)
2. **`graphql_query`** — GraphQL queries (read-only)
3. **`graphql_mutation`** — GraphQL mutations (writes)

This lets MCP clients permission reads and writes independently.

## Tool Schemas

### api_get

Make an authenticated GET request to the GitHub REST API.

| Parameter  | Type    | Required | Description                                  |
|------------|---------|----------|----------------------------------------------|
| endpoint   | string  | yes      | REST API path, e.g. `/repos/{owner}/{repo}/actions/runs` |
| params     | object  | no       | Query string parameters as key-value pairs   |
| headers    | array   | no       | Additional headers in `key:value` format     |
| paginate   | boolean | no       | Auto-paginate results                        |

Maps to: `gh api <endpoint> --method GET [-f key=value]... [-H header]... [--paginate]`

### graphql_query

Execute a read-only GraphQL query against the GitHub API.

| Parameter  | Type    | Required | Description                                  |
|------------|---------|----------|----------------------------------------------|
| query      | string  | yes      | GraphQL query string                         |
| variables  | object  | no       | GraphQL variables as key-value pairs         |
| paginate   | boolean | no       | Auto-paginate (requires endCursor/pageInfo)  |

Maps to: `gh api graphql -f query='...' [-F key=value]... [--paginate]`

### graphql_mutation

Execute a GraphQL mutation against the GitHub API.

| Parameter  | Type    | Required | Description                                  |
|------------|---------|----------|----------------------------------------------|
| query      | string  | yes      | GraphQL mutation string                      |
| variables  | object  | no       | GraphQL variables as key-value pairs         |

Maps to: `gh api graphql -f query='...' [-F key=value]...`

No `--paginate` support (mutations don't paginate).

## Implementation

- New file: `internal/tools/api.go`
- Functions: `registerAPITools`, `handleAPIGet`, `handleGraphQLQuery`, `handleGraphQLMutation`
- Hook into `RegisterAll()` in `registry.go`
- Update `plugin.json` manifest with new tool definitions

### Parameter Mapping

- `api_get` params object entries become `-f key=value` (query string for GET)
- GraphQL variables become `-F key=value` (typed coercion for booleans/numbers)
- Headers become `-H key:value`
