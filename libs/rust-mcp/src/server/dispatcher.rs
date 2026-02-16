use crate::error::ServerError;
use crate::protocol::{
    Capabilities, InitializeResult, JsonRpcError, JsonRpcRequest, JsonRpcResponse, ServerInfo,
};
use crate::server::Context;

#[cfg(feature = "tools")]
use crate::tools::{ToolInfo, ToolRegistry, ToolResult};

#[cfg(feature = "resources")]
use crate::resources::{ResourceRegistry};

#[cfg(feature = "prompts")]
use crate::prompts::PromptRegistry;

#[cfg(feature = "sampling")]
use crate::sampling::SamplingHandler;

use serde::{Deserialize, Serialize};
use serde_json::Value;

/// MCP Server
pub struct McpServer {
    pub(crate) server_info: ServerInfo,
    pub(crate) protocol_version: String,
    pub(crate) capabilities: Capabilities,

    #[cfg(feature = "tools")]
    pub(crate) tool_registry: ToolRegistry,

    #[cfg(feature = "resources")]
    pub(crate) resource_registry: ResourceRegistry,

    #[cfg(feature = "prompts")]
    pub(crate) prompt_registry: PromptRegistry,

    #[cfg(feature = "sampling")]
    pub(crate) sampling_handler: Option<Arc<dyn SamplingHandler>>,
}

impl McpServer {
    /// Create a new server builder
    pub fn builder(name: impl Into<String>, version: impl Into<String>) -> super::McpServerBuilder {
        super::McpServerBuilder::new(name, version)
    }

    /// Run the server on stdin/stdout
    pub async fn run_stdio(self) -> Result<(), ServerError> {
        super::stdio::run_stdio_server(self).await
    }

    /// Handle a JSON-RPC request
    pub async fn handle_request(&self, request: &str, ctx: &mut Context) -> Value {
        let parsed: Result<JsonRpcRequest, _> = serde_json::from_str(request);

        let response = match parsed {
            Ok(req) => self.dispatch(req, ctx).await,
            Err(e) => JsonRpcResponse::error(
                Value::Null,
                JsonRpcError::parse_error(format!("Parse error: {}", e)),
            ),
        };

        serde_json::to_value(response).unwrap_or(Value::Null)
    }

    async fn dispatch(&self, req: JsonRpcRequest, ctx: &mut Context) -> JsonRpcResponse {
        let id = req.id.clone().unwrap_or(Value::Null);

        let result = match req.method.as_str() {
            "initialize" => self.handle_initialize(req.params, ctx).await,
            "notifications/initialized" => return JsonRpcResponse::empty(id),
            #[cfg(feature = "tools")]
            "tools/list" => self.handle_tools_list().await,
            #[cfg(feature = "tools")]
            "tools/call" => self.handle_tool_call(req.params, ctx).await,
            #[cfg(feature = "resources")]
            "resources/list" => self.handle_resources_list().await,
            #[cfg(feature = "resources")]
            "resources/read" => self.handle_resources_read(req.params, ctx).await,
            #[cfg(feature = "prompts")]
            "prompts/list" => self.handle_prompts_list().await,
            #[cfg(feature = "prompts")]
            "prompts/get" => self.handle_prompts_get(req.params, ctx).await,
            _ => Err(JsonRpcError::method_not_found(format!(
                "Method not found: {}",
                req.method
            ))),
        };

        match result {
            Ok(value) => JsonRpcResponse::success(id, value),
            Err(e) => JsonRpcResponse::error(id, e),
        }
    }

    async fn handle_initialize(
        &self,
        params: Option<Value>,
        ctx: &mut Context,
    ) -> Result<Value, JsonRpcError> {
        // Extract client capabilities if provided
        if let Some(params) = params {
            if let Ok(init_params) = serde_json::from_value::<InitializeParams>(params) {
                ctx.client_capabilities = init_params.capabilities.unwrap_or_default();
            }
        }

        let result = InitializeResult {
            protocol_version: self.protocol_version.clone(),
            capabilities: self.capabilities.clone(),
            server_info: self.server_info.clone(),
        };

        serde_json::to_value(result).map_err(|e| JsonRpcError::internal_error(e.to_string()))
    }

    #[cfg(feature = "tools")]
    async fn handle_tools_list(&self) -> Result<Value, JsonRpcError> {
        let tools = self.tool_registry.list();
        let result = ToolsListResult { tools };
        serde_json::to_value(result).map_err(|e| JsonRpcError::internal_error(e.to_string()))
    }

    #[cfg(feature = "tools")]
    async fn handle_tool_call(
        &self,
        params: Option<Value>,
        ctx: &Context,
    ) -> Result<Value, JsonRpcError> {
        let params = params.ok_or_else(|| JsonRpcError::invalid_params("Missing params"))?;

        let call_params: ToolCallParams = serde_json::from_value(params)
            .map_err(|e| JsonRpcError::invalid_params(format!("Invalid params: {}", e)))?;

        let result = self
            .tool_registry
            .call(&call_params.name, call_params.arguments, ctx)
            .await;

        match result {
            Ok(tool_result) => {
                serde_json::to_value(tool_result).map_err(|e| JsonRpcError::internal_error(e.to_string()))
            }
            Err(e) => {
                let error_result = ToolResult {
                    content: vec![crate::protocol::Content::text(e.to_string())],
                    is_error: Some(true),
                };
                serde_json::to_value(error_result)
                    .map_err(|e| JsonRpcError::internal_error(e.to_string()))
            }
        }
    }

    #[cfg(feature = "resources")]
    async fn handle_resources_list(&self) -> Result<Value, JsonRpcError> {
        let resources = self.resource_registry.list();
        let result = ResourcesListResult { resources };
        serde_json::to_value(result).map_err(|e| JsonRpcError::internal_error(e.to_string()))
    }

    #[cfg(feature = "resources")]
    async fn handle_resources_read(
        &self,
        params: Option<Value>,
        ctx: &Context,
    ) -> Result<Value, JsonRpcError> {
        let params = params.ok_or_else(|| JsonRpcError::invalid_params("Missing params"))?;

        let read_params: ResourceReadParams = serde_json::from_value(params)
            .map_err(|e| JsonRpcError::invalid_params(format!("Invalid params: {}", e)))?;

        let content = self
            .resource_registry
            .read(&read_params.uri, ctx)
            .await
            .map_err(|e| JsonRpcError::internal_error(e.to_string()))?;

        let result = ResourceReadResult {
            contents: vec![content],
        };

        serde_json::to_value(result).map_err(|e| JsonRpcError::internal_error(e.to_string()))
    }

    #[cfg(feature = "prompts")]
    async fn handle_prompts_list(&self) -> Result<Value, JsonRpcError> {
        let prompts = self.prompt_registry.list();
        let result = PromptsListResult { prompts };
        serde_json::to_value(result).map_err(|e| JsonRpcError::internal_error(e.to_string()))
    }

    #[cfg(feature = "prompts")]
    async fn handle_prompts_get(
        &self,
        params: Option<Value>,
        ctx: &Context,
    ) -> Result<Value, JsonRpcError> {
        let params = params.ok_or_else(|| JsonRpcError::invalid_params("Missing params"))?;

        let get_params: PromptGetParams = serde_json::from_value(params)
            .map_err(|e| JsonRpcError::invalid_params(format!("Invalid params: {}", e)))?;

        let messages = self
            .prompt_registry
            .get(&get_params.name, get_params.arguments, ctx)
            .await
            .map_err(|e| JsonRpcError::internal_error(e.to_string()))?;

        let result = PromptGetResult { messages };

        serde_json::to_value(result).map_err(|e| JsonRpcError::internal_error(e.to_string()))
    }
}

// Request/Response parameter structures

#[derive(Debug, Deserialize)]
#[allow(dead_code)]
struct InitializeParams {
    #[serde(rename = "protocolVersion")]
    protocol_version: Option<String>,
    capabilities: Option<crate::protocol::ClientCapabilities>,
    #[serde(rename = "clientInfo")]
    client_info: Option<ClientInfo>,
}

#[derive(Debug, Deserialize)]
#[allow(dead_code)]
struct ClientInfo {
    name: String,
    version: String,
}

#[cfg(feature = "tools")]
#[derive(Debug, Serialize)]
struct ToolsListResult {
    tools: Vec<ToolInfo>,
}

#[cfg(feature = "tools")]
#[derive(Debug, Deserialize)]
struct ToolCallParams {
    name: String,
    #[serde(default)]
    arguments: Value,
}

#[cfg(feature = "resources")]
#[derive(Debug, Serialize)]
struct ResourcesListResult {
    resources: Vec<crate::resources::ResourceInfo>,
}

#[cfg(feature = "resources")]
#[derive(Debug, Deserialize)]
struct ResourceReadParams {
    uri: String,
}

#[cfg(feature = "resources")]
#[derive(Debug, Serialize)]
struct ResourceReadResult {
    contents: Vec<crate::resources::ResourceContent>,
}

#[cfg(feature = "prompts")]
#[derive(Debug, Serialize)]
struct PromptsListResult {
    prompts: Vec<crate::prompts::PromptInfo>,
}

#[cfg(feature = "prompts")]
#[derive(Debug, Deserialize)]
struct PromptGetParams {
    name: String,
    #[serde(default)]
    arguments: Option<Value>,
}

#[cfg(feature = "prompts")]
#[derive(Debug, Serialize)]
struct PromptGetResult {
    messages: Vec<crate::prompts::PromptMessage>,
}
