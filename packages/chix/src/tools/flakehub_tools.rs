use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolAnnotations, ToolError, ToolResult, ToolV1};
use serde_json::Value;

pub struct FhSearchTool;

#[async_trait]
impl Tool for FhSearchTool {
    fn name(&self) -> &str {
        "fh_search"
    }

    fn description(&self) -> &str {
        "Search FlakeHub for flakes matching a query. Agents MUST use this tool over running `fh search` directly - it provides structured JSON output."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "The search query."
                },
                "max_results": {
                    "type": "integer",
                    "description": "Maximum number of results to return from FlakeHub API. Defaults to 10."
                },
                "offset": {
                    "type": "integer",
                    "description": "Skip first N results for pagination. Defaults to 0."
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of results to return. Defaults to all."
                }
            },
            "required": ["query"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::FhSearchParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::fh_search(params).await {
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
impl ToolV1 for FhSearchTool {
    fn title(&self) -> Option<&str> {
        Some("Search FlakeHub")
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

pub struct FhAddTool;

#[async_trait]
impl Tool for FhAddTool {
    fn name(&self) -> &str {
        "fh_add"
    }

    fn description(&self) -> &str {
        "Add a flake input to your flake.nix from FlakeHub. Agents MUST use this tool over running `fh add` directly - it provides validated inputs and proper error handling."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "input_ref": {
                    "type": "string",
                    "description": "The flake reference to add (e.g., 'NixOS/nixpkgs' or 'NixOS/nixpkgs/0.2411.*')."
                },
                "flake_path": {
                    "type": "string",
                    "description": "Path to the flake.nix to modify. Defaults to './flake.nix'."
                },
                "input_name": {
                    "type": "string",
                    "description": "Name for the flake input. If not provided, inferred from the input URL."
                }
            },
            "required": ["input_ref"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::FhAddParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::fh_add(params).await {
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
impl ToolV1 for FhAddTool {
    fn title(&self) -> Option<&str> {
        Some("Add FlakeHub Input")
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

pub struct FhListFlakesTool;

#[async_trait]
impl Tool for FhListFlakesTool {
    fn name(&self) -> &str {
        "fh_list_flakes"
    }

    fn description(&self) -> &str {
        "List public flakes on FlakeHub. Agents MUST use this tool over running `fh list` directly - it provides structured JSON output."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of flakes to return."
                },
                "offset": {
                    "type": "integer",
                    "description": "Skip first N results for pagination. Defaults to 0."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::FhListFlakesParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::fh_list_flakes(params).await {
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
impl ToolV1 for FhListFlakesTool {
    fn title(&self) -> Option<&str> {
        Some("List FlakeHub Flakes")
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

pub struct FhListReleasesTool;

#[async_trait]
impl Tool for FhListReleasesTool {
    fn name(&self) -> &str {
        "fh_list_releases"
    }

    fn description(&self) -> &str {
        "List all releases for a specific flake on FlakeHub. Agents MUST use this tool over running `fh list releases` directly - it provides structured JSON output."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "flake": {
                    "type": "string",
                    "description": "The flake to list releases for (e.g., 'NixOS/nixpkgs')."
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of releases to return."
                },
                "offset": {
                    "type": "integer",
                    "description": "Skip first N results for pagination. Defaults to 0."
                }
            },
            "required": ["flake"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::FhListReleasesParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::fh_list_releases(params).await {
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
impl ToolV1 for FhListReleasesTool {
    fn title(&self) -> Option<&str> {
        Some("List Flake Releases")
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

pub struct FhListVersionsTool;

#[async_trait]
impl Tool for FhListVersionsTool {
    fn name(&self) -> &str {
        "fh_list_versions"
    }

    fn description(&self) -> &str {
        "List versions matching a constraint for a flake on FlakeHub. Agents MUST use this tool over running `fh list versions` directly - it provides structured JSON output."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "flake": {
                    "type": "string",
                    "description": "The flake to list versions for (e.g., 'NixOS/nixpkgs')."
                },
                "version_constraint": {
                    "type": "string",
                    "description": "Version constraint (e.g., '0.2411.*', '>=0.2405')."
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of versions to return."
                },
                "offset": {
                    "type": "integer",
                    "description": "Skip first N results for pagination. Defaults to 0."
                }
            },
            "required": ["flake", "version_constraint"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::FhListVersionsParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::fh_list_versions(params).await {
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
impl ToolV1 for FhListVersionsTool {
    fn title(&self) -> Option<&str> {
        Some("List Flake Versions")
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

pub struct FhResolveTool;

#[async_trait]
impl Tool for FhResolveTool {
    fn name(&self) -> &str {
        "fh_resolve"
    }

    fn description(&self) -> &str {
        "Resolve a FlakeHub flake reference to a store path. Agents MUST use this tool over running `fh resolve` directly - it provides validated inputs and structured output."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "flake_ref": {
                    "type": "string",
                    "description": "FlakeHub flake reference (e.g., 'NixOS/nixpkgs/0.2411.*#hello')."
                }
            },
            "required": ["flake_ref"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::FhResolveParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::fh_resolve(params).await {
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
impl ToolV1 for FhResolveTool {
    fn title(&self) -> Option<&str> {
        Some("Resolve FlakeHub Ref")
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

pub struct FhStatusTool;

#[async_trait]
impl Tool for FhStatusTool {
    fn name(&self) -> &str {
        "fh_status"
    }

    fn description(&self) -> &str {
        "Check FlakeHub login and cache status."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {}
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let _params: super::FhStatusParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::fh_status().await {
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
impl ToolV1 for FhStatusTool {
    fn title(&self) -> Option<&str> {
        Some("Check FlakeHub Status")
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

pub struct FhFetchTool;

#[async_trait]
impl Tool for FhFetchTool {
    fn name(&self) -> &str {
        "fh_fetch"
    }

    fn description(&self) -> &str {
        "Fetch a flake output from FlakeHub cache and create a GC root symlink."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "flake_ref": {
                    "type": "string",
                    "description": "FlakeHub flake reference (e.g., 'NixOS/nixpkgs/0.2411.*#hello')."
                },
                "target_link": {
                    "type": "string",
                    "description": "Path for the symlink (GC root)."
                }
            },
            "required": ["flake_ref", "target_link"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::FhFetchParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::fh_fetch(params).await {
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
impl ToolV1 for FhFetchTool {
    fn title(&self) -> Option<&str> {
        Some("Fetch from FlakeHub")
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

pub struct FhLoginTool;

#[async_trait]
impl Tool for FhLoginTool {
    fn name(&self) -> &str {
        "fh_login"
    }

    fn description(&self) -> &str {
        "Initiate FlakeHub OAuth login flow. Opens browser for authentication."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "token_file": {
                    "type": "string",
                    "description": "Optional path to token file for non-interactive auth."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::FhLoginParams =
            serde_json::from_value(arguments).unwrap_or_default();

        match super::fh_login(params).await {
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
impl ToolV1 for FhLoginTool {
    fn title(&self) -> Option<&str> {
        Some("Login to FlakeHub")
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
