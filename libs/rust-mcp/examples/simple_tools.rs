use mcp_server::{Context, Content, McpServer, Tool, ToolError, ToolResult};
use serde_json::{json, Value};

struct HelloTool;

#[async_trait::async_trait]
impl Tool for HelloTool {
    fn name(&self) -> &str {
        "hello"
    }

    fn description(&self) -> &str {
        "Says hello to the provided name"
    }

    fn input_schema(&self) -> Value {
        json!({
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Name to greet"
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let name = arguments
            .get("name")
            .and_then(|v| v.as_str())
            .unwrap_or("World");

        Ok(ToolResult {
            content: vec![Content::text(format!("Hello, {}!", name))],
            is_error: None,
        })
    }
}

struct EchoTool;

#[async_trait::async_trait]
impl Tool for EchoTool {
    fn name(&self) -> &str {
        "echo"
    }

    fn description(&self) -> &str {
        "Echoes back the provided message"
    }

    fn input_schema(&self) -> Value {
        json!({
            "type": "object",
            "properties": {
                "message": {
                    "type": "string",
                    "description": "Message to echo"
                }
            },
            "required": ["message"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let message = arguments
            .get("message")
            .and_then(|v| v.as_str())
            .ok_or_else(|| ToolError::InvalidArguments("Missing 'message' field".to_string()))?;

        Ok(ToolResult::text(message.to_string()))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let server = McpServer::builder("simple-tools-server", "0.1.0")
        .with_tool(HelloTool)
        .with_tool(EchoTool)
        .build();

    server.run_stdio().await?;
    Ok(())
}
