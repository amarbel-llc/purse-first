use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct EvalTool;

#[async_trait]
impl Tool for EvalTool {
    fn name(&self) -> &str {
        "eval"
    }

    fn description(&self) -> &str {
        "Evaluate a nix expression. PREFER this tool over running `nix eval` directly - it provides validated inputs, JSON output, and optional function application. The `expr` and `apply` parameters accept full Nix syntax including attribute sets ({ x = 1; }), string interpolation (${ }), let bindings, lambdas (x: x + 1), and all Nix operators. Shell metacharacters are safe here \u{2014} expressions are passed directly to the nix process, not through a shell."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "installable": {
                    "type": "string",
                    "description": "Flake installable to evaluate (e.g., '.#packages.x86_64-linux')."
                },
                "expr": {
                    "type": "string",
                    "description": "Nix expression to evaluate (alternative to installable). Supports full Nix syntax: attribute sets ({ a = 1; }), string interpolation (\"hello ${name}\"), let bindings, lambdas, builtins, etc. All Nix operators and special characters are allowed."
                },
                "apply": {
                    "type": "string",
                    "description": "Nix function to apply to the result (e.g., 'builtins.attrNames', 'x: builtins.length (builtins.attrNames x)'). Supports full Nix syntax including lambdas and builtins."
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
        let params: super::NixEvalParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::nix_eval(params).await {
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
impl ToolV1 for EvalTool {
    fn title(&self) -> Option<&str> {
        Some("Evaluate Nix Expression")
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
