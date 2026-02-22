use crate::background::{get_task_info, list_tasks};
use crate::resources::{self, ResourceReadParams};
use crate::tools::{
    self, CachixPushParams, CachixStatusParams, CachixUseParams, FhAddParams, FhFetchParams,
    FhListFlakesParams, FhListReleasesParams, FhListVersionsParams, FhLoginParams, FhResolveParams,
    FhSearchParams, NilCompletionsParams, NilDefinitionParams, NilDiagnosticsParams,
    NilHoverParams, NixBuildParams, NixCopyParams, NixDerivationShowParams, NixDevelopRunParams,
    NixEvalParams, NixFlakeCheckParams, NixFlakeInitParams, NixFlakeLockParams,
    NixFlakeMetadataParams, NixFlakeShowParams, NixFlakeUpdateParams, NixHashFileParams,
    NixHashPathParams, NixLogParams, NixRunParams, NixSearchParams, NixStoreCatParams,
    NixStoreGcParams, NixStoreLsParams, NixStorePathInfoParams, TaskStatusParams,
};
use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Debug, Deserialize)]
struct JsonRpcRequest {
    jsonrpc: String,
    id: Option<Value>,
    method: String,
    params: Option<Value>,
}

#[derive(Debug, Serialize)]
struct JsonRpcResponse {
    jsonrpc: String,
    id: Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    result: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<JsonRpcError>,
}

#[derive(Debug, Serialize)]
struct JsonRpcError {
    code: i32,
    message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    data: Option<Value>,
}

#[derive(Debug, Serialize)]
struct ServerInfo {
    name: String,
    version: String,
}

#[derive(Debug, Serialize)]
struct InitializeResult {
    #[serde(rename = "protocolVersion")]
    protocol_version: String,
    capabilities: Capabilities,
    #[serde(rename = "serverInfo")]
    server_info: ServerInfo,
}

#[derive(Debug, Serialize)]
struct Capabilities {
    tools: ToolsCapability,
    resources: ResourcesCapability,
}

#[derive(Debug, Serialize)]
struct ToolsCapability {
    #[serde(rename = "listChanged")]
    list_changed: bool,
}

#[derive(Debug, Serialize)]
struct ResourcesCapability {
    #[serde(rename = "subscribe")]
    subscribe: bool,
    #[serde(rename = "listChanged")]
    list_changed: bool,
}

#[derive(Debug, Serialize)]
struct ToolsListResult {
    tools: Vec<ToolDefinition>,
}

#[derive(Debug, Serialize)]
struct ToolDefinition {
    name: String,
    description: String,
    #[serde(rename = "inputSchema")]
    input_schema: Value,
}

#[derive(Debug, Serialize)]
struct ToolCallResult {
    content: Vec<ContentItem>,
    #[serde(rename = "isError", skip_serializing_if = "Option::is_none")]
    is_error: Option<bool>,
}

#[derive(Debug, Serialize)]
struct ResourcesListResult {
    resources: Vec<ResourceDefinition>,
}

#[derive(Debug, Serialize)]
struct ResourceDefinition {
    uri: String,
    name: String,
    description: String,
    #[serde(rename = "mimeType")]
    mime_type: String,
}

#[derive(Debug, Serialize)]
struct ResourceReadResult {
    contents: Vec<ResourceContentItem>,
}

#[derive(Debug, Serialize)]
struct ResourceContentItem {
    uri: String,
    #[serde(rename = "mimeType")]
    mime_type: String,
    text: String,
}

#[derive(Debug, Serialize)]
struct ContentItem {
    #[serde(rename = "type")]
    content_type: String,
    text: String,
}

pub struct Server {}

impl Server {
    pub fn new() -> Self {
        Server {}
    }

    pub async fn handle_request(&self, request: &str) -> Value {
        let parsed: Result<JsonRpcRequest, _> = serde_json::from_str(request);

        let response = match parsed {
            Ok(req) => self.dispatch(req).await,
            Err(e) => JsonRpcResponse {
                jsonrpc: "2.0".to_string(),
                id: Value::Null,
                result: None,
                error: Some(JsonRpcError {
                    code: -32700,
                    message: format!("Parse error: {}", e),
                    data: None,
                }),
            },
        };

        serde_json::to_value(response).unwrap_or(Value::Null)
    }

    async fn dispatch(&self, req: JsonRpcRequest) -> JsonRpcResponse {
        let id = req.id.clone().unwrap_or(Value::Null);

        let result = match req.method.as_str() {
            "initialize" => self.handle_initialize().await,
            "notifications/initialized" => return self.empty_response(id),
            "tools/list" => self.handle_tools_list().await,
            "tools/call" => self.handle_tool_call(req.params).await,
            "resources/list" => self.handle_resources_list().await,
            "resources/read" => self.handle_resources_read(req.params).await,
            _ => Err(JsonRpcError {
                code: -32601,
                message: format!("Method not found: {}", req.method),
                data: None,
            }),
        };

        match result {
            Ok(value) => JsonRpcResponse {
                jsonrpc: "2.0".to_string(),
                id,
                result: Some(value),
                error: None,
            },
            Err(e) => JsonRpcResponse {
                jsonrpc: "2.0".to_string(),
                id,
                result: None,
                error: Some(e),
            },
        }
    }

    fn empty_response(&self, id: Value) -> JsonRpcResponse {
        JsonRpcResponse {
            jsonrpc: "2.0".to_string(),
            id,
            result: Some(Value::Object(serde_json::Map::new())),
            error: None,
        }
    }

    async fn handle_initialize(&self) -> Result<Value, JsonRpcError> {
        let result = InitializeResult {
            protocol_version: "2024-11-05".to_string(),
            capabilities: Capabilities {
                tools: ToolsCapability {
                    list_changed: false,
                },
                resources: ResourcesCapability {
                    subscribe: false,
                    list_changed: false,
                },
            },
            server_info: ServerInfo {
                name: "chix".to_string(),
                version: env!("CARGO_PKG_VERSION").to_string(),
            },
        };
        serde_json::to_value(result).map_err(|e| JsonRpcError {
            code: -32603,
            message: e.to_string(),
            data: None,
        })
    }

    async fn handle_tools_list(&self) -> Result<Value, JsonRpcError> {
        let tool_infos = tools::list_tools();
        let tools: Vec<ToolDefinition> = tool_infos
            .into_iter()
            .map(|t| ToolDefinition {
                name: t.name.to_string(),
                description: t.description.to_string(),
                input_schema: t.input_schema,
            })
            .collect();

        let result = ToolsListResult { tools };
        serde_json::to_value(result).map_err(|e| JsonRpcError {
            code: -32603,
            message: e.to_string(),
            data: None,
        })
    }

    async fn handle_tool_call(&self, params: Option<Value>) -> Result<Value, JsonRpcError> {
        let params = params.ok_or_else(|| JsonRpcError {
            code: -32602,
            message: "Missing params".to_string(),
            data: None,
        })?;

        let name = params
            .get("name")
            .and_then(|v| v.as_str())
            .ok_or_else(|| JsonRpcError {
                code: -32602,
                message: "Missing tool name".to_string(),
                data: None,
            })?;

        let arguments = params
            .get("arguments")
            .cloned()
            .unwrap_or(Value::Object(serde_json::Map::new()));

        let result = self.call_tool(name, arguments).await;

        match result {
            Ok(value) => {
                let tool_result = ToolCallResult {
                    content: vec![ContentItem {
                        content_type: "text".to_string(),
                        text: serde_json::to_string_pretty(&value).unwrap_or_default(),
                    }],
                    is_error: None,
                };
                serde_json::to_value(tool_result).map_err(|e| JsonRpcError {
                    code: -32603,
                    message: e.to_string(),
                    data: None,
                })
            }
            Err(e) => {
                let tool_result = ToolCallResult {
                    content: vec![ContentItem {
                        content_type: "text".to_string(),
                        text: e,
                    }],
                    is_error: Some(true),
                };
                serde_json::to_value(tool_result).map_err(|e| JsonRpcError {
                    code: -32603,
                    message: e.to_string(),
                    data: None,
                })
            }
        }
    }

    async fn call_tool(&self, name: &str, arguments: Value) -> Result<Value, String> {
        match name {
            "build" => {
                let params: NixBuildParams = serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_build(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "flake_show" => {
                let params: NixFlakeShowParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_flake_show(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "flake_check" => {
                let params: NixFlakeCheckParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_flake_check(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "flake_metadata" => {
                let params: NixFlakeMetadataParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_flake_metadata(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "flake_update" => {
                let params: NixFlakeUpdateParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_flake_update(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "flake_lock" => {
                let params: NixFlakeLockParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_flake_lock(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "flake_init" => {
                let params: NixFlakeInitParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_flake_init(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "run" => {
                let params: NixRunParams = serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_run(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "develop_run" => {
                let params: NixDevelopRunParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::nix_develop_run(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "log" => {
                let params: NixLogParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::nix_log(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "eval" => {
                let params: NixEvalParams = serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_eval(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "search" => {
                let params: NixSearchParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::nix_search(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "store_path_info" => {
                let params: NixStorePathInfoParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::nix_store_path_info(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "store_gc" => {
                let params: NixStoreGcParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_store_gc(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "store_ls" => {
                let params: NixStoreLsParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::nix_store_ls(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "store_cat" => {
                let params: NixStoreCatParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::nix_store_cat(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "derivation_show" => {
                let params: NixDerivationShowParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::nix_derivation_show(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "hash_path" => {
                let params: NixHashPathParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::nix_hash_path(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "hash_file" => {
                let params: NixHashFileParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::nix_hash_file(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "copy" => {
                let params: NixCopyParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::nix_copy(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "fh_search" => {
                let params: FhSearchParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::fh_search(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "fh_add" => {
                let params: FhAddParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::fh_add(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "fh_list_flakes" => {
                let params: FhListFlakesParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::fh_list_flakes(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "fh_list_releases" => {
                let params: FhListReleasesParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::fh_list_releases(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "fh_list_versions" => {
                let params: FhListVersionsParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::fh_list_versions(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "fh_resolve" => {
                let params: FhResolveParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::fh_resolve(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            // Cachix tools
            "cachix_push" => {
                let params: CachixPushParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::cachix_push(params.cache_name, params.store_paths).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "cachix_use" => {
                let params: CachixUseParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::cachix_use(params.cache_name).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "cachix_status" => {
                let _params: CachixStatusParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::cachix_status().await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            // FlakeHub cache tools
            "fh_status" => {
                let result = tools::fh_status().await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "fh_fetch" => {
                let params: FhFetchParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::fh_fetch(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "fh_login" => {
                let params: FhLoginParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = tools::fh_login(params).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            // Background task tools
            "task_status" => {
                let params: TaskStatusParams =
                    serde_json::from_value(arguments).unwrap_or_default();
                let result = match params.task_id {
                    Some(id) => {
                        if let Some(info) = get_task_info(&id) {
                            serde_json::json!({ "task": info })
                        } else {
                            serde_json::json!({ "error": format!("Task not found: {}", id) })
                        }
                    }
                    None => {
                        let tasks = list_tasks();
                        serde_json::json!({ "tasks": tasks })
                    }
                };
                Ok(result)
            }
            // nil LSP tools
            "nil_diagnostics" => {
                let params: NilDiagnosticsParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result =
                    tools::nil_diagnostics(params.file_path, params.offset, params.limit).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "nil_completions" => {
                let params: NilCompletionsParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result = tools::nil_completions(
                    params.file_path,
                    params.line,
                    params.character,
                    params.offset,
                    params.limit,
                )
                .await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "nil_hover" => {
                let params: NilHoverParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result =
                    tools::nil_hover(params.file_path, params.line, params.character).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            "nil_definition" => {
                let params: NilDefinitionParams =
                    serde_json::from_value(arguments).map_err(|e| e.to_string())?;
                let result =
                    tools::nil_definition(params.file_path, params.line, params.character).await?;
                serde_json::to_value(result).map_err(|e| e.to_string())
            }
            _ => Err(format!("Unknown tool: {}", name)),
        }
    }

    async fn handle_resources_list(&self) -> Result<Value, JsonRpcError> {
        let resource_infos = resources::list_resources();
        let resources: Vec<ResourceDefinition> = resource_infos
            .into_iter()
            .map(|r| ResourceDefinition {
                uri: r.uri,
                name: r.name,
                description: r.description,
                mime_type: r.mime_type,
            })
            .collect();

        let result = ResourcesListResult { resources };
        serde_json::to_value(result).map_err(|e| JsonRpcError {
            code: -32603,
            message: e.to_string(),
            data: None,
        })
    }

    async fn handle_resources_read(&self, params: Option<Value>) -> Result<Value, JsonRpcError> {
        let params = params.ok_or_else(|| JsonRpcError {
            code: -32602,
            message: "Missing params".to_string(),
            data: None,
        })?;

        let read_params: ResourceReadParams =
            serde_json::from_value(params).map_err(|e| JsonRpcError {
                code: -32602,
                message: format!("Invalid params: {}", e),
                data: None,
            })?;

        let content = resources::read_resource(&read_params.uri)
            .await
            .map_err(|e| JsonRpcError {
                code: -32603,
                message: e,
                data: None,
            })?;

        let result = ResourceReadResult {
            contents: vec![ResourceContentItem {
                uri: content.uri,
                mime_type: content.mime_type,
                text: content.text,
            }],
        };

        serde_json::to_value(result).map_err(|e| JsonRpcError {
            code: -32603,
            message: e.to_string(),
            data: None,
        })
    }
}
