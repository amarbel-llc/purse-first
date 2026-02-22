use crate::nix_runner::run_nix_command;
use crate::output::{limit_stderr, PaginationInfo, TruncationInfo};
use crate::tools::{NixCopyParams, NixStoreCatParams, NixStoreLsParams, NixStoreGcParams, NixStorePathInfoParams};
use crate::validators::{validate_flake_ref, validate_no_shell_metacharacters, validate_store_path, validate_store_subpath};
use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct NixStorePathInfoResult {
    pub success: bool,
    pub path_info: serde_json::Value,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pagination: Option<PaginationInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_store_path_info(
    params: NixStorePathInfoParams,
) -> Result<NixStorePathInfoResult, String> {
    let mut args = vec!["path-info", "--json"];

    // Validate based on whether it's a store path or installable
    let path = &params.path;
    if path.starts_with("/nix/store/") {
        validate_store_path(path).map_err(|e| e.to_string())?;
    } else {
        validate_flake_ref(path).map_err(|e| e.to_string())?;
    }

    let use_closure = params.closure.unwrap_or(false);
    if use_closure {
        args.push("--closure");
    }

    if params.derivation.unwrap_or(false) {
        args.push("--derivation");
    }

    args.push(path);

    let result = run_nix_command(&args).await.map_err(|e| e.to_string())?;

    let limited_stderr = limit_stderr(&result.stderr);

    if !result.success {
        return Ok(NixStorePathInfoResult {
            success: false,
            path_info: serde_json::Value::Null,
            stderr: limited_stderr.content,
            pagination: None,
            truncated: if limited_stderr.truncated { Some(true) } else { None },
            truncation_info: limited_stderr.truncation_info,
        });
    }

    let parsed: serde_json::Value =
        serde_json::from_str(&result.stdout).unwrap_or(serde_json::Value::Null);

    // Apply pagination if closure was requested and we have an array
    let (path_info, pagination) = if use_closure {
        if let serde_json::Value::Array(arr) = parsed {
            let total = arr.len();
            let offset = params.closure_offset.unwrap_or(0);
            let limit = params.closure_limit.unwrap_or(total);

            let paginated: Vec<serde_json::Value> =
                arr.into_iter().skip(offset).take(limit).collect();

            let kept_count = paginated.len();
            let has_more = offset + kept_count < total;

            let pagination_info =
                if params.closure_limit.is_some() || params.closure_offset.is_some() {
                    Some(PaginationInfo {
                        offset,
                        limit,
                        total,
                        has_more,
                    })
                } else {
                    None
                };

            (serde_json::Value::Array(paginated), pagination_info)
        } else {
            (parsed, None)
        }
    } else {
        (parsed, None)
    };

    Ok(NixStorePathInfoResult {
        success: true,
        path_info,
        stderr: limited_stderr.content,
        pagination,
        truncated: if limited_stderr.truncated { Some(true) } else { None },
        truncation_info: limited_stderr.truncation_info,
    })
}

#[derive(Debug, Serialize)]
pub struct NixStoreGcResult {
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_store_gc(params: NixStoreGcParams) -> Result<NixStoreGcResult, String> {
    let mut args = vec!["store", "gc"];

    if params.dry_run.unwrap_or(false) {
        args.push("--dry-run");
    }

    let max_freed_str;
    if let Some(ref max) = params.max_freed {
        validate_no_shell_metacharacters(max).map_err(|e| e.to_string())?;
        max_freed_str = max.clone();
        args.push("--max");
        args.push(&max_freed_str);
    }

    let result = run_nix_command(&args).await.map_err(|e| e.to_string())?;

    let limited_stderr = limit_stderr(&result.stderr);

    Ok(NixStoreGcResult {
        success: result.success,
        stdout: result.stdout,
        stderr: limited_stderr.content,
        truncated: if limited_stderr.truncated { Some(true) } else { None },
        truncation_info: limited_stderr.truncation_info,
    })
}

#[derive(Debug, Serialize)]
pub struct NixCopyResult {
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_copy(params: NixCopyParams) -> Result<NixCopyResult, String> {
    let mut args = vec!["copy"];

    let to_store;
    if let Some(ref to) = params.to {
        validate_no_shell_metacharacters(to).map_err(|e| e.to_string())?;
        to_store = to.clone();
        args.push("--to");
        args.push(&to_store);
    }

    let from_store;
    if let Some(ref from) = params.from {
        validate_no_shell_metacharacters(from).map_err(|e| e.to_string())?;
        from_store = from.clone();
        args.push("--from");
        args.push(&from_store);
    }

    // Validate the installable/path
    let path = &params.installable;
    if path.starts_with("/nix/store/") {
        validate_store_path(path).map_err(|e| e.to_string())?;
    } else {
        validate_flake_ref(path).map_err(|e| e.to_string())?;
    }

    args.push(path);

    let result = run_nix_command(&args).await.map_err(|e| e.to_string())?;

    let limited_stderr = limit_stderr(&result.stderr);

    Ok(NixCopyResult {
        success: result.success,
        stdout: result.stdout,
        stderr: limited_stderr.content,
        truncated: if limited_stderr.truncated { Some(true) } else { None },
        truncation_info: limited_stderr.truncation_info,
    })
}

async fn resolve_and_validate_store_path(path: &str) -> Result<std::path::PathBuf, String> {
    let canonical = tokio::fs::canonicalize(path)
        .await
        .map_err(|e| format!("Failed to resolve path '{}': {}", path, e))?;

    let canonical_str = canonical
        .to_str()
        .ok_or_else(|| "Path contains invalid UTF-8".to_string())?;

    validate_store_subpath(canonical_str).map_err(|e| e.to_string())?;

    Ok(canonical)
}

#[derive(Debug, Serialize)]
pub struct NixStoreLsEntry {
    pub name: String,
    pub entry_type: String,
    pub size: Option<u64>,
}

#[derive(Debug, Serialize)]
pub struct NixStoreLsResult {
    pub path: String,
    pub entries: Vec<NixStoreLsEntry>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pagination: Option<PaginationInfo>,
}

pub async fn nix_store_ls(params: NixStoreLsParams) -> Result<NixStoreLsResult, String> {
    let canonical = resolve_and_validate_store_path(&params.path).await?;
    let long = params.long.unwrap_or(false);

    let mut entries = Vec::new();
    let mut read_dir = tokio::fs::read_dir(&canonical)
        .await
        .map_err(|e| format!("Failed to read directory '{}': {}", canonical.display(), e))?;

    while let Some(entry) = read_dir
        .next_entry()
        .await
        .map_err(|e| format!("Failed to read entry: {}", e))?
    {
        let name = entry
            .file_name()
            .to_str()
            .unwrap_or("<invalid>")
            .to_string();

        let file_type = entry
            .file_type()
            .await
            .map_err(|e| format!("Failed to get file type: {}", e))?;

        let entry_type = if file_type.is_dir() {
            "directory"
        } else if file_type.is_symlink() {
            "symlink"
        } else {
            "file"
        }
        .to_string();

        let size = if long && file_type.is_file() {
            entry
                .metadata()
                .await
                .map(|m| Some(m.len()))
                .unwrap_or(None)
        } else {
            None
        };

        entries.push(NixStoreLsEntry {
            name,
            entry_type,
            size,
        });
    }

    entries.sort_by(|a, b| a.name.cmp(&b.name));

    let total = entries.len();
    let offset = params.offset.unwrap_or(0);
    let limit = params.limit.unwrap_or(total);

    let paginated: Vec<NixStoreLsEntry> = entries.into_iter().skip(offset).take(limit).collect();
    let kept_count = paginated.len();
    let has_more = offset + kept_count < total;

    let pagination = if params.offset.is_some() || params.limit.is_some() {
        Some(PaginationInfo {
            offset,
            limit,
            total,
            has_more,
        })
    } else {
        None
    };

    Ok(NixStoreLsResult {
        path: canonical.to_string_lossy().to_string(),
        entries: paginated,
        pagination,
    })
}

#[derive(Debug, Serialize)]
pub struct NixStoreCatResult {
    pub path: String,
    pub content: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pagination: Option<PaginationInfo>,
}

pub async fn nix_store_cat(params: NixStoreCatParams) -> Result<NixStoreCatResult, String> {
    let canonical = resolve_and_validate_store_path(&params.path).await?;

    let content = tokio::fs::read_to_string(&canonical)
        .await
        .map_err(|e| format!("Failed to read file '{}': {}", canonical.display(), e))?;

    let lines: Vec<&str> = content.lines().collect();
    let total = lines.len();
    let offset = params.offset.unwrap_or(0);
    let limit = params.limit.unwrap_or(total);

    let paginated: Vec<&str> = lines.iter().skip(offset).take(limit).copied().collect();
    let kept_count = paginated.len();
    let has_more = offset + kept_count < total;

    let pagination = if params.offset.is_some() || params.limit.is_some() {
        Some(PaginationInfo {
            offset,
            limit,
            total,
            has_more,
        })
    } else {
        None
    };

    Ok(NixStoreCatResult {
        path: canonical.to_string_lossy().to_string(),
        content: paginated.join("\n"),
        pagination,
    })
}
