pub mod context;
pub mod dispatcher;
pub mod stdio;

#[cfg(feature = "http-transport")]
pub mod http;

pub use context::Context;
pub use dispatcher::McpServer;
pub use stdio::run_stdio_server;

#[cfg(feature = "http-transport")]
pub use http::run_http_server;

use crate::protocol::{Capabilities, ServerInfo, PROTOCOL_VERSION_V0};
use crate::protocol::capabilities_v1::CapabilitiesV1;

#[cfg(feature = "tools")]
use crate::tools::ToolRegistry;

#[cfg(feature = "resources")]
use crate::resources::ResourceRegistry;

#[cfg(feature = "prompts")]
use crate::prompts::PromptRegistry;

#[cfg(feature = "sampling")]
use crate::sampling::SamplingHandler;

#[cfg(feature = "completions")]
use crate::completions::CompletionRegistry;

/// Builder for MCP server
pub struct McpServerBuilder {
    name: String,
    version: String,
    protocol_version: String,
    instructions: Option<String>,
    enable_v1: bool,

    #[cfg(feature = "tools")]
    tool_registry: ToolRegistry,

    #[cfg(feature = "resources")]
    resource_registry: ResourceRegistry,

    #[cfg(feature = "prompts")]
    prompt_registry: PromptRegistry,

    #[cfg(feature = "sampling")]
    sampling_handler: Option<Arc<dyn SamplingHandler>>,

    #[cfg(feature = "completions")]
    completion_registry: CompletionRegistry,
}

impl McpServerBuilder {
    pub fn new(name: impl Into<String>, version: impl Into<String>) -> Self {
        McpServerBuilder {
            name: name.into(),
            version: version.into(),
            protocol_version: PROTOCOL_VERSION_V0.to_string(),
            instructions: None,
            enable_v1: false,

            #[cfg(feature = "tools")]
            tool_registry: ToolRegistry::new(),

            #[cfg(feature = "resources")]
            resource_registry: ResourceRegistry::new(),

            #[cfg(feature = "prompts")]
            prompt_registry: PromptRegistry::new(),

            #[cfg(feature = "sampling")]
            sampling_handler: None,

            #[cfg(feature = "completions")]
            completion_registry: CompletionRegistry::new(),
        }
    }

    pub fn protocol_version(mut self, version: impl Into<String>) -> Self {
        self.protocol_version = version.into();
        self
    }

    /// Set server instructions (V1 feature, enables V1 negotiation).
    pub fn instructions(mut self, instructions: impl Into<String>) -> Self {
        self.instructions = Some(instructions.into());
        self.enable_v1 = true;
        self
    }

    #[cfg(feature = "tools")]
    pub fn with_tool<T: crate::tools::Tool + 'static>(mut self, tool: T) -> Self {
        self.tool_registry.register(tool);
        self
    }

    /// Register a V1 tool (enables V1 negotiation).
    #[cfg(feature = "tools")]
    pub fn with_tool_v1<T: crate::tools::ToolV1 + 'static>(mut self, tool: T) -> Self {
        self.tool_registry.register_v1(tool);
        self.enable_v1 = true;
        self
    }

    #[cfg(feature = "resources")]
    pub fn with_resource<R: crate::resources::Resource + 'static>(mut self, resource: R) -> Self {
        self.resource_registry.register(resource);
        self
    }

    /// Register a V1 resource (enables V1 negotiation).
    #[cfg(feature = "resources")]
    pub fn with_resource_v1<R: crate::resources::ResourceV1 + 'static>(mut self, resource: R) -> Self {
        self.resource_registry.register_v1(resource);
        self.enable_v1 = true;
        self
    }

    #[cfg(feature = "prompts")]
    pub fn with_prompt<P: crate::prompts::Prompt + 'static>(mut self, prompt: P) -> Self {
        self.prompt_registry.register(prompt);
        self
    }

    /// Register a V1 prompt (enables V1 negotiation).
    #[cfg(feature = "prompts")]
    pub fn with_prompt_v1<P: crate::prompts::PromptV1 + 'static>(mut self, prompt: P) -> Self {
        self.prompt_registry.register_v1(prompt);
        self.enable_v1 = true;
        self
    }

    #[cfg(feature = "sampling")]
    pub fn with_sampling_handler<H: SamplingHandler + 'static>(mut self, handler: H) -> Self {
        self.sampling_handler = Some(Arc::new(handler));
        self
    }

    #[cfg(feature = "completions")]
    pub fn with_completion_provider<P: crate::completions::CompletionProvider + 'static>(
        mut self,
        provider: P,
    ) -> Self {
        self.completion_registry.register(provider);
        self.enable_v1 = true;
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

        #[cfg(feature = "completions")]
        let has_completions = self.completion_registry.has_provider();

        #[cfg(not(feature = "completions"))]
        let has_completions = false;

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

        // Build V1 capabilities if any V1 features are enabled.
        let capabilities_v1 = if self.enable_v1 {
            let mut caps = CapabilitiesV1::new();
            if has_tools {
                caps = caps.with_tools();
            }
            if has_resources {
                caps = caps.with_resources();
            }
            if has_prompts {
                caps = caps.with_prompts();
            }
            if has_sampling {
                caps = caps.with_sampling();
            }
            if has_completions {
                caps = caps.with_completions();
            }
            Some(caps)
        } else {
            None
        };

        McpServer {
            server_info: ServerInfo {
                name: self.name,
                version: self.version,
            },
            protocol_version: self.protocol_version,
            capabilities,
            capabilities_v1,
            instructions: self.instructions,

            #[cfg(feature = "tools")]
            tool_registry: self.tool_registry,

            #[cfg(feature = "resources")]
            resource_registry: self.resource_registry,

            #[cfg(feature = "prompts")]
            prompt_registry: self.prompt_registry,

            #[cfg(feature = "sampling")]
            sampling_handler: self.sampling_handler,

            #[cfg(feature = "completions")]
            completion_registry: self.completion_registry,
        }
    }
}
