use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolError, ToolResult};
use serde_json::Value;

pub struct CachixPushTool;

#[async_trait]
impl Tool for CachixPushTool {
    fn name(&self) -> &str {
        "cachix_push"
    }

    fn description(&self) -> &str {
        "Push store paths to a Cachix binary cache. Requires CACHIX_AUTH_TOKEN env var or config in ~/.config/nix-mcp-server/config.toml."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "cache_name": {
                    "type": "string",
                    "description": "Cachix cache name. Uses default from config if not specified."
                },
                "store_paths": {
                    "type": "array",
                    "items": { "type": "string" },
                    "description": "Nix store paths to push (e.g., '/nix/store/...-hello')."
                }
            },
            "required": ["store_paths"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::CachixPushParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::cachix_push(params.cache_name, params.store_paths).await {
            Ok(result) => {
                let json = serde_json::to_string_pretty(&result)
                    .map_err(|e| ToolError::Serialization(e))?;
                Ok(ToolResult::text(json))
            }
            Err(e) => Ok(ToolResult::error(e)),
        }
    }
}

pub struct CachixUseTool;

#[async_trait]
impl Tool for CachixUseTool {
    fn name(&self) -> &str {
        "cachix_use"
    }

    fn description(&self) -> &str {
        "Configure Nix to use a Cachix binary cache as a substituter."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "cache_name": {
                    "type": "string",
                    "description": "Cachix cache name to add as substituter."
                }
            },
            "required": ["cache_name"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::CachixUseParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::cachix_use(params.cache_name).await {
            Ok(result) => {
                let json = serde_json::to_string_pretty(&result)
                    .map_err(|e| ToolError::Serialization(e))?;
                Ok(ToolResult::text(json))
            }
            Err(e) => Ok(ToolResult::error(e)),
        }
    }
}

pub struct CachixStatusTool;

#[async_trait]
impl Tool for CachixStatusTool {
    fn name(&self) -> &str {
        "cachix_status"
    }

    fn description(&self) -> &str {
        "Check Cachix authentication status."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {}
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let _params: super::CachixStatusParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::cachix_status().await {
            Ok(result) => {
                let json = serde_json::to_string_pretty(&result)
                    .map_err(|e| ToolError::Serialization(e))?;
                Ok(ToolResult::text(json))
            }
            Err(e) => Ok(ToolResult::error(e)),
        }
    }
}
