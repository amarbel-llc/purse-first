use crate::protocol::completions::{CompletionCompleteParams, CompletionResult};
use crate::server::Context;
use async_trait::async_trait;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum CompletionError {
    #[error("Completion failed: {0}")]
    Failed(String),

    #[error("{0}")]
    Other(String),
}

impl From<String> for CompletionError {
    fn from(s: String) -> Self {
        CompletionError::Other(s)
    }
}

#[async_trait]
pub trait CompletionProvider: Send + Sync {
    async fn complete(
        &self,
        params: CompletionCompleteParams,
        ctx: &Context,
    ) -> Result<CompletionResult, CompletionError>;
}
