use crate::nix_runner::run_nix_command;
use crate::output::PaginationInfo;
use crate::resources::{ParsedUri, ResourceContent};
use crate::validators::validate_store_path;
use serde::Serialize;

#[derive(Debug, Serialize)]
struct BuildLogResponse {
    log: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pagination: Option<PaginationInfo>,
}

pub async fn read_build_log(parsed: &ParsedUri) -> Result<ResourceContent, String> {
    // The path should be a store path or hash
    let path = if parsed.path.starts_with("/nix/store/") {
        parsed.path.clone()
    } else {
        format!("/nix/store/{}", parsed.path)
    };

    validate_store_path(&path).map_err(|e| e.to_string())?;

    // Parse pagination params
    let offset: usize = parsed
        .params
        .get("offset")
        .and_then(|s| s.parse().ok())
        .unwrap_or(0);
    let limit: Option<usize> = parsed.params.get("limit").and_then(|s| s.parse().ok());

    // Get the build log
    let args = vec!["log", &path];
    let result = run_nix_command(&args).await.map_err(|e| e.to_string())?;

    if !result.success {
        return Err(format!("Failed to get build log: {}", result.stderr));
    }

    // Apply pagination
    let lines: Vec<&str> = result.stdout.lines().collect();
    let total_lines = lines.len();

    let (content, pagination) = if let Some(lim) = limit {
        let paginated_lines: Vec<&str> = lines.into_iter().skip(offset).take(lim).collect();
        let kept_count = paginated_lines.len();
        let has_more = offset + kept_count < total_lines;

        let pagination_info = PaginationInfo {
            offset,
            limit: lim,
            total: total_lines,
            has_more,
        };

        (paginated_lines.join("\n"), Some(pagination_info))
    } else if offset > 0 {
        let paginated_lines: Vec<&str> = lines.into_iter().skip(offset).collect();

        let pagination_info = PaginationInfo {
            offset,
            limit: total_lines,
            total: total_lines,
            has_more: false,
        };

        (paginated_lines.join("\n"), Some(pagination_info))
    } else {
        (result.stdout, None)
    };

    let response = BuildLogResponse {
        log: content,
        pagination,
    };

    Ok(ResourceContent {
        uri: format!("nix://build-log/{}", parsed.path),
        mime_type: "application/json".to_string(),
        text: serde_json::to_string_pretty(&response).map_err(|e| e.to_string())?,
    })
}
