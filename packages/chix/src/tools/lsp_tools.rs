use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct NilDiagnosticsTool;

#[async_trait]
impl Tool for NilDiagnosticsTool {
    fn name(&self) -> &str {
        "nil_diagnostics"
    }

    fn description(&self) -> &str {
        "Get Nix language diagnostics (errors, warnings, undefined names) for a file using the nil language server."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "file_path": {
                    "type": "string",
                    "description": "Absolute path to the .nix file to analyze."
                },
                "offset": {
                    "type": "integer",
                    "description": "Skip first N diagnostics for pagination. Defaults to 0."
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of diagnostics to return. Defaults to all."
                }
            },
            "required": ["file_path"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NilDiagnosticsParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nil_diagnostics(params.file_path, params.offset, params.limit).await {
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
impl ToolV1 for NilDiagnosticsTool {
    fn title(&self) -> Option<&str> {
        Some("Nix Diagnostics")
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

pub struct NilCompletionsTool;

#[async_trait]
impl Tool for NilCompletionsTool {
    fn name(&self) -> &str {
        "nil_completions"
    }

    fn description(&self) -> &str {
        "Get Nix code completions at a specific position using the nil language server."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "file_path": {
                    "type": "string",
                    "description": "Absolute path to the .nix file."
                },
                "line": {
                    "type": "integer",
                    "description": "0-indexed line number."
                },
                "character": {
                    "type": "integer",
                    "description": "0-indexed character offset within the line."
                },
                "offset": {
                    "type": "integer",
                    "description": "Skip first N completions for pagination. Defaults to 0."
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of completions to return. Defaults to all."
                }
            },
            "required": ["file_path", "line", "character"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NilCompletionsParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nil_completions(
            params.file_path,
            params.line,
            params.character,
            params.offset,
            params.limit,
        )
        .await
        {
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
impl ToolV1 for NilCompletionsTool {
    fn title(&self) -> Option<&str> {
        Some("Nix Completions")
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

pub struct NilHoverTool;

#[async_trait]
impl Tool for NilHoverTool {
    fn name(&self) -> &str {
        "nil_hover"
    }

    fn description(&self) -> &str {
        "Get hover information (documentation, type info) at a specific position using the nil language server."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "file_path": {
                    "type": "string",
                    "description": "Absolute path to the .nix file."
                },
                "line": {
                    "type": "integer",
                    "description": "0-indexed line number."
                },
                "character": {
                    "type": "integer",
                    "description": "0-indexed character offset within the line."
                }
            },
            "required": ["file_path", "line", "character"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NilHoverParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nil_hover(params.file_path, params.line, params.character).await {
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
impl ToolV1 for NilHoverTool {
    fn title(&self) -> Option<&str> {
        Some("Nix Hover Info")
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

pub struct NilDefinitionTool;

#[async_trait]
impl Tool for NilDefinitionTool {
    fn name(&self) -> &str {
        "nil_definition"
    }

    fn description(&self) -> &str {
        "Go to definition for a symbol at a specific position using the nil language server."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "file_path": {
                    "type": "string",
                    "description": "Absolute path to the .nix file."
                },
                "line": {
                    "type": "integer",
                    "description": "0-indexed line number."
                },
                "character": {
                    "type": "integer",
                    "description": "0-indexed character offset within the line."
                }
            },
            "required": ["file_path", "line", "character"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NilDefinitionParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nil_definition(params.file_path, params.line, params.character).await {
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
impl ToolV1 for NilDefinitionTool {
    fn title(&self) -> Option<&str> {
        Some("Nix Go to Definition")
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
