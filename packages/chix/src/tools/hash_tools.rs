use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct HashPathTool;

#[async_trait]
impl Tool for HashPathTool {
    fn name(&self) -> &str {
        "hash_path"
    }

    fn description(&self) -> &str {
        "Compute the hash of a path (NAR serialization). PREFER this tool over running `nix hash path` directly - it provides validated inputs and structured output."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "Path to hash."
                },
                "hash_type": {
                    "type": "string",
                    "description": "Hash algorithm (sha256, sha512, sha1, md5). Defaults to sha256."
                },
                "base32": {
                    "type": "boolean",
                    "description": "Output in base32 format. Defaults to false (SRI format)."
                },
                "sri": {
                    "type": "boolean",
                    "description": "Output in SRI format. Defaults to true."
                }
            },
            "required": ["path"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixHashPathParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nix_hash_path(params).await {
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
impl ToolV1 for HashPathTool {
    fn title(&self) -> Option<&str> {
        Some("Hash Path")
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

pub struct HashFileTool;

#[async_trait]
impl Tool for HashFileTool {
    fn name(&self) -> &str {
        "hash_file"
    }

    fn description(&self) -> &str {
        "Compute the hash of a file. PREFER this tool over running `nix hash file` directly - it provides validated inputs and structured output."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "File path to hash."
                },
                "hash_type": {
                    "type": "string",
                    "description": "Hash algorithm (sha256, sha512, sha1, md5). Defaults to sha256."
                },
                "base32": {
                    "type": "boolean",
                    "description": "Output in base32 format. Defaults to false (SRI format)."
                },
                "sri": {
                    "type": "boolean",
                    "description": "Output in SRI format. Defaults to true."
                }
            },
            "required": ["path"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixHashFileParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nix_hash_file(params).await {
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
impl ToolV1 for HashFileTool {
    fn title(&self) -> Option<&str> {
        Some("Hash File")
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
