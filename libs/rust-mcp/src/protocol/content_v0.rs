use serde::{Deserialize, Serialize};

/// Embedded resource contents, matching the MCP spec's `TextResourceContents`.
///
/// Serialized as the value of the `resource` field on a `Content::Resource`
/// block.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResourceContents {
    pub uri: String,
    #[serde(rename = "mimeType")]
    pub mime_type: String,
    pub text: String,
}

/// Content type for MCP messages
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum Content {
    Text {
        text: String,
    },
    Image {
        data: String,
        #[serde(rename = "mimeType")]
        mime_type: String,
    },
    /// Embedded resource. Per the MCP spec, the resource contents nest under
    /// a `resource` field rather than being flattened beside `type`.
    Resource {
        resource: ResourceContents,
    },
}

/// Content type discriminator
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ContentType {
    Text,
    Image,
    Resource,
}

impl Content {
    pub fn text(text: impl Into<String>) -> Self {
        Content::Text { text: text.into() }
    }

    pub fn image(data: impl Into<String>, mime_type: impl Into<String>) -> Self {
        Content::Image {
            data: data.into(),
            mime_type: mime_type.into(),
        }
    }

    pub fn resource(
        uri: impl Into<String>,
        mime_type: impl Into<String>,
        text: impl Into<String>,
    ) -> Self {
        Content::Resource {
            resource: ResourceContents {
                uri: uri.into(),
                mime_type: mime_type.into(),
                text: text.into(),
            },
        }
    }

    pub fn content_type(&self) -> ContentType {
        match self {
            Content::Text { .. } => ContentType::Text,
            Content::Image { .. } => ContentType::Image,
            Content::Resource { .. } => ContentType::Resource,
        }
    }
}
