use crate::protocol::Content;
use crate::server::Context;
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use thiserror::Error;

/// Error type for tool execution
#[derive(Error, Debug)]
pub enum ToolError {
    #[error("Invalid arguments: {0}")]
    InvalidArguments(String),

    #[error("Execution failed: {0}")]
    ExecutionFailed(String),

    #[error("Serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("{0}")]
    Other(String),
}

impl From<String> for ToolError {
    fn from(s: String) -> Self {
        ToolError::Other(s)
    }
}

impl From<&str> for ToolError {
    fn from(s: &str) -> Self {
        ToolError::Other(s.to_string())
    }
}

/// Result of tool execution
#[derive(Debug, Serialize)]
pub struct ToolResult {
    pub content: Vec<Content>,
    #[serde(skip_serializing_if = "Option::is_none", rename = "isError")]
    pub is_error: Option<bool>,
}

impl ToolResult {
    pub fn text(text: impl Into<String>) -> Self {
        ToolResult {
            content: vec![Content::text(text)],
            is_error: None,
        }
    }

    pub fn error(message: impl Into<String>) -> Self {
        ToolResult {
            content: vec![Content::text(message)],
            is_error: Some(true),
        }
    }

    pub fn with_content(content: Vec<Content>) -> Self {
        ToolResult {
            content,
            is_error: None,
        }
    }
}

/// Tool trait - implement this to create a tool
#[async_trait]
pub trait Tool: Send + Sync {
    /// Unique name for the tool
    fn name(&self) -> &str;

    /// Human-readable description
    fn description(&self) -> &str;

    /// JSON schema for input parameters
    fn input_schema(&self) -> Value;

    /// Execute the tool with the given arguments
    async fn execute(&self, arguments: Value, ctx: &Context) -> Result<ToolResult, ToolError>;
}

/// Tool information for listing
#[derive(Debug, Serialize, Deserialize)]
pub struct ToolInfo {
    pub name: String,
    pub description: String,
    #[serde(rename = "inputSchema")]
    pub input_schema: Value,
}
