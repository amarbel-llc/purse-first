use crate::protocol::content_v1::ContentV1;
use crate::protocol::icons::Icon;
use crate::server::Context;
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use serde_json::Value;

use super::handler::{MessageRole, Prompt, PromptError};

/// V1 prompt message using V1 content blocks.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PromptMessageV1 {
    pub role: MessageRole,
    pub content: ContentV1,
}

/// V1 prompt information for listing.
#[derive(Debug, Serialize)]
pub struct PromptInfoV1 {
    pub name: String,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,

    pub description: String,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub icons: Option<Vec<Icon>>,

    #[serde(skip_serializing_if = "Option::is_none", rename = "arguments")]
    pub arguments_schema: Option<Value>,
}

/// V1 Prompt trait extending the base Prompt trait.
#[async_trait]
pub trait PromptV1: Prompt {
    /// Human-readable display name.
    fn title(&self) -> Option<&str> {
        None
    }

    /// Visual icons for display.
    fn icons(&self) -> Option<Vec<Icon>> {
        None
    }

    /// Generate V1 prompt messages.
    /// Default implementation delegates to V0 and upgrades content.
    async fn get_messages_v1(
        &self,
        arguments: Option<Value>,
        ctx: &Context,
    ) -> Result<Vec<PromptMessageV1>, PromptError> {
        let v0_messages = self.get_messages(arguments, ctx).await?;
        Ok(v0_messages
            .into_iter()
            .map(|m| PromptMessageV1 {
                role: m.role,
                content: ContentV1::from_v0(m.content),
            })
            .collect())
    }

    /// Build V1 prompt info for listing.
    fn prompt_info_v1(&self) -> PromptInfoV1 {
        PromptInfoV1 {
            name: self.name().to_string(),
            title: self.title().map(|s| s.to_string()),
            description: self.description().to_string(),
            icons: self.icons(),
            arguments_schema: self.arguments_schema(),
        }
    }
}
