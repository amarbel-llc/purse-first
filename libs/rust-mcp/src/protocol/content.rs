use serde::{Deserialize, Serialize};

/// Content type for MCP messages
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum Content {
    Text { text: String },
    Image {
        data: String,
        #[serde(rename = "mimeType")]
        mime_type: String,
    },
    Resource {
        uri: String,
        #[serde(rename = "mimeType")]
        mime_type: String,
        text: String,
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

    pub fn resource(uri: impl Into<String>, mime_type: impl Into<String>, text: impl Into<String>) -> Self {
        Content::Resource {
            uri: uri.into(),
            mime_type: mime_type.into(),
            text: text.into(),
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
