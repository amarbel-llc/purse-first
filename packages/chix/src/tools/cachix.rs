use crate::config::{get_cachix_token, get_default_cache, load_config};
use crate::nix_runner::{run_cachix_command, run_cachix_command_with_env, NixError};
use crate::output::{limit_stderr, TruncationInfo};
use crate::validators::{validate_cache_name, validate_store_paths};
use serde::Serialize;

const DEFAULT_TIMEOUT_SECS: u64 = 300;

#[derive(Debug, Serialize)]
pub struct CachixPushResult {
    pub success: bool,
    pub paths_pushed: Vec<String>,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

#[derive(Debug, Serialize)]
pub struct CachixUseResult {
    pub success: bool,
    pub cache_name: String,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

#[derive(Debug, Serialize)]
pub struct CachixStatusResult {
    pub success: bool,
    pub authenticated: bool,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn cachix_push(
    cache_name: Option<String>,
    store_paths: Vec<String>,
) -> Result<CachixPushResult, String> {
    let config = load_config();

    let cache = match cache_name.as_deref() {
        Some(name) => name.to_string(),
        None => get_default_cache(&config)
            .ok_or_else(|| "No cache name provided and no default cache configured".to_string())?,
    };

    validate_cache_name(&cache).map_err(|e| e.to_string())?;
    validate_store_paths(&store_paths).map_err(|e| e.to_string())?;

    if store_paths.is_empty() {
        return Err("No store paths provided".to_string());
    }

    let token = get_cachix_token(&config, Some(&cache));
    let env_vars: Vec<(&str, &str)> = match &token {
        Some(t) => vec![("CACHIX_AUTH_TOKEN", t.as_str())],
        None => vec![],
    };

    // cachix push reads paths from stdin
    // For now, use command-line args approach with multiple paths
    let mut args = vec!["push", &cache];
    let path_refs: Vec<&str> = store_paths.iter().map(|s| s.as_str()).collect();
    args.extend(path_refs.iter());

    let output = run_cachix_command_with_env(&args, &env_vars, DEFAULT_TIMEOUT_SECS)
        .await
        .map_err(|e| match e {
            NixError::Timeout(secs) => format!("cachix push timed out after {} seconds", secs),
            NixError::CommandFailed(msg) => format!("cachix push failed: {}", msg),
            NixError::Io(e) => format!("IO error running cachix: {}", e),
        })?;

    let limited_stderr = limit_stderr(&output.stderr);

    Ok(CachixPushResult {
        success: output.success,
        paths_pushed: if output.success {
            store_paths
        } else {
            vec![]
        },
        stdout: output.stdout,
        stderr: limited_stderr.content,
        truncated: if limited_stderr.truncated { Some(true) } else { None },
        truncation_info: limited_stderr.truncation_info,
    })
}

pub async fn cachix_use(cache_name: String) -> Result<CachixUseResult, String> {
    validate_cache_name(&cache_name).map_err(|e| e.to_string())?;

    let output = run_cachix_command(&["use", &cache_name])
        .await
        .map_err(|e| match e {
            NixError::Timeout(secs) => format!("cachix use timed out after {} seconds", secs),
            NixError::CommandFailed(msg) => format!("cachix use failed: {}", msg),
            NixError::Io(e) => format!("IO error running cachix: {}", e),
        })?;

    let limited_stderr = limit_stderr(&output.stderr);

    Ok(CachixUseResult {
        success: output.success,
        cache_name,
        stdout: output.stdout,
        stderr: limited_stderr.content,
        truncated: if limited_stderr.truncated { Some(true) } else { None },
        truncation_info: limited_stderr.truncation_info,
    })
}

pub async fn cachix_status() -> Result<CachixStatusResult, String> {
    // 'cachix authtoken' with no args shows current auth status
    // If not authenticated, it will show a message about not being logged in
    let output = run_cachix_command(&["authtoken"])
        .await
        .map_err(|e| match e {
            NixError::Timeout(secs) => format!("cachix authtoken timed out after {} seconds", secs),
            NixError::CommandFailed(msg) => format!("cachix authtoken failed: {}", msg),
            NixError::Io(e) => format!("IO error running cachix: {}", e),
        })?;

    // Check if output indicates authentication
    let authenticated = output.success
        || output.stdout.contains("token")
        || !output.stderr.contains("not authenticated");

    let limited_stderr = limit_stderr(&output.stderr);

    Ok(CachixStatusResult {
        success: true,
        authenticated,
        stdout: output.stdout,
        stderr: limited_stderr.content,
        truncated: if limited_stderr.truncated { Some(true) } else { None },
        truncation_info: limited_stderr.truncation_info,
    })
}
