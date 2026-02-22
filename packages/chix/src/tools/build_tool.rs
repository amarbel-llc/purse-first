use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct BuildTool;

#[async_trait]
impl Tool for BuildTool {
    fn name(&self) -> &str {
        "build"
    }

    fn description(&self) -> &str {
        "Build a nix flake package. Returns store paths on success. Agents MUST use this tool over running `nix build` directly - it provides validated inputs, structured output, and proper error handling."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "installable": {
                    "type": "string",
                    "description": "Flake installable (e.g., '.#default', 'nixpkgs#hello'). Defaults to '.#default'."
                },
                "print_build_logs": {
                    "type": "boolean",
                    "description": "Whether to print build logs (-L flag). Defaults to true."
                },
                "flake_dir": {
                    "type": "string",
                    "description": "Directory containing the flake. Defaults to current directory."
                },
                "max_log_bytes": {
                    "type": "integer",
                    "description": "Maximum bytes of build log output to return. Defaults to config value (100KB)."
                },
                "log_tail": {
                    "type": "integer",
                    "description": "Only return the last N lines of build log. Takes precedence over max_log_bytes."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixBuildParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_build(params).await {
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
impl ToolV1 for BuildTool {
    fn title(&self) -> Option<&str> {
        Some("Build Nix Package")
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
