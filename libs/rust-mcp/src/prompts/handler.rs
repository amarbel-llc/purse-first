use crate::protocol::Content;
use crate::server::Context;
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use thiserror::Error;

/// Error type for prompt operations
#[derive(Error, Debug)]
pub enum PromptError {
    #[error("Invalid arguments: {0}")]
    InvalidArguments(String),

    #[error("Prompt not found: {0}")]
    NotFound(String),

    #[error("{0}")]
    Other(String),
}

impl From<String> for PromptError {
    fn from(s: String) -> Self {
        PromptError::Other(s)
    }
}

impl From<&str> for PromptError {
    fn from(s: &str) -> Self {
        PromptError::Other(s.to_string())
    }
}

/// Message role
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum MessageRole {
    User,
    Assistant,
    System,
}

/// Prompt message
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PromptMessage {
    pub role: MessageRole,
    pub content: Content,
}

/// Prompt trait - implement this to create a prompt
#[async_trait]
pub trait Prompt: Send + Sync {
    /// Unique name for the prompt
    fn name(&self) -> &str;

    /// Human-readable description
    fn description(&self) -> &str;

    /// Optional argument schema
    fn arguments_schema(&self) -> Option<Value> {
        None
    }

    /// Generate prompt messages with given arguments
    async fn get_messages(
        &self,
        arguments: Option<Value>,
        ctx: &Context,
    ) -> Result<Vec<PromptMessage>, PromptError>;
}

/// Prompt information for listing
#[derive(Debug, Serialize)]
pub struct PromptInfo {
    pub name: String,
    pub description: String,
    #[serde(skip_serializing_if = "Option::is_none", rename = "arguments")]
    pub arguments_schema: Option<Value>,
}
