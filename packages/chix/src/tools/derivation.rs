use crate::nix_runner::run_nix_command_in_dir;
use crate::output::{limit_stderr, PaginationInfo, TruncationInfo};
use crate::tools::NixDerivationShowParams;
use crate::validators::{validate_flake_ref, validate_path, validate_store_path};
use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct DerivationSummary {
    pub path: String,
    pub name: Option<String>,
    pub outputs: Vec<String>,
    pub input_count: usize,
}

#[derive(Debug, Serialize)]
pub struct NixDerivationShowResult {
    pub success: bool,
    /// Full derivation data (when summary_only=false) or null (when summary_only=true)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub derivation: Option<serde_json::Value>,
    /// Summary data (when summary_only=true)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub summary: Option<Vec<DerivationSummary>>,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pagination: Option<PaginationInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

fn extract_derivation_summary(
    path: &str,
    drv: &serde_json::Value,
) -> DerivationSummary {
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

pub async fn nix_derivation_show(
    params: NixDerivationShowParams,
) -> Result<NixDerivationShowResult, String> {
    let installable = params.installable.unwrap_or_else(|| ".#default".to_string());

    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    // Validate based on whether it's a store path or installable
    if installable.starts_with("/nix/store/") {
        validate_store_path(&installable).map_err(|e| e.to_string())?;
    } else {
        validate_flake_ref(&installable).map_err(|e| e.to_string())?;
    }

    let mut args = vec!["derivation", "show"];

    if params.recursive.unwrap_or(false) {
        args.push("--recursive");
    }

    args.push(&installable);

    let result = run_nix_command_in_dir(&args, flake_dir)
        .await
        .map_err(|e| e.to_string())?;

    let limited_stderr = limit_stderr(&result.stderr);

    if !result.success {
        return Ok(NixDerivationShowResult {
            success: false,
            derivation: Some(serde_json::Value::Null),
            summary: None,
            stderr: limited_stderr.content,
            pagination: None,
            truncated: if limited_stderr.truncated { Some(true) } else { None },
            truncation_info: limited_stderr.truncation_info,
        });
    }

    let parsed: serde_json::Value =
        serde_json::from_str(&result.stdout).unwrap_or(serde_json::Value::Null);

    // Handle summary_only mode
    if params.summary_only.unwrap_or(false) {
        if let serde_json::Value::Object(map) = &parsed {
            let total = map.len();
            let offset = params.inputs_offset.unwrap_or(0);
            let limit = params.max_inputs.unwrap_or(total);

            // Sort by path for consistent ordering
            let mut entries: Vec<_> = map.iter().collect();
            entries.sort_by(|a, b| a.0.cmp(b.0));

            let summaries: Vec<DerivationSummary> = entries
                .into_iter()
                .skip(offset)
                .take(limit)
                .map(|(path, drv)| extract_derivation_summary(path, drv))
                .collect();

            let kept_count = summaries.len();
            let has_more = offset + kept_count < total;

            let pagination = if params.max_inputs.is_some() || params.inputs_offset.is_some() {
                Some(PaginationInfo {
                    offset,
                    limit,
                    total,
                    has_more,
                })
            } else {
                None
            };

            return Ok(NixDerivationShowResult {
                success: true,
                derivation: None,
                summary: Some(summaries),
                stderr: limited_stderr.content,
                pagination,
                truncated: if limited_stderr.truncated { Some(true) } else { None },
                truncation_info: limited_stderr.truncation_info,
            });
        }
    }

    // Full derivation mode with optional pagination
    if let serde_json::Value::Object(map) = parsed {
        let total = map.len();
        let offset = params.inputs_offset.unwrap_or(0);
        let limit = params.max_inputs.unwrap_or(total);

        // Sort and paginate
        let mut entries: Vec<_> = map.into_iter().collect();
        entries.sort_by(|a, b| a.0.cmp(&b.0));

        let paginated: serde_json::Map<String, serde_json::Value> = entries
            .into_iter()
            .skip(offset)
            .take(limit)
            .collect();

        let kept_count = paginated.len();
        let has_more = offset + kept_count < total;

        let pagination = if params.max_inputs.is_some() || params.inputs_offset.is_some() {
            Some(PaginationInfo {
                offset,
                limit,
                total,
                has_more,
            })
        } else {
            None
        };

        Ok(NixDerivationShowResult {
            success: true,
            derivation: Some(serde_json::Value::Object(paginated)),
            summary: None,
            stderr: limited_stderr.content,
            pagination,
            truncated: if limited_stderr.truncated { Some(true) } else { None },
            truncation_info: limited_stderr.truncation_info,
        })
    } else {
        Ok(NixDerivationShowResult {
            success: true,
            derivation: Some(parsed),
            summary: None,
            stderr: limited_stderr.content,
            pagination: None,
            truncated: if limited_stderr.truncated { Some(true) } else { None },
            truncation_info: limited_stderr.truncation_info,
        })
    }
}
