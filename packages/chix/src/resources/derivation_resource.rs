use async_trait::async_trait;
use mcp_server::resources::{Resource, ResourceContent, ResourceError};
use mcp_server::server::Context;

use super::parse_nix_uri;

pub struct DerivationResource;

#[async_trait]
impl Resource for DerivationResource {
    fn uri_template(&self) -> &str {
        "nix://derivation/{drv-path}"
    }

    fn name(&self) -> &str {
        "Derivation"
    }

    fn description(&self) -> &str {
        "Access derivation data with optional summary mode. Query params: summary (true/false), offset, limit"
    }

    fn mime_type(&self) -> &str {
        "application/json"
    }

    async fn read(&self, uri: &str, _ctx: &Context) -> Result<ResourceContent, ResourceError> {
        let parsed = parse_nix_uri(uri).map_err(ResourceError::InvalidUri)?;
        let result = super::read_derivation(&parsed)
            .await
            .map_err(ResourceError::ReadFailed)?;

        Ok(ResourceContent {
            uri: result.uri,
            mime_type: result.mime_type,
            text: result.text,
        })
    }
}
