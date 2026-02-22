use async_trait::async_trait;
use mcp_server::resources::{Resource, ResourceContent, ResourceError};
use mcp_server::server::Context;

use super::parse_nix_uri;

pub struct BuildLogResource;

#[async_trait]
impl Resource for BuildLogResource {
    fn uri_template(&self) -> &str {
        "nix://build-log/{store-path}"
    }

    fn name(&self) -> &str {
        "Build Log"
    }

    fn description(&self) -> &str {
        "Access build logs for a store path with pagination. Query params: offset, limit"
    }

    fn mime_type(&self) -> &str {
        "text/plain"
    }

    async fn read(&self, uri: &str, _ctx: &Context) -> Result<ResourceContent, ResourceError> {
        let parsed = parse_nix_uri(uri).map_err(ResourceError::InvalidUri)?;
        let result = super::read_build_log(&parsed)
            .await
            .map_err(ResourceError::ReadFailed)?;

        Ok(ResourceContent {
            uri: result.uri,
            mime_type: result.mime_type,
            text: result.text,
        })
    }
}
