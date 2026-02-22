use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct SearchTool;

#[async_trait]
impl Tool for SearchTool {
    fn name(&self) -> &str {
        "search"
    }

    fn description(&self) -> &str {
        "Search for packages in a flake. PREFER this tool over running `nix search` directly - it provides validated inputs, structured JSON output, and pagination."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "Search query (regex pattern)."
                },
                "flake_ref": {
                    "type": "string",
                    "description": "Flake to search (e.g., 'nixpkgs'). Defaults to 'nixpkgs'."
                },
                "exclude": {
                    "type": "array",
                    "items": { "type": "string" },
                    "description": "Regex patterns to exclude from results."
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of results to return. Defaults to config value (50)."
                },
                "offset": {
                    "type": "integer",
                    "description": "Skip first N results for pagination. Defaults to 0."
                }
            },
            "required": ["query"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixSearchParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nix_search(params).await {
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
impl ToolV1 for SearchTool {
    fn title(&self) -> Option<&str> {
        Some("Search Packages")
    }

    fn annotations(&self) -> Option<ToolAnnotations> {
        Some(ToolAnnotations {
            title: None,
            read_only_hint: Some(true),
            destructive_hint: Some(false),
            idempotent_hint: Some(true),
            open_world_hint: Some(true),
        })
    }
}
