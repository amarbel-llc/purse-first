use super::handler::{Resource, ResourceContent, ResourceError, ResourceInfo};
use crate::server::Context;
use std::sync::Arc;

/// Registry for resources
pub struct ResourceRegistry {
    resources: Vec<Arc<dyn Resource>>,
}

impl ResourceRegistry {
    pub fn new() -> Self {
        ResourceRegistry {
            resources: Vec::new(),
        }
    }

    /// Register a resource
    pub fn register<R: Resource + 'static>(&mut self, resource: R) {
        self.resources.push(Arc::new(resource));
    }

    /// Check if registry is empty
    pub fn is_empty(&self) -> bool {
        self.resources.is_empty()
    }

    /// List all registered resources
    pub fn list(&self) -> Vec<ResourceInfo> {
        self.resources
            .iter()
            .map(|resource| ResourceInfo {
                uri: resource.uri_template().to_string(),
                name: resource.name().to_string(),
                description: resource.description().to_string(),
                mime_type: resource.mime_type().to_string(),
            })
            .collect()
    }

    /// Read a resource by URI
    pub async fn read(&self, uri: &str, ctx: &Context) -> Result<ResourceContent, ResourceError> {
        // Find matching resource by checking if URI matches template pattern
        // For now, simple matching - can be enhanced with URI template matching
        for resource in &self.resources {
            let template = resource.uri_template();
            // Extract scheme from both
            if let Some(uri_scheme) = uri.split("://").next() {
                if let Some(template_scheme) = template.split("://").next() {
                    if uri_scheme == template_scheme {
                        // Try to read - if it succeeds, return
                        if let Ok(content) = resource.read(uri, ctx).await {
                            return Ok(content);
                        }
                    }
                }
            }
        }

        Err(ResourceError::NotFound(format!(
            "No resource found for URI: {}",
            uri
        )))
    }
}

impl Default for ResourceRegistry {
    fn default() -> Self {
        Self::new()
    }
}
