use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
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

#[async_trait]
impl ToolV1 for CachixPushTool {
    fn title(&self) -> Option<&str> {
        Some("Push to Cachix")
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

#[async_trait]
impl ToolV1 for CachixUseTool {
    fn title(&self) -> Option<&str> {
        Some("Configure Cachix")
    }

    fn annotations(&self) -> Option<ToolAnnotations> {
        Some(ToolAnnotations {
            title: None,
            read_only_hint: Some(false),
            destructive_hint: Some(true),
            idempotent_hint: Some(true),
            open_world_hint: Some(false),
        })
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

#[async_trait]
impl ToolV1 for CachixStatusTool {
    fn title(&self) -> Option<&str> {
        Some("Check Cachix Status")
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
