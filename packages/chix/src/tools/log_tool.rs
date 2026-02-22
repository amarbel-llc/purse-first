use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct LogTool;

#[async_trait]
impl Tool for LogTool {
    fn name(&self) -> &str {
        "log"
    }

    fn description(&self) -> &str {
        "Get build logs for a derivation. Agents MUST use this tool over running `nix log` directly - it provides validated inputs and optional head/tail functionality."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "installable": {
                    "type": "string",
                    "description": "Flake installable or store path."
                },
                "head": {
                    "type": "integer",
                    "description": "Only return the first N lines."
                },
                "tail": {
                    "type": "integer",
                    "description": "Only return the last N lines."
                },
                "max_bytes": {
                    "type": "integer",
                    "description": "Maximum bytes of log output to return. Defaults to config value (100KB)."
                }
            },
            "required": ["installable"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixLogParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nix_log(params).await {
            Ok(result) => {
                let json = serde_json::to_string_pretty(&result)
                    .map_err(|e| ToolError::Serialization(e))?;
                Ok(ToolResult::text(json))
            }
            Err(e) => Ok(ToolResult::error(e)),
        }
    }
}

#[async_trait]
impl ToolV1 for LogTool {
    fn title(&self) -> Option<&str> {
        Some("Show Build Log")
    }

    fn annotations(&self) -> Option<ToolAnnotations> {
        Some(ToolAnnotations {
            title: None,
            read_only_hint: Some(true),
            destructive_hint: Some(false),
            idempotent_hint: Some(true),
            open_world_hint: Some(false),
        })
    }
}
