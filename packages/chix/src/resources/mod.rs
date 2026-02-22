mod build_log;
mod closure;
mod derivation;

pub use build_log::read_build_log;
pub use closure::read_closure;
pub use derivation::read_derivation;

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Resource URI format: nix://{resource_type}/{path}?{params}
/// Examples:
/// - nix://build-log/abc123-hello?offset=0&limit=1000
/// - nix://derivation/abc123-hello.drv?summary=true
/// - nix://closure/abc123-hello?offset=0&limit=100

#[derive(Debug, Serialize)]
pub struct ResourceInfo {
    pub uri: String,
    pub name: String,
    pub description: String,
    #[serde(rename = "mimeType")]
    pub mime_type: String,
}

#[derive(Debug, Serialize)]
pub struct ResourceContent {
    pub uri: String,
    #[serde(rename = "mimeType")]
    pub mime_type: String,
    pub text: String,
}

#[derive(Debug, Deserialize, Default)]
pub struct ResourceReadParams {
    pub uri: String,
}

#[derive(Debug, Clone)]
pub struct ParsedUri {
    pub resource_type: String,
    pub path: String,
    pub params: HashMap<String, String>,
}

pub fn parse_nix_uri(uri: &str) -> Result<ParsedUri, String> {
    // Expected format: nix://{type}/{path}?{params}
    if !uri.starts_with("nix://") {
        return Err(format!("Invalid URI scheme, expected nix://: {}", uri));
    }

    let rest = &uri[6..]; // Remove "nix://"

    // Split path and query params
    let (path_part, query_part) = if let Some(idx) = rest.find('?') {
        (&rest[..idx], Some(&rest[idx + 1..]))
    } else {
        (rest, None)
    };

    // Split resource type and path
    let (resource_type, path) = if let Some(idx) = path_part.find('/') {
        (&path_part[..idx], &path_part[idx + 1..])
    } else {
        return Err(format!("Invalid URI format, expected type/path: {}", uri));
    };

    // Parse query params
    let mut params = HashMap::new();
    if let Some(query) = query_part {
        for pair in query.split('&') {
            if let Some(idx) = pair.find('=') {
                let key = &pair[..idx];
                let value = &pair[idx + 1..];
                params.insert(key.to_string(), value.to_string());
            }
        }
    }

    Ok(ParsedUri {
        resource_type: resource_type.to_string(),
        path: path.to_string(),
        params,
    })
}

/// List available resource templates
pub fn list_resources() -> Vec<ResourceInfo> {
    vec![
        ResourceInfo {
            uri: "nix://build-log/{store-path}".to_string(),
            name: "Build Log".to_string(),
            description: "Access build logs for a store path with pagination. Query params: offset, limit".to_string(),
            mime_type: "text/plain".to_string(),
        },
        ResourceInfo {
            uri: "nix://derivation/{drv-path}".to_string(),
            name: "Derivation".to_string(),
            description: "Access derivation data with optional summary mode. Query params: summary (true/false), offset, limit".to_string(),
            mime_type: "application/json".to_string(),
        },
        ResourceInfo {
            uri: "nix://closure/{store-path}".to_string(),
            name: "Store Closure".to_string(),
            description: "Access closure information for a store path. Query params: offset, limit".to_string(),
            mime_type: "application/json".to_string(),
        },
    ]
}

/// Read a resource by URI
pub async fn read_resource(uri: &str) -> Result<ResourceContent, String> {
    let parsed = parse_nix_uri(uri)?;

    match parsed.resource_type.as_str() {
        "build-log" => read_build_log(&parsed).await,
        "derivation" => read_derivation(&parsed).await,
        "closure" => read_closure(&parsed).await,
        _ => Err(format!("Unknown resource type: {}", parsed.resource_type)),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_nix_uri_simple() {
        let uri = "nix://build-log/abc123-hello";
        let parsed = parse_nix_uri(uri).unwrap();
        assert_eq!(parsed.resource_type, "build-log");
        assert_eq!(parsed.path, "abc123-hello");
        assert!(parsed.params.is_empty());
    }

    #[test]
    fn test_parse_nix_uri_with_params() {
        let uri = "nix://derivation/abc123.drv?summary=true&offset=10";
        let parsed = parse_nix_uri(uri).unwrap();
        assert_eq!(parsed.resource_type, "derivation");
        assert_eq!(parsed.path, "abc123.drv");
        assert_eq!(parsed.params.get("summary"), Some(&"true".to_string()));
        assert_eq!(parsed.params.get("offset"), Some(&"10".to_string()));
    }

    #[test]
    fn test_parse_nix_uri_full_store_path() {
        let uri = "nix://closure//nix/store/abc123-hello?limit=50";
        let parsed = parse_nix_uri(uri).unwrap();
        assert_eq!(parsed.resource_type, "closure");
        assert_eq!(parsed.path, "/nix/store/abc123-hello");
        assert_eq!(parsed.params.get("limit"), Some(&"50".to_string()));
    }

    #[test]
    fn test_parse_nix_uri_invalid_scheme() {
        let uri = "http://example.com";
        let result = parse_nix_uri(uri);
        assert!(result.is_err());
    }
}
