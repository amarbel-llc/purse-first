use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashMap;

use super::capabilities::Capabilities;

/// Server information
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ServerInfo {
    pub name: String,
    pub version: String,

    /// Human-readable context about the implementation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
}

/// Initialize result
#[derive(Debug, Serialize)]
pub struct InitializeResult {
    #[serde(rename = "protocolVersion")]
    pub protocol_version: String,
    pub capabilities: Capabilities,
    #[serde(rename = "serverInfo")]
    pub server_info: ServerInfo,
}

/// Client capabilities
#[derive(Debug, Deserialize, Clone, Default)]
pub struct ClientCapabilities {
    #[serde(default)]
    pub experimental: HashMap<String, Value>,

    #[cfg(feature = "sampling")]
    #[serde(default)]
    pub sampling: Option<ClientSamplingCapability>,
}

#[derive(Debug, Deserialize, Clone)]
pub struct ClientSamplingCapability {}
