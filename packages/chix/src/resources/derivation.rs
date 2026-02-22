use crate::nix_runner::run_nix_command;
use crate::output::PaginationInfo;
use crate::resources::{ParsedUri, ResourceContent};
use crate::validators::{validate_flake_ref, validate_store_path};
use serde::Serialize;

#[derive(Debug, Serialize)]
struct DerivationSummary {
    path: String,
    name: Option<String>,
    outputs: Vec<String>,
    input_count: usize,
}

#[derive(Debug, Serialize)]
struct DerivationResponse {
    #[serde(skip_serializing_if = "Option::is_none")]
    derivation: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    summary: Option<Vec<DerivationSummary>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pagination: Option<PaginationInfo>,
}

fn extract_summary(path: &str, drv: &serde_json::Value) -> DerivationSummary {
    let name = drv
        .get("name")
        .and_then(|v| v.as_str())
        .map(|s| s.to_string());

    let outputs = drv
        .get("outputs")
        .and_then(|v| v.as_object())
        .map(|o| o.keys().cloned().collect())
        .unwrap_or_default();

    let input_count = drv
        .get("inputDrvs")
        .and_then(|v| v.as_object())
        .map(|o| o.len())
        .unwrap_or(0);

    DerivationSummary {
        path: path.to_string(),
        name,
        outputs,
        input_count,
    }
}

pub async fn read_derivation(parsed: &ParsedUri) -> Result<ResourceContent, String> {
    // The path can be a store path, drv path, or installable
    let path = if parsed.path.starts_with("/nix/store/") {
        parsed.path.clone()
    } else if parsed.path.contains('#') || parsed.path.starts_with('.') {
        // Looks like an installable
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

    // Parse params
    let summary_mode = parsed
        .params
        .get("summary")
        .map(|s| s == "true")
        .unwrap_or(false);
    let offset: usize = parsed
        .params
        .get("offset")
        .and_then(|s| s.parse().ok())
        .unwrap_or(0);
    let limit: Option<usize> = parsed.params.get("limit").and_then(|s| s.parse().ok());
    let recursive = parsed
        .params
        .get("recursive")
        .map(|s| s == "true")
        .unwrap_or(false);

    // Build command
    let mut args = vec!["derivation", "show"];
    if recursive {
        args.push("--recursive");
    }
    args.push(&path);

    let result = run_nix_command(&args).await.map_err(|e| e.to_string())?;

    if !result.success {
        return Err(format!("Failed to get derivation: {}", result.stderr));
    }

    let parsed_json: serde_json::Value =
        serde_json::from_str(&result.stdout).map_err(|e| e.to_string())?;

    let response = if let serde_json::Value::Object(map) = parsed_json {
        let total = map.len();
        let lim = limit.unwrap_or(total);

        // Sort entries for consistent ordering
        let mut entries: Vec<_> = map.into_iter().collect();
        entries.sort_by(|a, b| a.0.cmp(&b.0));

        if summary_mode {
            let summaries: Vec<DerivationSummary> = entries
                .iter()
                .skip(offset)
                .take(lim)
                .map(|(p, v)| extract_summary(p, v))
                .collect();

            let kept_count = summaries.len();
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

            DerivationResponse {
                derivation: None,
                summary: Some(summaries),
                pagination,
            }
        } else {
            let paginated: serde_json::Map<String, serde_json::Value> = entries
                .into_iter()
                .skip(offset)
                .take(lim)
                .collect();

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

            DerivationResponse {
                derivation: Some(serde_json::Value::Object(paginated)),
                summary: None,
                pagination,
            }
        }
    } else {
        DerivationResponse {
            derivation: Some(parsed_json),
            summary: None,
            pagination: None,
        }
    };

    Ok(ResourceContent {
        uri: format!("nix://derivation/{}", parsed.path),
        mime_type: "application/json".to_string(),
        text: serde_json::to_string_pretty(&response).map_err(|e| e.to_string())?,
    })
}
