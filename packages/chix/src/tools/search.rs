use crate::nix_runner::run_nix_command;
use crate::output::{limit_stderr, OutputLimitsConfig, PaginationInfo, TruncationInfo};
use crate::tools::NixSearchParams;
use crate::validators::{validate_flake_ref, validate_no_shell_metacharacters};
use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct NixSearchResult {
    pub success: bool,
    pub packages: serde_json::Value,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pagination: Option<PaginationInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_search(params: NixSearchParams) -> Result<NixSearchResult, String> {
    let flake_ref = params.flake_ref.unwrap_or_else(|| "nixpkgs".to_string());
    validate_flake_ref(&flake_ref).map_err(|e| e.to_string())?;

    validate_no_shell_metacharacters(&params.query).map_err(|e| e.to_string())?;

    let mut args = vec!["search", "--json", &flake_ref, &params.query];

    // Add --exclude patterns if provided
    let excludes = params.exclude.unwrap_or_default();
    let exclude_args: Vec<String> = excludes
        .iter()
        .flat_map(|e| vec!["--exclude".to_string(), e.clone()])
        .collect();
    for arg in &exclude_args {
        args.push(arg);
    }

    let result = run_nix_command(&args).await.map_err(|e| e.to_string())?;

    let config = OutputLimitsConfig::default();

    let (packages, pagination) = if result.success {
        match serde_json::from_str::<serde_json::Value>(&result.stdout) {
            Ok(serde_json::Value::Object(map)) => {
                let total = map.len();
                let offset = params.offset.unwrap_or(0);
                let limit = params.limit.unwrap_or(config.search_limit_default());

                // Apply pagination by converting to sorted vec, slicing, then back to object
                let mut entries: Vec<_> = map.into_iter().collect();
                entries.sort_by(|a, b| a.0.cmp(&b.0)); // Sort by package name

                let paginated: serde_json::Map<String, serde_json::Value> = entries
                    .into_iter()
                    .skip(offset)
                    .take(limit)
                    .collect();

                let kept_count = paginated.len();
                let has_more = offset + kept_count < total;

                let pagination_info = if kept_count < total || offset > 0 {
                    Some(PaginationInfo {
                        offset,
                        limit,
                        total,
                        has_more,
                    })
                } else {
                    None
                };

                (serde_json::Value::Object(paginated), pagination_info)
            }
            Ok(other) => (other, None),
            Err(_) => (serde_json::Value::Null, None),
        }
    } else {
        (serde_json::Value::Null, None)
    };

    let limited_stderr = limit_stderr(&result.stderr);

    Ok(NixSearchResult {
        success: result.success,
        packages,
        stderr: limited_stderr.content,
        pagination,
        truncated: if limited_stderr.truncated { Some(true) } else { None },
        truncation_info: limited_stderr.truncation_info,
    })
}
