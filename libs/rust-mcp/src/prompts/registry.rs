use super::handler::{Prompt, PromptError, PromptInfo, PromptMessage};
use super::handler_v1::{PromptInfoV1, PromptMessageV1, PromptV1};
use crate::protocol::content_v1::ContentV1;
use crate::server::Context;
use serde_json::Value;
use std::collections::HashMap;
use std::sync::Arc;

/// Registry for prompts
pub struct PromptRegistry {
    prompts: HashMap<String, Arc<dyn Prompt>>,
    prompts_v1: HashMap<String, Arc<dyn PromptV1>>,
}

impl PromptRegistry {
    pub fn new() -> Self {
        PromptRegistry {
            prompts: HashMap::new(),
            prompts_v1: HashMap::new(),
        }
    }

    /// Register a prompt
    pub fn register<P: Prompt + 'static>(&mut self, prompt: P) {
        let name = prompt.name().to_string();
        self.prompts.insert(name, Arc::new(prompt));
    }

    /// Register a V1 prompt (also registered as a V0 prompt)
    pub fn register_v1<P: PromptV1 + 'static>(&mut self, prompt: P) {
        let name = prompt.name().to_string();
        let arc: Arc<P> = Arc::new(prompt);
        self.prompts.insert(name.clone(), arc.clone());
        self.prompts_v1.insert(name, arc);
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

    /// List all registered prompts with V1 metadata.
    pub fn list_v1(&self) -> Vec<PromptInfoV1> {
        self.prompts
            .iter()
            .map(|(name, prompt)| {
                if let Some(v1_prompt) = self.prompts_v1.get(name) {
                    v1_prompt.prompt_info_v1()
                } else {
                    PromptInfoV1 {
                        name: prompt.name().to_string(),
                        title: None,
                        description: prompt.description().to_string(),
                        icons: None,
                        arguments_schema: prompt.arguments_schema(),
                    }
                }
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

    /// Get prompt messages by name with V1 content.
    pub async fn get_v1(
        &self,
        name: &str,
        arguments: Option<Value>,
        ctx: &Context,
    ) -> Result<Vec<PromptMessageV1>, PromptError> {
        if let Some(v1_prompt) = self.prompts_v1.get(name) {
            return v1_prompt.get_messages_v1(arguments, ctx).await;
        }

        let prompt = self
            .prompts
            .get(name)
            .ok_or_else(|| PromptError::NotFound(format!("Unknown prompt: {}", name)))?;

        let v0_messages = prompt.get_messages(arguments, ctx).await?;
        Ok(v0_messages
            .into_iter()
            .map(|m| PromptMessageV1 {
                role: m.role,
                content: ContentV1::from_v0(m.content),
            })
            .collect())
    }
}

impl Default for PromptRegistry {
    fn default() -> Self {
        Self::new()
    }
}
