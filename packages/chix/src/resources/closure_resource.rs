use async_trait::async_trait;
use mcp_server::resources::{Resource, ResourceContent, ResourceError};
use mcp_server::server::Context;

use super::parse_nix_uri;

pub struct ClosureResource;

#[async_trait]
impl Resource for ClosureResource {
    fn uri_template(&self) -> &str {
        "nix://closure/{store-path}"
    }

    fn name(&self) -> &str {
        "Store Closure"
    }

    fn description(&self) -> &str {
        "Access closure information for a store path. Query params: offset, limit"
    }

    fn mime_type(&self) -> &str {
        "application/json"
    }

    async fn read(&self, uri: &str, _ctx: &Context) -> Result<ResourceContent, ResourceError> {
        let parsed = parse_nix_uri(uri).map_err(ResourceError::InvalidUri)?;
        let result = super::read_closure(&parsed)
            .await
            .map_err(ResourceError::ReadFailed)?;

        Ok(ResourceContent {
            uri: result.uri,
            mime_type: result.mime_type,
            text: result.text,
        })
    }
}
