use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct RunTool;

#[async_trait]
impl Tool for RunTool {
    fn name(&self) -> &str {
        "run"
    }

    fn description(&self) -> &str {
        "Run a flake app. Agents MUST use this tool over running `nix run` directly - it provides validated inputs, secure argument handling, and proper process management."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "installable": {
                    "type": "string",
                    "description": "Flake installable to run. Defaults to '.#default'."
                },
                "args": {
                    "type": "array",
                    "items": { "type": "string" },
                    "description": "Arguments to pass to the app."
                },
                "flake_dir": {
                    "type": "string",
                    "description": "Directory containing the flake. Defaults to current directory."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixRunParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_run(params).await {
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
impl ToolV1 for RunTool {
    fn title(&self) -> Option<&str> {
        Some("Run Flake App")
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

pub struct DevelopRunTool;

#[async_trait]
impl Tool for DevelopRunTool {
    fn name(&self) -> &str {
        "develop_run"
    }

    fn description(&self) -> &str {
        "Run a command inside a flake's devShell. Agents MUST use this tool over running `nix develop -c` directly - it provides validated inputs, secure command execution, and proper process management. Use `flake_dir` to set the working directory instead of `cd`. Use separate entries in `commands` instead of shell operators like `&&`. Shell metacharacters are not allowed in command arguments."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "flake_ref": {
                    "type": "string",
                    "description": "Flake reference. Defaults to '.'."
                },
                "commands": {
                    "type": "array",
                    "minItems": 1,
                    "items": {
                        "type": "object",
                        "properties": {
                            "command": {
                                "type": "string",
                                "description": "Command to run in the devShell."
                            },
                            "args": {
                                "type": "array",
                                "items": { "type": "string" },
                                "description": "Arguments to pass to the command."
                            }
                        },
                        "required": ["command"]
                    },
                    "description": "Commands to run sequentially. Execution stops on the first failure (like && in shell). Each command runs as a separate `nix develop -c` invocation."
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
            },
            "required": ["commands"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixDevelopRunParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::nix_develop_run(params).await {
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
impl ToolV1 for DevelopRunTool {
    fn title(&self) -> Option<&str> {
        Some("Run in Dev Shell")
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
