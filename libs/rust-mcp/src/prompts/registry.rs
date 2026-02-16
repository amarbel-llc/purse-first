use super::handler::{Prompt, PromptError, PromptInfo, PromptMessage};
use crate::server::Context;
use serde_json::Value;
use std::collections::HashMap;
use std::sync::Arc;

/// Registry for prompts
pub struct PromptRegistry {
    prompts: HashMap<String, Arc<dyn Prompt>>,
}

impl PromptRegistry {
    pub fn new() -> Self {
        PromptRegistry {
            prompts: HashMap::new(),
        }
    }

    /// Register a prompt
    pub fn register<P: Prompt + 'static>(&mut self, prompt: P) {
        let name = prompt.name().to_string();
        self.prompts.insert(name, Arc::new(prompt));
    }

    /// Check if registry is empty
    pub fn is_empty(&self) -> bool {
        self.prompts.is_empty()
    }

    /// List all registered prompts
    pub fn list(&self) -> Vec<PromptInfo> {
        self.prompts
            .values()
            .map(|prompt| PromptInfo {
                name: prompt.name().to_string(),
                description: prompt.description().to_string(),
                arguments_schema: prompt.arguments_schema(),
            })
            .collect()
    }

    /// Get prompt messages by name
    pub async fn get(
        &self,
        name: &str,
        arguments: Option<Value>,
        ctx: &Context,
    ) -> Result<Vec<PromptMessage>, PromptError> {
        let prompt = self
            .prompts
            .get(name)
            .ok_or_else(|| PromptError::NotFound(format!("Unknown prompt: {}", name)))?;

        prompt.get_messages(arguments, ctx).await
    }
}

impl Default for PromptRegistry {
    fn default() -> Self {
        Self::new()
    }
}
