use crate::nix_runner::run_nix_command_in_dir;
use crate::output::{limit_stderr, limit_text_output, OutputLimits, TruncationInfo};
use crate::tools::{
    NixFlakeCheckParams, NixFlakeInitParams, NixFlakeLockParams, NixFlakeMetadataParams,
    NixFlakeShowParams, NixFlakeUpdateParams,
};
use crate::validators::{validate_args, validate_flake_ref, validate_path};
use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct NixFlakeShowResult {
    pub success: bool,
    pub outputs: serde_json::Value,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_flake_show(params: NixFlakeShowParams) -> Result<NixFlakeShowResult, String> {
    let flake_ref = params.flake_ref.unwrap_or_else(|| ".".to_string());
    validate_flake_ref(&flake_ref).map_err(|e| e.to_string())?;

    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    let mut args = vec!["flake", "show", "--json"];

    if params.all_systems.unwrap_or(false) {
        args.push("--all-systems");
    }

    args.push(&flake_ref);

    let result = run_nix_command_in_dir(&args, flake_dir)
        .await
        .map_err(|e| e.to_string())?;

    let limited_stderr = limit_stderr(&result.stderr);

    if !result.success {
        return Ok(NixFlakeShowResult {
            success: false,
            outputs: serde_json::Value::Null,
            stderr: limited_stderr.content,
            truncated: if limited_stderr.truncated { Some(true) } else { None },
            truncation_info: limited_stderr.truncation_info,
        });
    }

    let limits = OutputLimits {
        head: params.head,
        tail: params.tail,
        max_bytes: params.max_bytes,
        max_lines: None,
    };

    let limited = limit_text_output(&result.stdout, &limits);

    let outputs =
        serde_json::from_str(&limited.content).unwrap_or(serde_json::Value::String(limited.content));

    let truncated = limited.truncated || limited_stderr.truncated;

    Ok(NixFlakeShowResult {
        success: true,
        outputs,
        stderr: limited_stderr.content,
        truncated: if truncated { Some(true) } else { None },
        truncation_info: limited.truncation_info.or(limited_stderr.truncation_info),
    })
}

#[derive(Debug, Serialize)]
pub struct NixFlakeCheckResult {
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_flake_check(params: NixFlakeCheckParams) -> Result<NixFlakeCheckResult, String> {
    let flake_ref = params.flake_ref.unwrap_or_else(|| ".".to_string());
    validate_flake_ref(&flake_ref).map_err(|e| e.to_string())?;

    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    let mut args = vec!["flake", "check"];

    if params.keep_going.unwrap_or(true) {
        args.push("--keep-going");
    }

    args.push(&flake_ref);

    let result = run_nix_command_in_dir(&args, flake_dir)
        .await
        .map_err(|e| e.to_string())?;

    // Apply output limits to both stdout and stderr
    let limits = OutputLimits {
        head: params.head,
        tail: params.tail,
        max_bytes: params.max_bytes,
        max_lines: None,
    };

    let limited_stdout = limit_text_output(&result.stdout, &limits);
    let limited_stderr = limit_text_output(&result.stderr, &limits);

    // Report truncation if either was truncated
    let truncated = limited_stdout.truncated || limited_stderr.truncated;

    Ok(NixFlakeCheckResult {
        success: result.success,
        stdout: limited_stdout.content,
        stderr: limited_stderr.content,
        truncated: if truncated { Some(true) } else { None },
        truncation_info: limited_stderr.truncation_info.or(limited_stdout.truncation_info),
    })
}

#[derive(Debug, Serialize)]
pub struct NixFlakeMetadataResult {
    pub success: bool,
    pub metadata: serde_json::Value,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_flake_metadata(
    params: NixFlakeMetadataParams,
) -> Result<NixFlakeMetadataResult, String> {
    let flake_ref = params.flake_ref.unwrap_or_else(|| ".".to_string());
    validate_flake_ref(&flake_ref).map_err(|e| e.to_string())?;

    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    let args = vec!["flake", "metadata", "--json", &flake_ref];

    let result = run_nix_command_in_dir(&args, flake_dir)
        .await
        .map_err(|e| e.to_string())?;

    let limited_stderr = limit_stderr(&result.stderr);

    if !result.success {
        return Ok(NixFlakeMetadataResult {
            success: false,
            metadata: serde_json::Value::Null,
            stderr: limited_stderr.content,
            truncated: if limited_stderr.truncated { Some(true) } else { None },
            truncation_info: limited_stderr.truncation_info,
        });
    }

    let limits = OutputLimits {
        head: params.head,
        tail: params.tail,
        max_bytes: params.max_bytes,
        max_lines: None,
    };

    let limited = limit_text_output(&result.stdout, &limits);

    let metadata =
        serde_json::from_str(&limited.content).unwrap_or(serde_json::Value::String(limited.content));

    let truncated = limited.truncated || limited_stderr.truncated;

    Ok(NixFlakeMetadataResult {
        success: true,
        metadata,
        stderr: limited_stderr.content,
        truncated: if truncated { Some(true) } else { None },
        truncation_info: limited.truncation_info.or(limited_stderr.truncation_info),
    })
}

#[derive(Debug, Serialize)]
pub struct NixFlakeUpdateResult {
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_flake_update(params: NixFlakeUpdateParams) -> Result<NixFlakeUpdateResult, String> {
    let flake_ref = params.flake_ref.unwrap_or_else(|| ".".to_string());
    validate_flake_ref(&flake_ref).map_err(|e| e.to_string())?;

    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    let inputs = params.inputs.unwrap_or_default();
    validate_args(&inputs).map_err(|e| e.to_string())?;

    let mut args = vec!["flake", "update"];

    // Add specific inputs if provided
    let input_refs: Vec<String> = inputs.iter().map(|s| s.clone()).collect();
    for input in &input_refs {
        args.push(input);
    }

    args.push("--flake");
    args.push(&flake_ref);

    let result = run_nix_command_in_dir(&args, flake_dir)
        .await
        .map_err(|e| e.to_string())?;

    let limits = OutputLimits {
        head: params.head,
        tail: params.tail,
        max_bytes: params.max_bytes,
        max_lines: None,
    };

    let limited_stdout = limit_text_output(&result.stdout, &limits);
    let limited_stderr = limit_text_output(&result.stderr, &limits);

    let truncated = limited_stdout.truncated || limited_stderr.truncated;

    Ok(NixFlakeUpdateResult {
        success: result.success,
        stdout: limited_stdout.content,
        stderr: limited_stderr.content,
        truncated: if truncated { Some(true) } else { None },
        truncation_info: limited_stdout.truncation_info.or(limited_stderr.truncation_info),
    })
}

#[derive(Debug, Serialize)]
pub struct NixFlakeLockResult {
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_flake_lock(params: NixFlakeLockParams) -> Result<NixFlakeLockResult, String> {
    let flake_ref = params.flake_ref.unwrap_or_else(|| ".".to_string());
    validate_flake_ref(&flake_ref).map_err(|e| e.to_string())?;

    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    let mut args = vec!["flake", "lock"];

    // Build update-input args
    let update_inputs = params.update_inputs.unwrap_or_default();
    validate_args(&update_inputs).map_err(|e| e.to_string())?;
    let update_input_args: Vec<String> = update_inputs
        .iter()
        .flat_map(|i| vec!["--update-input".to_string(), i.clone()])
        .collect();
    for arg in &update_input_args {
        args.push(arg);
    }

    // Build override-input args
    let override_inputs = params.override_inputs.unwrap_or_default();
    let override_input_args: Vec<String> = override_inputs
        .iter()
        .flat_map(|(k, v)| vec!["--override-input".to_string(), k.clone(), v.clone()])
        .collect();
    for arg in &override_input_args {
        args.push(arg);
    }

    args.push(&flake_ref);

    let result = run_nix_command_in_dir(&args, flake_dir)
        .await
        .map_err(|e| e.to_string())?;

    let limits = OutputLimits {
        head: params.head,
        tail: params.tail,
        max_bytes: params.max_bytes,
        max_lines: None,
    };

    let limited_stdout = limit_text_output(&result.stdout, &limits);
    let limited_stderr = limit_text_output(&result.stderr, &limits);

    let truncated = limited_stdout.truncated || limited_stderr.truncated;

    Ok(NixFlakeLockResult {
        success: result.success,
        stdout: limited_stdout.content,
        stderr: limited_stderr.content,
        truncated: if truncated { Some(true) } else { None },
        truncation_info: limited_stdout.truncation_info.or(limited_stderr.truncation_info),
    })
}

#[derive(Debug, Serialize)]
pub struct NixFlakeInitResult {
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_flake_init(params: NixFlakeInitParams) -> Result<NixFlakeInitResult, String> {
    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    let mut args = vec!["flake", "init"];

    let template_ref;
    if let Some(ref template) = params.template {
        validate_flake_ref(template).map_err(|e| e.to_string())?;
        template_ref = template.clone();
        args.push("--template");
        args.push(&template_ref);
    }

    let result = run_nix_command_in_dir(&args, flake_dir)
        .await
        .map_err(|e| e.to_string())?;

    let limited_stderr = limit_stderr(&result.stderr);

    Ok(NixFlakeInitResult {
        success: result.success,
        stdout: result.stdout,
        stderr: limited_stderr.content,
        truncated: if limited_stderr.truncated { Some(true) } else { None },
        truncation_info: limited_stderr.truncation_info,
    })
}
