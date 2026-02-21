use serde::{Deserialize, Serialize};

use super::capabilities_v0::{
    PromptsCapability, ResourcesCapability, SamplingCapability, ToolsCapability,
};

/// V1 server capabilities with additional features.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CapabilitiesV1 {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tools: Option<ToolsCapability>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub resources: Option<ResourcesCapability>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub prompts: Option<PromptsCapability>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub sampling: Option<SamplingCapability>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub logging: Option<LoggingCapability>,

    #[serde(rename = "completions", skip_serializing_if = "Option::is_none")]
    pub completions: Option<CompletionsCapability>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub tasks: Option<TasksCapability>,
}

impl CapabilitiesV1 {
    pub fn new() -> Self {
        CapabilitiesV1 {
            tools: None,
            resources: None,
            prompts: None,
            sampling: None,
            logging: None,
            completions: None,
            tasks: None,
        }
    }

    pub fn with_tools(mut self) -> Self {
        self.tools = Some(ToolsCapability {
            list_changed: false,
        });
        self
    }

    pub fn with_resources(mut self) -> Self {
        self.resources = Some(ResourcesCapability {
            subscribe: false,
            list_changed: false,
        });
        self
    }

    pub fn with_prompts(mut self) -> Self {
        self.prompts = Some(PromptsCapability {
            list_changed: false,
        });
        self
    }

    pub fn with_sampling(mut self) -> Self {
        self.sampling = Some(SamplingCapability {});
        self
    }

    pub fn with_logging(mut self) -> Self {
        self.logging = Some(LoggingCapability {});
        self
    }

    pub fn with_completions(mut self) -> Self {
        self.completions = Some(CompletionsCapability {});
        self
    }

    pub fn with_tasks(mut self) -> Self {
        self.tasks = Some(TasksCapability {});
        self
    }
}

impl Default for CapabilitiesV1 {
    fn default() -> Self {
        Self::new()
    }
}

/// Logging capability marker.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct LoggingCapability {}

/// Completions capability marker.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CompletionsCapability {}

/// Tasks capability marker.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksCapability {}

/// V1 client capabilities with elicitation and tasks.
#[derive(Debug, Deserialize, Clone, Default)]
pub struct ClientCapabilitiesV1 {
    #[serde(default)]
    pub experimental: std::collections::HashMap<String, serde_json::Value>,

    #[serde(default)]
    pub sampling: Option<SamplingCapability>,

    #[serde(default)]
    pub elicitation: Option<ElicitationCapability>,

    #[serde(default)]
    pub tasks: Option<TasksCapability>,
}

/// Elicitation capability marker.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ElicitationCapability {}
