use serde::{Deserialize, Serialize};

use super::capabilities_v1::CapabilitiesV1;
use super::icons::Icon;

/// V1 server information with extended metadata.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ServerInfoV1 {
    pub name: String,
    pub version: String,

    /// Human-readable display name.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,

    /// Purpose explanation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,

    /// Branded images for display.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub icons: Option<Vec<Icon>>,

    /// Link to documentation.
    #[serde(rename = "websiteUrl", skip_serializing_if = "Option::is_none")]
    pub website_url: Option<String>,
}

/// V1 initialize result with instructions.
#[derive(Debug, Serialize)]
pub struct InitializeResultV1 {
    #[serde(rename = "protocolVersion")]
    pub protocol_version: String,

    pub capabilities: CapabilitiesV1,

    #[serde(rename = "serverInfo")]
    pub server_info: ServerInfoV1,

    /// Server usage hints.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub instructions: Option<String>,
}
