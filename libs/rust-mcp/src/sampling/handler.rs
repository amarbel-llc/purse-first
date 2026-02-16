use super::request::{CreateMessageRequest, CreateMessageResult, SamplingError};
use async_trait::async_trait;

/// Sampling handler trait - implement this to handle sampling requests from tools
#[async_trait]
pub trait SamplingHandler: Send + Sync {
    /// Handle a sampling request to create a message
    async fn create_message(
        &self,
        request: CreateMessageRequest,
    ) -> Result<CreateMessageResult, SamplingError>;
}
