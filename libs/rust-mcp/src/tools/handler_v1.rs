use crate::protocol::content_v1::ContentV1;
use crate::protocol::icons::Icon;
use crate::server::Context;
use async_trait::async_trait;
use serde::Serialize;
use serde_json::Value;

use super::handler::{Tool, ToolError};

/// V1 tool annotations providing hints about tool behavior.
#[derive(Debug, Clone, Serialize)]
pub struct ToolAnnotations {
    /// Human-readable display name.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,

    /// Indicates the tool does not modify state.
    #[serde(rename = "readOnlyHint", skip_serializing_if = "Option::is_none")]
    pub read_only_hint: Option<bool>,

    /// Indicates the tool may perform destructive operations.
    #[serde(rename = "destructiveHint", skip_serializing_if = "Option::is_none")]
    pub destructive_hint: Option<bool>,

    /// Indicates repeated calls with same args have no additional effect.
    #[serde(rename = "idempotentHint", skip_serializing_if = "Option::is_none")]
    pub idempotent_hint: Option<bool>,

    /// Indicates the tool interacts with external entities.
    #[serde(rename = "openWorldHint", skip_serializing_if = "Option::is_none")]
    pub open_world_hint: Option<bool>,
}

/// V1 tool result with structured content support.
#[derive(Debug, Serialize)]
pub struct ToolResultV1 {
    /// Unstructured content output.
    pub content: Vec<ContentV1>,

    /// Structured content output.
    #[serde(rename = "structuredContent", skip_serializing_if = "Option::is_none")]
    pub structured_content: Option<Value>,

    /// Whether the tool execution failed.
    #[serde(skip_serializing_if = "Option::is_none", rename = "isError")]
    pub is_error: Option<bool>,
}

impl ToolResultV1 {
    pub fn text(text: impl Into<String>) -> Self {
        ToolResultV1 {
            content: vec![ContentV1::text(text)],
            structured_content: None,
            is_error: None,
        }
    }

    pub fn error(message: impl Into<String>) -> Self {
        ToolResultV1 {
            content: vec![ContentV1::text(message)],
            structured_content: None,
            is_error: Some(true),
        }
    }

    pub fn structured(content_text: impl Into<String>, structured: Value) -> Self {
        ToolResultV1 {
            content: vec![ContentV1::text(content_text)],
            structured_content: Some(structured),
            is_error: None,
        }
    }
}

/// V1 tool information for listing.
#[derive(Debug, Serialize)]
pub struct ToolInfoV1 {
    pub name: String,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,

    pub description: String,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub icons: Option<Vec<Icon>>,

    #[serde(rename = "inputSchema")]
    pub input_schema: Value,

    #[serde(rename = "outputSchema", skip_serializing_if = "Option::is_none")]
    pub output_schema: Option<Value>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub annotations: Option<ToolAnnotations>,
}

/// V1 Tool trait extending the base Tool trait.
/// Provides default implementations that delegate to V0 methods.
#[async_trait]
pub trait ToolV1: Tool {
    /// Human-readable display name.
    fn title(&self) -> Option<&str> {
        None
    }

    /// Visual icons for display.
    fn icons(&self) -> Option<Vec<Icon>> {
        None
    }

    /// JSON schema for output validation.
    fn output_schema(&self) -> Option<Value> {
        None
    }

    /// Tool behavior annotations.
    fn annotations(&self) -> Option<ToolAnnotations> {
        None
    }

    /// Execute the tool with V1 result type.
    /// Default implementation delegates to the V0 execute method.
    async fn execute_v1(
        &self,
        arguments: Value,
        ctx: &Context,
    ) -> Result<ToolResultV1, ToolError> {
        let v0_result = self.execute(arguments, ctx).await?;
        Ok(ToolResultV1 {
            content: v0_result
                .content
                .into_iter()
                .map(ContentV1::from_v0)
                .collect(),
            structured_content: None,
            is_error: v0_result.is_error,
        })
    }

    /// Build V1 tool info for listing.
    fn tool_info_v1(&self) -> ToolInfoV1 {
        ToolInfoV1 {
            name: self.name().to_string(),
            title: self.title().map(|s| s.to_string()),
            description: self.description().to_string(),
            icons: self.icons(),
            input_schema: self.input_schema(),
            output_schema: self.output_schema(),
            annotations: self.annotations(),
        }
    }
}

// Any Tool automatically satisfies ToolV1 with default implementations.
// Implementors can override specific V1 methods as needed.
