use crate::server::Context;
use async_trait::async_trait;
use serde::Serialize;
use thiserror::Error;

/// Error type for resource operations
#[derive(Error, Debug)]
pub enum ResourceError {
    #[error("Invalid URI: {0}")]
    InvalidUri(String),

    #[error("Resource not found: {0}")]
    NotFound(String),

    #[error("Read failed: {0}")]
    ReadFailed(String),

    #[error("{0}")]
    Other(String),
}

impl From<String> for ResourceError {
    fn from(s: String) -> Self {
        ResourceError::Other(s)
    }
}

impl From<&str> for ResourceError {
    fn from(s: &str) -> Self {
        ResourceError::Other(s.to_string())
    }
}

/// Resource content
#[derive(Debug, Serialize)]
pub struct ResourceContent {
    pub uri: String,
    #[serde(rename = "mimeType")]
    pub mime_type: String,
    pub text: String,
}

/// Resource trait - implement this to create a resource
#[async_trait]
pub trait Resource: Send + Sync {
    /// URI template for this resource (e.g., "myapp://logs/{id}")
    fn uri_template(&self) -> &str;

    /// Human-readable name
    fn name(&self) -> &str;

    /// Description
    fn description(&self) -> &str;

    /// MIME type
    fn mime_type(&self) -> &str;

    /// Read the resource at the given URI
    async fn read(&self, uri: &str, ctx: &Context) -> Result<ResourceContent, ResourceError>;
}

/// Resource information for listing
#[derive(Debug, Serialize)]
pub struct ResourceInfo {
    pub uri: String,
    pub name: String,
    pub description: String,
    #[serde(rename = "mimeType")]
    pub mime_type: String,
}
