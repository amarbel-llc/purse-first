use crate::protocol::content_v1::ContentAnnotations;
use crate::protocol::icons::Icon;
use async_trait::async_trait;
use serde::Serialize;

use super::handler::Resource;

/// V1 resource information for listing.
#[derive(Debug, Serialize)]
pub struct ResourceInfoV1 {
    pub uri: String,

    pub name: String,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,

    pub description: String,

    #[serde(rename = "mimeType")]
    pub mime_type: String,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub size: Option<i64>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub icons: Option<Vec<Icon>>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub annotations: Option<ContentAnnotations>,
}

/// V1 Resource trait extending the base Resource trait.
#[async_trait]
pub trait ResourceV1: Resource {
    /// Human-readable display name.
    fn title(&self) -> Option<&str> {
        None
    }

    /// Resource size in bytes.
    fn size(&self) -> Option<i64> {
        None
    }

    /// Visual icons for display.
    fn icons(&self) -> Option<Vec<Icon>> {
        None
    }

    /// Content annotations.
    fn annotations(&self) -> Option<ContentAnnotations> {
        None
    }

    /// Build V1 resource info for listing.
    fn resource_info_v1(&self) -> ResourceInfoV1 {
        ResourceInfoV1 {
            uri: self.uri_template().to_string(),
            name: self.name().to_string(),
            title: self.title().map(|s| s.to_string()),
            description: self.description().to_string(),
            mime_type: self.mime_type().to_string(),
            size: self.size(),
            icons: self.icons(),
            annotations: self.annotations(),
        }
    }
}
