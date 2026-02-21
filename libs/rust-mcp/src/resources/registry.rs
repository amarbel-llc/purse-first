use super::handler::{Resource, ResourceContent, ResourceError, ResourceInfo};
use super::handler_v1::{ResourceInfoV1, ResourceV1};
use crate::server::Context;
use std::sync::Arc;

/// Registry for resources
pub struct ResourceRegistry {
    resources: Vec<Arc<dyn Resource>>,
    resources_v1: Vec<Arc<dyn ResourceV1>>,
}

impl ResourceRegistry {
    pub fn new() -> Self {
        ResourceRegistry {
            resources: Vec::new(),
            resources_v1: Vec::new(),
        }
    }

    /// Register a resource
    pub fn register<R: Resource + 'static>(&mut self, resource: R) {
        self.resources.push(Arc::new(resource));
    }

    /// Register a V1 resource (also registered as a V0 resource)
    pub fn register_v1<R: ResourceV1 + 'static>(&mut self, resource: R) {
        let arc: Arc<R> = Arc::new(resource);
        self.resources.push(arc.clone());
        self.resources_v1.push(arc);
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

    /// List all registered resources with V1 metadata.
    pub fn list_v1(&self) -> Vec<ResourceInfoV1> {
        // Build a set of V1-registered URIs for lookup.
        let v1_uris: std::collections::HashSet<String> = self
            .resources_v1
            .iter()
            .map(|r| r.uri_template().to_string())
            .collect();

        let mut result: Vec<ResourceInfoV1> = Vec::new();

        // Add V1-registered resources with full V1 info.
        for r in &self.resources_v1 {
            result.push(r.resource_info_v1());
        }

        // Add non-V1 resources with default V1 info.
        for r in &self.resources {
            let uri = r.uri_template().to_string();
            if !v1_uris.contains(&uri) {
                result.push(ResourceInfoV1 {
                    uri,
                    name: r.name().to_string(),
                    title: None,
                    description: r.description().to_string(),
                    mime_type: r.mime_type().to_string(),
                    size: None,
                    icons: None,
                    annotations: None,
                });
            }
        }

        result
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
