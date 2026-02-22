use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct StorePathInfoTool;

#[async_trait]
impl Tool for StorePathInfoTool {
    fn name(&self) -> &str {
        "store_path_info"
    }

    fn description(&self) -> &str {
        "Get information about a store path or installable. PREFER this tool over running `nix path-info` directly - it provides validated inputs, structured JSON output, and closure limiting."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "Store path or flake installable to query."
                },
                "closure": {
                    "type": "boolean",
                    "description": "Include closure (all dependencies). Defaults to false."
                },
                "derivation": {
                    "type": "boolean",
                    "description": "Show derivation path instead of output path. Defaults to false."
                },
                "closure_limit": {
                    "type": "integer",
                    "description": "Maximum number of closure entries to return. Defaults to config value (100)."
                },
                "closure_offset": {
                    "type": "integer",
                    "description": "Skip first N closure entries for pagination. Defaults to 0."
                }
            },
            "required": ["path"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixStorePathInfoParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nix_store_path_info(params).await {
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
impl ToolV1 for StorePathInfoTool {
    fn title(&self) -> Option<&str> {
        Some("Show Store Path Info")
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

pub struct StoreGcTool;

#[async_trait]
impl Tool for StoreGcTool {
    fn name(&self) -> &str {
        "store_gc"
    }

    fn description(&self) -> &str {
        "Run garbage collection on the Nix store. PREFER this tool over running `nix store gc` directly - it provides validated inputs and proper error handling."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "dry_run": {
                    "type": "boolean",
                    "description": "Only print what would be deleted. Defaults to false."
                },
                "max_freed": {
                    "type": "string",
                    "description": "Stop after freeing this much space (e.g., '1G', '500M')."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixStoreGcParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_store_gc(params).await {
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
impl ToolV1 for StoreGcTool {
    fn title(&self) -> Option<&str> {
        Some("Garbage Collect Store")
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

pub struct StoreLsTool;

#[async_trait]
impl Tool for StoreLsTool {
    fn name(&self) -> &str {
        "store_ls"
    }

    fn description(&self) -> &str {
        "List directory contents of a path that resolves into /nix/store/. Accepts ./result, ./result/bin, /nix/store/..., etc. Resolves symlinks and validates the canonical path is within the Nix store."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "Path to list (e.g., './result', './result/bin', '/nix/store/...-name/bin'). Symlinks are resolved before validation."
                },
                "long": {
                    "type": "boolean",
                    "description": "Include file sizes for regular files. Defaults to false."
                },
                "offset": {
                    "type": "integer",
                    "description": "Skip first N entries for pagination. Defaults to 0."
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of entries to return. Defaults to all entries."
                }
            },
            "required": ["path"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixStoreLsParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nix_store_ls(params).await {
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
impl ToolV1 for StoreLsTool {
    fn title(&self) -> Option<&str> {
        Some("List Store Path")
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

pub struct StoreCatTool;

#[async_trait]
impl Tool for StoreCatTool {
    fn name(&self) -> &str {
        "store_cat"
    }

    fn description(&self) -> &str {
        "Read file contents from a path that resolves into /nix/store/. Accepts ./result, /nix/store/..., etc. Supports line-based pagination with offset and limit. Resolves symlinks and validates the canonical path is within the Nix store."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "Path to the file to read (e.g., './result/bin/hello', '/nix/store/...-name/etc/config'). Symlinks are resolved before validation."
                },
                "offset": {
                    "type": "integer",
                    "description": "Number of lines to skip from the beginning. Defaults to 0."
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of lines to return. Defaults to all lines."
                }
            },
            "required": ["path"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixStoreCatParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nix_store_cat(params).await {
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
impl ToolV1 for StoreCatTool {
    fn title(&self) -> Option<&str> {
        Some("Read Store File")
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
