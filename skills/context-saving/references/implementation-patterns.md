# Context-Saving Implementation Patterns

Detailed code examples from a Rust MCP server (nix-mcp) showing both pagination and truncation patterns applied across different tool categories.

## Pagination Pattern: Full Example (store_ls)

### Before (no context-saving)

```rust
#[derive(Debug, Serialize)]
pub struct NixStoreLsResult {
    pub path: String,
    pub entries: Vec<NixStoreLsEntry>,
}

pub async fn nix_store_ls(params: NixStoreLsParams) -> Result<NixStoreLsResult, String> {
    // ... collect entries ...
    entries.sort_by(|a, b| a.name.cmp(&b.name));

    Ok(NixStoreLsResult {
        path: canonical.to_string_lossy().to_string(),
        entries,
    })
}
```

### After (with pagination)

```rust
#[derive(Debug, Serialize)]
pub struct NixStoreLsResult {
    pub path: String,
    pub entries: Vec<NixStoreLsEntry>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pagination: Option<PaginationInfo>,
}

pub async fn nix_store_ls(params: NixStoreLsParams) -> Result<NixStoreLsResult, String> {
    // ... collect entries ...
    entries.sort_by(|a, b| a.name.cmp(&b.name));

    let total = entries.len();
    let offset = params.offset.unwrap_or(0);
    let limit = params.limit.unwrap_or(total);

    let paginated: Vec<NixStoreLsEntry> = entries.into_iter().skip(offset).take(limit).collect();
    let kept_count = paginated.len();
    let has_more = offset + kept_count < total;

    let pagination = if params.offset.is_some() || params.limit.is_some() {
        Some(PaginationInfo {
            offset,
            limit,
            total,
            has_more,
        })
    } else {
        None
    };

    Ok(NixStoreLsResult {
        path: canonical.to_string_lossy().to_string(),
        entries: paginated,
        pagination,
    })
}
```

### Key details

- `PaginationInfo` is only included when the caller specifies offset or limit
- `skip_serializing_if = "Option::is_none"` keeps the response clean for default calls
- Pagination is applied after sorting to ensure stable results across pages
- `has_more` tells the caller whether another page exists

---

## Pagination Pattern: JSON Arrays from External Commands (FlakeHub)

When the data comes from an external CLI that returns JSON arrays, parse first, then paginate client-side.

```rust
fn paginate_json_array(
    value: serde_json::Value,
    offset: Option<usize>,
    limit: Option<usize>,
) -> (serde_json::Value, Option<PaginationInfo>) {
    if let serde_json::Value::Array(arr) = value {
        let total = arr.len();
        let off = offset.unwrap_or(0);
        let lim = limit.unwrap_or(total);

        let paginated: Vec<serde_json::Value> = arr.into_iter().skip(off).take(lim).collect();
        let kept_count = paginated.len();
        let has_more = off + kept_count < total;

        let pagination = if offset.is_some() || limit.is_some() {
            Some(PaginationInfo {
                offset: off,
                limit: lim,
                total,
                has_more,
            })
        } else {
            None
        };

        (serde_json::Value::Array(paginated), pagination)
    } else {
        (value, None)
    }
}
```

This helper gracefully handles non-array values by passing them through unchanged. Useful when the CLI might return different shapes on error.

---

## Pagination Pattern: LSP Results with Changed Signatures

When adding pagination to functions that take individual parameters (not a params struct), extend the signature:

```rust
// Before
pub async fn nil_diagnostics(file_path: String) -> Result<DiagnosticsResult, String> { ... }

// After
pub async fn nil_diagnostics(
    file_path: String,
    offset: Option<usize>,
    limit: Option<usize>,
) -> Result<DiagnosticsResult, String> { ... }
```

Update all call sites (server dispatch) accordingly:

```rust
// Before
let result = tools::nil_diagnostics(params.file_path).await?;

// After
let result = tools::nil_diagnostics(params.file_path, params.offset, params.limit).await?;
```

---

## Truncation Pattern: Full Example (flake_show)

### Before (no context-saving)

```rust
#[derive(Debug, Serialize)]
pub struct NixFlakeShowResult {
    pub success: bool,
    pub outputs: serde_json::Value,
    pub stderr: String,
}

pub async fn nix_flake_show(params: NixFlakeShowParams) -> Result<NixFlakeShowResult, String> {
    // ... run nix command ...
    let outputs = if result.success {
        serde_json::from_str(&result.stdout).unwrap_or(serde_json::Value::Null)
    } else {
        serde_json::Value::Null
    };

    Ok(NixFlakeShowResult {
        success: result.success,
        outputs,
        stderr: result.stderr,
    })
}
```

### After (with truncation)

```rust
#[derive(Debug, Serialize)]
pub struct NixFlakeShowResult {
    pub success: bool,
    pub outputs: serde_json::Value,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_flake_show(params: NixFlakeShowParams) -> Result<NixFlakeShowResult, String> {
    // ... run nix command ...

    if !result.success {
        return Ok(NixFlakeShowResult {
            success: false,
            outputs: serde_json::Value::Null,
            stderr: result.stderr,
            truncated: None,
            truncation_info: None,
        });
    }

    let limits = OutputLimits {
        head: params.head,
        tail: params.tail,
        max_bytes: params.max_bytes,
        max_lines: None,
    };

    let limited = limit_text_output(&result.stdout, &limits);

    // Parse truncated content; fall back to string if JSON is broken
    let outputs = serde_json::from_str(&limited.content)
        .unwrap_or(serde_json::Value::String(limited.content));

    Ok(NixFlakeShowResult {
        success: true,
        outputs,
        stderr: result.stderr,
        truncated: if limited.truncated { Some(true) } else { None },
        truncation_info: limited.truncation_info,
    })
}
```

### Key details

- Truncation is applied to the raw stdout string before JSON parsing
- If truncation breaks JSON validity, the content degrades to a string value
- Early return for failures avoids unnecessary truncation logic
- Both `truncated` and `truncation_info` are omitted from serialization when not applicable

---

## Tool Schema Updates

Always add the corresponding properties to the tool's JSON schema so callers know the parameters exist.

### Pagination schema additions

```json
"offset": {
    "type": "integer",
    "description": "Skip first N entries for pagination. Defaults to 0."
},
"limit": {
    "type": "integer",
    "description": "Maximum number of entries to return. Defaults to all."
}
```

### Truncation schema additions

```json
"max_bytes": {
    "type": "integer",
    "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
},
"head": {
    "type": "integer",
    "description": "Only return the first N lines of output."
},
"tail": {
    "type": "integer",
    "description": "Only return the last N lines of output."
}
```

---

## Full Audit Results from nix-mcp

### Tools with pagination

| Tool | Items | Params |
|------|-------|--------|
| store_ls | Directory entries | offset, limit |
| store_cat | File lines | offset, limit |
| store_path_info | Closure entries | closure_offset, closure_limit |
| search | Package results | offset, limit |
| derivation_show | Input derivations | inputs_offset, max_inputs |
| nil_diagnostics | Diagnostic items | offset, limit |
| nil_completions | Completion items | offset, limit |
| fh_search | Search results | offset, limit |
| fh_list_flakes | Flake list | offset, limit |
| fh_list_releases | Release list | offset, limit |
| fh_list_versions | Version list | offset, limit |

### Tools with truncation

| Tool | Content Type | Params |
|------|-------------|--------|
| build | Build logs | log_tail, max_log_bytes |
| flake_check | stdout/stderr | head, tail, max_bytes |
| log | Build logs | head, tail, max_bytes |
| flake_show | JSON output | head, tail, max_bytes |
| flake_metadata | JSON output | head, tail, max_bytes |
| eval | JSON output | head, tail, max_bytes |

### Tools that do NOT need context-saving

| Tool | Reason |
|------|--------|
| hash_path, hash_file | Single scalar output |
| copy | Status message |
| store_gc | Status message |
| fh_resolve | Single resolution |
| fh_add | Confirmation message |
| fh_status, fh_login | Status/message |
| cachix_push, cachix_use, cachix_status | Small status output |
| nil_hover | Single hover result |
| nil_definition | 1-3 locations |
| run, develop_run | User-initiated; output is their responsibility |
| flake_update, flake_lock, flake_init | Small operational output |
| task_status | Small metadata |
