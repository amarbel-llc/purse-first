# Migration Guide: nix-mcp-server to rust-lib-mcp

## Current Status

✅ **Library Complete**: rust-lib-mcp is fully functional with:
- Tools, Resources, Prompts, and Sampling support
- Trait-based registration system
- Builder pattern for server setup
- All protocol infrastructure

✅ **Dependency Added**: nix-mcp-server now depends on rust-lib-mcp via path dependency

## Migration Scope

nix-mcp-server currently has:
- **49 exported functions** in tools/mod.rs (30+ actual tools)
- **3 resource types** (build-log, derivation, closure)
- **~600 lines** in server.rs for dispatching

## Migration Strategy

### Option 1: Gradual Migration (Recommended)

Convert tools one at a time while keeping the system functional.

**Advantages:**
- Can test each tool conversion
- Minimal risk of breaking changes
- Can pause migration at any point

**Steps:**
1. Create wrapper `ToolAdapter` for existing function-based tools
2. Update main.rs to use `McpServer::builder()`
3. Convert high-value tools first (nix_build, nix_flake_*, etc.)
4. Gradually convert remaining tools
5. Remove old server.rs when all tools converted

### Option 2: Big Bang Migration

Convert everything at once.

**Advantages:**
- Clean break, no technical debt
- Simpler final result

**Disadvantages:**
- Higher risk
- Harder to debug issues
- All-or-nothing approach

## Implementation Example

### Current Pattern (Function-based)

```rust
// src/tools/build.rs
pub async fn nix_build(params: NixBuildParams) -> Result<NixBuildResult, String> {
    // ... implementation ...
}

// src/server.rs
match name {
    "nix_build" => {
        let params: NixBuildParams = serde_json::from_value(arguments)?;
        tools::nix_build(params).await?
    }
    // ... 30+ more cases ...
}
```

### New Pattern (Trait-based)

```rust
// src/tools/build.rs
pub struct NixBuildTool;

#[async_trait]
impl Tool for NixBuildTool {
    fn name(&self) -> &str { "nix_build" }
    fn description(&self) -> &str { "Build a nix flake package..." }
    fn input_schema(&self) -> Value { json!({...}) }

    async fn execute(&self, arguments: Value, _ctx: &Context)
        -> Result<ToolResult, ToolError>
    {
        let params: NixBuildParams = serde_json::from_value(arguments)?;
        let result = nix_build(params).await?;  // Call existing function
        Ok(ToolResult::text(serde_json::to_string_pretty(&result)?))
    }
}

// Keep existing function for now
async fn nix_build(params: NixBuildParams) -> Result<NixBuildResult, String> {
    // ... same implementation ...
}

// src/main.rs
let server = McpServer::builder("nix-mcp-server", VERSION)
    .with_tool(NixBuildTool)
    .with_tool(NixFlakeShowTool)
    // ... register all tools ...
    .build();

server.run_stdio().await?;
```

## Tool Conversion Template

```rust
use mcp_server::{Content, Context, Tool, ToolError, ToolResult};
use serde_json::{json, Value};

pub struct MyTool;

#[async_trait::async_trait]
impl Tool for MyTool {
    fn name(&self) -> &str {
        "my_tool"
    }

    fn description(&self) -> &str {
        "Description from list_tools()"
    }

    fn input_schema(&self) -> Value {
        json!({
            // Copy from list_tools() in tools/mod.rs
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context)
        -> Result<ToolResult, ToolError>
    {
        let params: MyParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        let result = my_existing_function(params).await
            .map_err(|e| ToolError::ExecutionFailed(e))?;

        Ok(ToolResult::text(serde_json::to_string_pretty(&result)?))
    }
}
```

## Resource Migration

Current resources in `src/resources/`:
- `build_log.rs` - Implements read_build_log()
- `derivation.rs` - Implements read_derivation()
- `closure.rs` - Implements read_closure()

These need to implement the `Resource` trait from mcp-server.

## Files to Modify

### Phase 1: Setup
- [x] `Cargo.toml` - Add mcp-server dependency
- [ ] `src/main.rs` - Use McpServer::builder()

### Phase 2: Core Tools (High Priority)
- [ ] `src/tools/build.rs` - nix_build
- [ ] `src/tools/flake.rs` - nix_flake_* (6 tools)
- [ ] `src/tools/run.rs` - nix_run, nix_develop_run
- [ ] `src/tools/eval.rs` - nix_eval

### Phase 3: Support Tools
- [ ] `src/tools/search.rs` - nix_search
- [ ] `src/tools/log.rs` - nix_log
- [ ] `src/tools/store.rs` - nix_store_* (3 tools)
- [ ] `src/tools/derivation.rs` - nix_derivation_show
- [ ] `src/tools/hash.rs` - nix_hash_* (2 tools)

### Phase 4: External Tools
- [ ] `src/tools/flakehub.rs` - fh_* (9 tools)
- [ ] `src/tools/cachix.rs` - cachix_* (3 tools)
- [ ] `src/tools/lsp.rs` - nil_* (4 tools)

### Phase 5: Resources
- [ ] `src/resources/build_log.rs`
- [ ] `src/resources/derivation.rs`
- [ ] `src/resources/closure.rs`

### Phase 6: Cleanup
- [ ] Remove `src/server.rs` (replaced by library)
- [ ] Update `src/tools/mod.rs` (export structs instead of functions)

## Testing Strategy

1. Keep old server.rs temporarily
2. Add feature flag to switch between old/new
3. Test each converted tool with Claude Code
4. Compare outputs between old and new implementations
5. Remove old code once all tools verified

## Estimated Effort

- **Per Tool**: ~15-30 minutes (copy schema, wrap function)
- **30 Tools**: 8-15 hours
- **Resources**: 1-2 hours
- **Main.rs Update**: 1-2 hours
- **Testing**: 3-5 hours

**Total**: 13-24 hours for complete migration

## Next Steps

To continue migration:

1. **Quick Win**: Update main.rs to use library (preserve existing server)
2. **Prove It Works**: Convert 2-3 high-value tools
3. **Iterate**: Convert remaining tools in priority order
4. **Clean Up**: Remove old server code

## Example: Converted Build Tool

See `src/tools/build_new.rs` for a complete example of nix_build converted to the new pattern.
