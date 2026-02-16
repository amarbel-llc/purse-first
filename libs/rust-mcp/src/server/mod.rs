pub mod context;
pub mod dispatcher;
pub mod stdio;

pub use context::Context;
pub use dispatcher::McpServer;
pub use stdio::run_stdio_server;

use crate::protocol::{Capabilities, ServerInfo};

#[cfg(feature = "tools")]
use crate::tools::ToolRegistry;

#[cfg(feature = "resources")]
use crate::resources::ResourceRegistry;

#[cfg(feature = "prompts")]
use crate::prompts::PromptRegistry;

#[cfg(feature = "sampling")]
use crate::sampling::SamplingHandler;

/// Builder for MCP server
pub struct McpServerBuilder {
    name: String,
    version: String,
    protocol_version: String,

    #[cfg(feature = "tools")]
    tool_registry: ToolRegistry,

    #[cfg(feature = "resources")]
    resource_registry: ResourceRegistry,

    #[cfg(feature = "prompts")]
    prompt_registry: PromptRegistry,

    #[cfg(feature = "sampling")]
    sampling_handler: Option<Arc<dyn SamplingHandler>>,
}

impl McpServerBuilder {
    pub fn new(name: impl Into<String>, version: impl Into<String>) -> Self {
        McpServerBuilder {
            name: name.into(),
            version: version.into(),
            protocol_version: "2024-11-05".to_string(),

            #[cfg(feature = "tools")]
            tool_registry: ToolRegistry::new(),

            #[cfg(feature = "resources")]
            resource_registry: ResourceRegistry::new(),

            #[cfg(feature = "prompts")]
            prompt_registry: PromptRegistry::new(),

            #[cfg(feature = "sampling")]
            sampling_handler: None,
        }
    }

    pub fn protocol_version(mut self, version: impl Into<String>) -> Self {
        self.protocol_version = version.into();
        self
    }

    #[cfg(feature = "tools")]
    pub fn with_tool<T: crate::tools::Tool + 'static>(mut self, tool: T) -> Self {
        self.tool_registry.register(tool);
        self
    }

    #[cfg(feature = "resources")]
    pub fn with_resource<R: crate::resources::Resource + 'static>(mut self, resource: R) -> Self {
        self.resource_registry.register(resource);
        self
    }

    #[cfg(feature = "prompts")]
    pub fn with_prompt<P: crate::prompts::Prompt + 'static>(mut self, prompt: P) -> Self {
        self.prompt_registry.register(prompt);
        self
    }

    #[cfg(feature = "sampling")]
    pub fn with_sampling_handler<H: SamplingHandler + 'static>(mut self, handler: H) -> Self {
        self.sampling_handler = Some(Arc::new(handler));
        self
    }

    pub fn build(self) -> McpServer {
        let mut capabilities = Capabilities::new();

        #[cfg(feature = "tools")]
        let has_tools = !self.tool_registry.is_empty();

        #[cfg(not(feature = "tools"))]
        let has_tools = false;

        #[cfg(feature = "resources")]
        let has_resources = !self.resource_registry.is_empty();

        #[cfg(not(feature = "resources"))]
        let has_resources = false;

        #[cfg(feature = "prompts")]
        let has_prompts = !self.prompt_registry.is_empty();

        #[cfg(not(feature = "prompts"))]
        let has_prompts = false;

        #[cfg(feature = "sampling")]
        let has_sampling = self.sampling_handler.is_some();

        #[cfg(not(feature = "sampling"))]
        let has_sampling = false;

        if has_tools {
            #[cfg(feature = "tools")]
            {
                capabilities = capabilities.with_tools();
            }
        }

        if has_resources {
            #[cfg(feature = "resources")]
            {
                capabilities = capabilities.with_resources();
            }
        }

        if has_prompts {
            #[cfg(feature = "prompts")]
            {
                capabilities = capabilities.with_prompts();
            }
        }

        if has_sampling {
            #[cfg(feature = "sampling")]
            {
                capabilities = capabilities.with_sampling();
            }
        }

        McpServer {
            server_info: ServerInfo {
                name: self.name,
                version: self.version,
            },
            protocol_version: self.protocol_version,
            capabilities,

            #[cfg(feature = "tools")]
            tool_registry: self.tool_registry,

            #[cfg(feature = "resources")]
            resource_registry: self.resource_registry,

            #[cfg(feature = "prompts")]
            prompt_registry: self.prompt_registry,

            #[cfg(feature = "sampling")]
            sampling_handler: self.sampling_handler,
        }
    }
}
