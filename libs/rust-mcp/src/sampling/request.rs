use crate::prompts::PromptMessage;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;
use thiserror::Error;

/// Error type for sampling operations
#[derive(Error, Debug)]
pub enum SamplingError {
    #[error("Sampling not supported")]
    NotSupported,

    #[error("Invalid request: {0}")]
    InvalidRequest(String),

    #[error("{0}")]
    Other(String),
}

/// Sampling message (alias for PromptMessage)
pub type SamplingMessage = PromptMessage;

/// Model preferences for sampling
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelPreferences {
    #[serde(skip_serializing_if = "Option::is_none", rename = "costPriority")]
    pub cost_priority: Option<f64>,

    #[serde(skip_serializing_if = "Option::is_none", rename = "speedPriority")]
    pub speed_priority: Option<f64>,

    #[serde(skip_serializing_if = "Option::is_none", rename = "intelligencePriority")]
    pub intelligence_priority: Option<f64>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub hints: Option<Vec<String>>,
}

/// Create message request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CreateMessageRequest {
    pub messages: Vec<SamplingMessage>,

    #[serde(skip_serializing_if = "Option::is_none", rename = "modelPreferences")]
    pub model_preferences: Option<ModelPreferences>,

    #[serde(skip_serializing_if = "Option::is_none", rename = "systemPrompt")]
    pub system_prompt: Option<String>,

    #[serde(skip_serializing_if = "Option::is_none", rename = "includeContext")]
    pub include_context: Option<String>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f64>,

    #[serde(rename = "maxTokens")]
    pub max_tokens: i32,

    #[serde(skip_serializing_if = "Option::is_none", rename = "stopSequences")]
    pub stop_sequences: Option<Vec<String>>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<HashMap<String, Value>>,
}

/// Stop reason for message generation
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum StopReason {
    EndTurn,
    StopSequence,
    MaxTokens,
}

/// Create message result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CreateMessageResult {
    pub role: crate::prompts::MessageRole,
    pub content: crate::protocol::Content,
    pub model: String,

    #[serde(skip_serializing_if = "Option::is_none", rename = "stopReason")]
    pub stop_reason: Option<StopReason>,
}
