# mcp-server

A reusable Rust library for building MCP (Model Context Protocol) servers.

## Features

- **Tools**: Execute commands and operations with structured input/output
- **Resources**: URI-based access to data and content
- **Prompts**: Template-based prompt generation for LLM interactions
- **Sampling**: Server-initiated LLM requests (experimental MCP extension)
- **Command Executor**: Generic async command execution with timeout support

## Quick Start

```rust
use mcp_server::{McpServer, Tool, ToolResult, Context, Content, ToolError};
use serde_json::{json, Value};

struct HelloTool;

#[async_trait::async_trait]
impl Tool for HelloTool {
    fn name(&self) -> &str { "hello" }
    fn description(&self) -> &str { "Says hello" }
    fn input_schema(&self) -> Value {
        json!({
            "type": "object",
            "properties": {
                "name": { "type": "string" }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context)
        -> Result<ToolResult, ToolError>
    {
        let name = arguments.get("name")
            .and_then(|v| v.as_str())
            .unwrap_or("World");

        Ok(ToolResult {
            content: vec![Content::text(format!("Hello, {}!", name))],
            is_error: None,
        })
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let server = McpServer::builder("hello-server", "0.1.0")
        .with_tool(HelloTool)
        .build();

    server.run_stdio().await?;
    Ok(())
}
```

## Examples

See the `examples/` directory for complete examples:

- `simple_tools.rs` - Basic tool server

## Development

### Building

```bash
just build          # Build with nix
just check          # Run cargo check + clippy
just fmt            # Format code
nix flake check     # Validate flake and run all checks
```

### Testing

```bash
just test           # Run tests
```

## License

MIT OR Apache-2.0
