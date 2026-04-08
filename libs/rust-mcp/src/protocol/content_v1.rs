use serde::{Deserialize, Serialize};

use super::content_v0::{Content as ContentV0, ResourceContents};

/// ContentAnnotations provides metadata about content blocks.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContentAnnotations {
    /// Intended recipients of the content.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audience: Option<Vec<String>>,

    /// Importance level (0.0 to 1.0).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub priority: Option<f64>,

    /// ISO 8601 timestamp of the last modification.
    #[serde(rename = "lastModified", skip_serializing_if = "Option::is_none")]
    pub last_modified: Option<String>,
}

/// V1 content type for MCP messages with annotations and new content types.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ContentV1 {
    /// Plain text content.
    Text {
        text: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        annotations: Option<ContentAnnotations>,
    },
    /// Base64-encoded image content.
    Image {
        data: String,
        #[serde(rename = "mimeType")]
        mime_type: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        annotations: Option<ContentAnnotations>,
    },
    /// Base64-encoded audio content.
    Audio {
        data: String,
        #[serde(rename = "mimeType")]
        mime_type: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        annotations: Option<ContentAnnotations>,
    },
    /// Embedded resource content. Per the MCP spec, the resource contents
    /// nest under a `resource` field rather than being flattened beside
    /// `type`.
    Resource {
        resource: ResourceContents,
        #[serde(skip_serializing_if = "Option::is_none")]
        annotations: Option<ContentAnnotations>,
    },
    /// Resource link content.
    #[serde(rename = "resource_link")]
    ResourceLink {
        uri: String,
        name: String,
        #[serde(skip_serializing_if = "Option::is_none")]
        description: Option<String>,
        #[serde(rename = "mimeType", skip_serializing_if = "Option::is_none")]
        mime_type: Option<String>,
        #[serde(skip_serializing_if = "Option::is_none")]
        annotations: Option<ContentAnnotations>,
    },
}

impl ContentV1 {
    pub fn text(text: impl Into<String>) -> Self {
        ContentV1::Text {
            text: text.into(),
            annotations: None,
        }
    }

    pub fn image(data: impl Into<String>, mime_type: impl Into<String>) -> Self {
        ContentV1::Image {
            data: data.into(),
            mime_type: mime_type.into(),
            annotations: None,
        }
    }

    pub fn audio(data: impl Into<String>, mime_type: impl Into<String>) -> Self {
        ContentV1::Audio {
            data: data.into(),
            mime_type: mime_type.into(),
            annotations: None,
        }
    }

    pub fn resource_link(
        uri: impl Into<String>,
        name: impl Into<String>,
        description: Option<String>,
        mime_type: Option<String>,
    ) -> Self {
        ContentV1::ResourceLink {
            uri: uri.into(),
            name: name.into(),
            description,
            mime_type,
            annotations: None,
        }
    }

    /// Convert from a V0 Content to a V1 ContentV1 (lossless upcast).
    pub fn from_v0(v0: ContentV0) -> Self {
        match v0 {
            ContentV0::Text { text } => ContentV1::Text {
                text,
                annotations: None,
            },
            ContentV0::Image {
                data, mime_type, ..
            } => ContentV1::Image {
                data,
                mime_type,
                annotations: None,
            },
            ContentV0::Resource { resource } => ContentV1::Resource {
                resource,
                annotations: None,
            },
        }
    }
}
