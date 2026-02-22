use crate::nix_runner::run_nix_command;
use crate::output::PaginationInfo;
use crate::resources::{ParsedUri, ResourceContent};
use crate::validators::{validate_flake_ref, validate_store_path};
use serde::Serialize;

#[derive(Debug, Serialize)]
struct ClosureResponse {
    paths: serde_json::Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pagination: Option<PaginationInfo>,
}

pub async fn read_closure(parsed: &ParsedUri) -> Result<ResourceContent, String> {
    // The path should be a store path or installable
    let path = if parsed.path.starts_with("/nix/store/") {
        parsed.path.clone()
    } else if parsed.path.contains('#') || parsed.path.starts_with('.') {
        parsed.path.clone()
    } else {
        format!("/nix/store/{}", parsed.path)
    };

    // Validate
    if path.starts_with("/nix/store/") {
        validate_store_path(&path).map_err(|e| e.to_string())?;
    } else {
        validate_flake_ref(&path).map_err(|e| e.to_string())?;
    }

    // Parse pagination params
    let offset: usize = parsed
        .params
        .get("offset")
        .and_then(|s| s.parse().ok())
        .unwrap_or(0);
    let limit: Option<usize> = parsed.params.get("limit").and_then(|s| s.parse().ok());

    // Get closure info
    let args = vec!["path-info", "--json", "--closure", &path];
    let result = run_nix_command(&args).await.map_err(|e| e.to_string())?;

    if !result.success {
        return Err(format!("Failed to get closure: {}", result.stderr));
    }

    let parsed_json: serde_json::Value =
        serde_json::from_str(&result.stdout).map_err(|e| e.to_string())?;

    let response = if let serde_json::Value::Array(arr) = parsed_json {
        let total = arr.len();
        let lim = limit.unwrap_or(total);

        let paginated: Vec<serde_json::Value> =
            arr.into_iter().skip(offset).take(lim).collect();

        let kept_count = paginated.len();
        let has_more = offset + kept_count < total;

        let pagination = if limit.is_some() || offset > 0 {
            Some(PaginationInfo {
                offset,
                limit: lim,
                total,
                has_more,
            })
        } else {
            None
        };

        ClosureResponse {
            paths: serde_json::Value::Array(paginated),
            pagination,
        }
    } else {
        ClosureResponse {
            paths: parsed_json,
            pagination: None,
        }
    };

    Ok(ResourceContent {
        uri: format!("nix://closure/{}", parsed.path),
        mime_type: "application/json".to_string(),
        text: serde_json::to_string_pretty(&response).map_err(|e| e.to_string())?,
    })
}
