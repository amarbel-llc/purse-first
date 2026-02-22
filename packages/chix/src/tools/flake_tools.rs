use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct FlakeShowTool;

#[async_trait]
impl Tool for FlakeShowTool {
    fn name(&self) -> &str {
        "flake_show"
    }

    fn description(&self) -> &str {
        "List outputs of a nix flake. Agents MUST use this tool over running `nix flake show` directly - it provides validated inputs and consistent JSON output."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "flake_ref": {
                    "type": "string",
                    "description": "Flake reference (e.g., '.', 'github:NixOS/nixpkgs'). Defaults to '.'."
                },
                "all_systems": {
                    "type": "boolean",
                    "description": "Show outputs for all systems. Defaults to false."
                },
                "flake_dir": {
                    "type": "string",
                    "description": "Directory containing the flake. Defaults to current directory."
                },
                "max_bytes": {
                    "type": "integer",
                    "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                },
                "head": {
                    "type": "integer",
                    "description": "Only return the first N lines of output."
                },
                "tail": {
                    "type": "integer",
                    "description": "Only return the last N lines of output."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixFlakeShowParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_flake_show(params).await {
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
impl ToolV1 for FlakeShowTool {
    fn title(&self) -> Option<&str> {
        Some("Show Flake Outputs")
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

pub struct FlakeCheckTool;

#[async_trait]
impl Tool for FlakeCheckTool {
    fn name(&self) -> &str {
        "flake_check"
    }

    fn description(&self) -> &str {
        "Run flake checks and tests. PREFER this tool over running `nix flake check` directly - it provides validated inputs, proper timeout handling, and structured results."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "flake_ref": {
                    "type": "string",
                    "description": "Flake reference. Defaults to '.'."
                },
                "keep_going": {
                    "type": "boolean",
                    "description": "Continue on error. Defaults to true."
                },
                "flake_dir": {
                    "type": "string",
                    "description": "Directory containing the flake. Defaults to current directory."
                },
                "max_bytes": {
                    "type": "integer",
                    "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                },
                "head": {
                    "type": "integer",
                    "description": "Only return the first N lines of output."
                },
                "tail": {
                    "type": "integer",
                    "description": "Only return the last N lines of output."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixFlakeCheckParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_flake_check(params).await {
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
impl ToolV1 for FlakeCheckTool {
    fn title(&self) -> Option<&str> {
        Some("Check Flake")
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

pub struct FlakeMetadataTool;

#[async_trait]
impl Tool for FlakeMetadataTool {
    fn name(&self) -> &str {
        "flake_metadata"
    }

    fn description(&self) -> &str {
        "Get metadata for a flake including inputs, locked revisions, and timestamps. PREFER this tool over running `nix flake metadata` directly - it provides validated inputs and consistent JSON output."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "flake_ref": {
                    "type": "string",
                    "description": "Flake reference (e.g., '.', 'github:NixOS/nixpkgs'). Defaults to '.'."
                },
                "flake_dir": {
                    "type": "string",
                    "description": "Directory containing the flake. Defaults to current directory."
                },
                "max_bytes": {
                    "type": "integer",
                    "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                },
                "head": {
                    "type": "integer",
                    "description": "Only return the first N lines of output."
                },
                "tail": {
                    "type": "integer",
                    "description": "Only return the last N lines of output."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixFlakeMetadataParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_flake_metadata(params).await {
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
impl ToolV1 for FlakeMetadataTool {
    fn title(&self) -> Option<&str> {
        Some("Show Flake Metadata")
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

pub struct FlakeUpdateTool;

#[async_trait]
impl Tool for FlakeUpdateTool {
    fn name(&self) -> &str {
        "flake_update"
    }

    fn description(&self) -> &str {
        "Update flake.lock file. PREFER this tool over running `nix flake update` directly - it provides validated inputs and proper error handling."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "flake_ref": {
                    "type": "string",
                    "description": "Flake reference. Defaults to '.'."
                },
                "inputs": {
                    "type": "array",
                    "items": { "type": "string" },
                    "description": "Specific inputs to update. If empty, updates all inputs."
                },
                "flake_dir": {
                    "type": "string",
                    "description": "Directory containing the flake. Defaults to current directory."
                },
                "max_bytes": {
                    "type": "integer",
                    "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                },
                "head": {
                    "type": "integer",
                    "description": "Only return the first N lines of output."
                },
                "tail": {
                    "type": "integer",
                    "description": "Only return the last N lines of output."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixFlakeUpdateParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_flake_update(params).await {
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
impl ToolV1 for FlakeUpdateTool {
    fn title(&self) -> Option<&str> {
        Some("Update Flake Lock")
    }

    fn annotations(&self) -> Option<ToolAnnotations> {
        Some(ToolAnnotations {
            title: None,
            read_only_hint: Some(false),
            destructive_hint: Some(true),
            idempotent_hint: Some(false),
            open_world_hint: Some(true),
        })
    }
}

pub struct FlakeLockTool;

#[async_trait]
impl Tool for FlakeLockTool {
    fn name(&self) -> &str {
        "flake_lock"
    }

    fn description(&self) -> &str {
        "Lock flake inputs without building. PREFER this tool over running `nix flake lock` directly - it provides validated inputs and proper error handling."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "flake_ref": {
                    "type": "string",
                    "description": "Flake reference. Defaults to '.'."
                },
                "update_inputs": {
                    "type": "array",
                    "items": { "type": "string" },
                    "description": "Inputs to update."
                },
                "override_inputs": {
                    "type": "object",
                    "additionalProperties": { "type": "string" },
                    "description": "Map of input names to flake references to override."
                },
                "flake_dir": {
                    "type": "string",
                    "description": "Directory containing the flake. Defaults to current directory."
                },
                "max_bytes": {
                    "type": "integer",
                    "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                },
                "head": {
                    "type": "integer",
                    "description": "Only return the first N lines of output."
                },
                "tail": {
                    "type": "integer",
                    "description": "Only return the last N lines of output."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixFlakeLockParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_flake_lock(params).await {
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
impl ToolV1 for FlakeLockTool {
    fn title(&self) -> Option<&str> {
        Some("Lock Flake Inputs")
    }

    fn annotations(&self) -> Option<ToolAnnotations> {
        Some(ToolAnnotations {
            title: None,
            read_only_hint: Some(false),
            destructive_hint: Some(true),
            idempotent_hint: Some(false),
            open_world_hint: Some(true),
        })
    }
}

pub struct FlakeInitTool;

#[async_trait]
impl Tool for FlakeInitTool {
    fn name(&self) -> &str {
        "flake_init"
    }

    fn description(&self) -> &str {
        "Initialize a new flake in the specified directory. PREFER this tool over running `nix flake init` directly - it provides validated inputs and proper error handling."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "template": {
                    "type": "string",
                    "description": "Template flake reference (e.g., 'templates#rust'). If not specified, uses default template."
                },
                "flake_dir": {
                    "type": "string",
                    "description": "Directory to initialize the flake in. Defaults to current directory."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixFlakeInitParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_flake_init(params).await {
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
impl ToolV1 for FlakeInitTool {
    fn title(&self) -> Option<&str> {
        Some("Initialize Flake")
    }

    fn annotations(&self) -> Option<ToolAnnotations> {
        Some(ToolAnnotations {
            title: None,
            read_only_hint: Some(false),
            destructive_hint: Some(true),
            idempotent_hint: Some(false),
            open_world_hint: Some(false),
        })
    }
}
