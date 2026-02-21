use serde::{Deserialize, Serialize};

/// Server capabilities
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Capabilities {
    #[cfg(feature = "tools")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tools: Option<ToolsCapability>,

    #[cfg(feature = "resources")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub resources: Option<ResourcesCapability>,

    #[cfg(feature = "prompts")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub prompts: Option<PromptsCapability>,

    #[cfg(feature = "sampling")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sampling: Option<SamplingCapability>,
}

impl Capabilities {
    pub fn new() -> Self {
        Capabilities {
            #[cfg(feature = "tools")]
            tools: None,
            #[cfg(feature = "resources")]
            resources: None,
            #[cfg(feature = "prompts")]
            prompts: None,
            #[cfg(feature = "sampling")]
            sampling: None,
        }
    }

    #[cfg(feature = "tools")]
    pub fn with_tools(mut self) -> Self {
        self.tools = Some(ToolsCapability {
            list_changed: false,
        });
        self
    }

    #[cfg(feature = "resources")]
    pub fn with_resources(mut self) -> Self {
        self.resources = Some(ResourcesCapability {
            subscribe: false,
            list_changed: false,
        });
        self
    }

    #[cfg(feature = "prompts")]
    pub fn with_prompts(mut self) -> Self {
        self.prompts = Some(PromptsCapability {
            list_changed: false,
        });
        self
    }

    #[cfg(feature = "sampling")]
    pub fn with_sampling(mut self) -> Self {
        self.sampling = Some(SamplingCapability {});
        self
    }
}

impl Default for Capabilities {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ToolsCapability {
    #[serde(rename = "listChanged")]
    pub list_changed: bool,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ResourcesCapability {
    pub subscribe: bool,
    #[serde(rename = "listChanged")]
    pub list_changed: bool,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct PromptsCapability {
    #[serde(rename = "listChanged")]
    pub list_changed: bool,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct SamplingCapability {}
