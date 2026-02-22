use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct CopyTool;

#[async_trait]
impl Tool for CopyTool {
    fn name(&self) -> &str {
        "copy"
    }

    fn description(&self) -> &str {
        "Copy store paths between Nix stores. PREFER this tool over running `nix copy` directly - it provides validated inputs and proper error handling."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "installable": {
                    "type": "string",
                    "description": "Store path or flake installable to copy."
                },
                "to": {
                    "type": "string",
                    "description": "Destination store URI (e.g., 's3://bucket', 'ssh://host')."
                },
                "from": {
                    "type": "string",
                    "description": "Source store URI."
                }
            },
            "required": ["installable"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixCopyParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nix_copy(params).await {
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
impl ToolV1 for CopyTool {
    fn title(&self) -> Option<&str> {
        Some("Copy Store Paths")
    }

    fn annotations(&self) -> Option<ToolAnnotations> {
        Some(ToolAnnotations {
            title: None,
            read_only_hint: Some(false),
            destructive_hint: Some(false),
            idempotent_hint: Some(true),
            open_world_hint: Some(true),
        })
    }
}
