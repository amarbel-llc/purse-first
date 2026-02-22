use crate::nix_runner::{parse_json_store_paths, parse_store_paths, run_nix_command_in_dir};
use crate::output::{limit_text_output, OutputLimits, TruncationInfo};
use crate::tools::NixBuildParams;
use crate::validators::{validate_installable, validate_path};
use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct NixBuildResult {
    pub success: bool,
    pub store_paths: Vec<String>,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_build(params: NixBuildParams) -> Result<NixBuildResult, String> {
    let installable = params
        .installable
        .unwrap_or_else(|| ".#default".to_string());
    validate_installable(&installable).map_err(|e| e.to_string())?;

    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    let mut args = vec!["build", "--json", "--print-out-paths"];

    if params.print_build_logs.unwrap_or(true) {
        args.push("-L");
    }

    args.push(&installable);

    let result = run_nix_command_in_dir(&args, flake_dir)
        .await
        .map_err(|e| e.to_string())?;

    let mut store_paths = parse_json_store_paths(&result.stdout);
    if store_paths.is_empty() {
        store_paths = parse_store_paths(&result.stdout);
    }

    // Apply output limits to stderr (build logs)
    let limits = OutputLimits {
        head: None,
        tail: params.log_tail,
        max_bytes: params.max_log_bytes,
        max_lines: None,
    };

    let limited_stderr = limit_text_output(&result.stderr, &limits);

    Ok(NixBuildResult {
        success: result.success,
        store_paths,
        stderr: limited_stderr.content,
        truncated: if limited_stderr.truncated {
            Some(true)
        } else {
            None
        },
        truncation_info: limited_stderr.truncation_info,
    })
}
