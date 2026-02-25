use super::handler::{Tool, ToolError, ToolInfo, ToolResult};
use super::handler_v1::{ToolInfoV1, ToolResultV1, ToolV1};
use crate::protocol::content_v1::ContentV1;
use crate::server::Context;
use serde_json::Value;
use std::collections::HashMap;
use std::sync::Arc;

/// Registry for tools
pub struct ToolRegistry {
    tools: HashMap<String, Arc<dyn Tool>>,
    tools_v1: HashMap<String, Arc<dyn ToolV1>>,
}

impl ToolRegistry {
    pub fn new() -> Self {
        ToolRegistry {
            tools: HashMap::new(),
            tools_v1: HashMap::new(),
        }
    }

    /// Register a tool
    pub fn register<T: Tool + 'static>(&mut self, tool: T) {
        let name = tool.name().to_string();
        self.tools.insert(name, Arc::new(tool));
    }

    /// Register a V1 tool (also registered as a V0 tool)
    pub fn register_v1<T: ToolV1 + 'static>(&mut self, tool: T) {
        let name = tool.name().to_string();
        let arc: Arc<T> = Arc::new(tool);
        self.tools.insert(name.clone(), arc.clone());
        self.tools_v1.insert(name, arc);
    }

    /// Check if registry is empty
    pub fn is_empty(&self) -> bool {
        self.tools.is_empty()
    }

    /// List all registered tools
    pub fn list(&self) -> Vec<ToolInfo> {
        self.tools
            .values()
            .map(|tool| ToolInfo {
                name: tool.name().to_string(),
                description: tool.description().to_string(),
                input_schema: tool.input_schema(),
            })
            .collect()
    }

    /// List all registered tools with V1 metadata.
    /// Tools registered via register_v1 use their V1 info; others get default V1 info.
    pub fn list_v1(&self) -> Vec<ToolInfoV1> {
        self.tools
            .iter()
            .map(|(name, tool)| {
                if let Some(v1_tool) = self.tools_v1.get(name) {
                    v1_tool.tool_info_v1()
                } else {
                    ToolInfoV1 {
                        name: tool.name().to_string(),
                        title: None,
                        description: tool.description().to_string(),
                        icons: None,
                        input_schema: tool.input_schema(),
                        output_schema: None,
                        annotations: None,
                        execution: None,
                    }
                }
            })
            .collect()
    }

    /// Call a tool by name
    pub async fn call(
        &self,
        name: &str,
        arguments: Value,
        ctx: &Context,
    ) -> Result<ToolResult, ToolError> {
        let tool = self
            .tools
            .get(name)
            .ok_or_else(|| ToolError::Other(format!("Unknown tool: {}", name)))?;

        tool.execute(arguments, ctx).await
    }

    /// Call a tool by name with V1 result type.
    /// V1-registered tools use their execute_v1; others delegate through V0.
    pub async fn call_v1(
        &self,
        name: &str,
        arguments: Value,
        ctx: &Context,
    ) -> Result<ToolResultV1, ToolError> {
        if let Some(v1_tool) = self.tools_v1.get(name) {
            return v1_tool.execute_v1(arguments, ctx).await;
        }

        let tool = self
            .tools
            .get(name)
            .ok_or_else(|| ToolError::Other(format!("Unknown tool: {}", name)))?;

        let v0_result = tool.execute(arguments, ctx).await?;
        Ok(ToolResultV1 {
            content: v0_result
                .content
                .into_iter()
                .map(ContentV1::from_v0)
                .collect(),
            structured_content: None,
            is_error: v0_result.is_error,
            meta: None,
        })
    }
}

impl Default for ToolRegistry {
    fn default() -> Self {
        Self::new()
    }
}
