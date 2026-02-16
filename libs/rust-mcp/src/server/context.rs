use crate::protocol::ClientCapabilities;
use std::any::Any;
use std::collections::HashMap;
use std::sync::Arc;

/// Request context passed to tools, resources, and prompts
pub struct Context {
    /// Server name
    pub server_name: String,

    /// Server version
    pub server_version: String,

    /// Client capabilities from initialize
    pub client_capabilities: ClientCapabilities,

    /// Application-specific extensions
    extensions: HashMap<String, Arc<dyn Any + Send + Sync>>,
}

impl Context {
    pub fn new(
        server_name: impl Into<String>,
        server_version: impl Into<String>,
        client_capabilities: ClientCapabilities,
    ) -> Self {
        Context {
            server_name: server_name.into(),
            server_version: server_version.into(),
            client_capabilities,
            extensions: HashMap::new(),
        }
    }

    /// Store application-specific data in the context
    pub fn set_extension<T: Send + Sync + 'static>(&mut self, key: impl Into<String>, value: T) {
        self.extensions.insert(key.into(), Arc::new(value));
    }

    /// Retrieve application-specific data from the context
    pub fn get_extension<T: Send + Sync + 'static>(&self, key: &str) -> Option<Arc<T>> {
        self.extensions
            .get(key)
            .and_then(|any| any.clone().downcast().ok())
    }
}

impl Clone for Context {
    fn clone(&self) -> Self {
        Context {
            server_name: self.server_name.clone(),
            server_version: self.server_version.clone(),
            client_capabilities: self.client_capabilities.clone(),
            extensions: self.extensions.clone(),
        }
    }
}
