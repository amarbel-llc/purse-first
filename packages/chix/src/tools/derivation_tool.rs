use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct DerivationShowTool;

#[async_trait]
impl Tool for DerivationShowTool {
    fn name(&self) -> &str {
        "derivation_show"
    }

    fn description(&self) -> &str {
        "Show the contents of a derivation. PREFER this tool over running `nix derivation show` directly - it provides validated inputs, structured JSON output, and summary mode for large dependency trees."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "installable": {
                    "type": "string",
                    "description": "Flake installable or store path. Defaults to '.#default'."
                },
                "recursive": {
                    "type": "boolean",
                    "description": "Include derivations of dependencies. Defaults to false."
                },
                "flake_dir": {
                    "type": "string",
                    "description": "Directory containing the flake. Defaults to current directory."
                },
                "summary_only": {
                    "type": "boolean",
                    "description": "Return only derivation summary (name, path, outputs, input count) instead of full content. Useful for exploring large dependency trees."
                },
                "max_inputs": {
                    "type": "integer",
                    "description": "Maximum number of input derivations to include. Defaults to config value (100)."
                },
                "inputs_offset": {
                    "type": "integer",
                    "description": "Skip first N input derivations for pagination. Defaults to 0."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::NixDerivationShowParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_derivation_show(params).await {
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
impl ToolV1 for DerivationShowTool {
    fn title(&self) -> Option<&str> {
        Some("Show Derivation")
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
