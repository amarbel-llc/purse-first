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
        self.tasks = Some(TasksCapability {
            list: None,
            cancel: None,
            requests: None,
        });
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

/// Tasks capability indicating support for task-augmented requests.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksCapability {
    /// Support for the tasks/list operation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub list: Option<TasksListCapability>,

    /// Support for the tasks/cancel operation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cancel: Option<TasksCancelCapability>,

    /// Which request types support task augmentation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub requests: Option<TasksRequestsCapability>,
}

/// Marker for tasks/list support.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksListCapability {}

/// Marker for tasks/cancel support.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksCancelCapability {}

/// Which request types support task augmentation.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksRequestsCapability {
    /// Tool operations that support tasks.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tools: Option<TasksToolsCapability>,

    /// Sampling operations that support tasks (client-side).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sampling: Option<TasksSamplingCapability>,

    /// Elicitation operations that support tasks (client-side).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub elicitation: Option<TasksElicitationCapability>,
}

/// Tool operations that support tasks.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksToolsCapability {
    /// tools/call supports task augmentation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub call: Option<TasksCallCapability>,
}

/// Sampling operations that support tasks.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksSamplingCapability {
    /// sampling/createMessage supports task augmentation.
    #[serde(rename = "createMessage", skip_serializing_if = "Option::is_none")]
    pub create_message: Option<TasksCreateMessageCapability>,
}

/// Elicitation operations that support tasks.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksElicitationCapability {
    /// elicitation/create supports task augmentation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub create: Option<TasksCreateCapability>,
}

/// Marker for tools/call task support.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksCallCapability {}

/// Marker for sampling/createMessage task support.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksCreateMessageCapability {}

/// Marker for elicitation/create task support.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct TasksCreateCapability {}

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

/// Elicitation capability.
/// An empty object is equivalent to form-only support for backwards compatibility.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ElicitationCapability {
    /// Support for form-based elicitation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub form: Option<ElicitationFormCapability>,

    /// Support for URL-based elicitation.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub url: Option<ElicitationURLCapability>,
}

/// Form-based elicitation capability marker.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ElicitationFormCapability {}

/// URL-based elicitation capability marker.
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ElicitationURLCapability {}
