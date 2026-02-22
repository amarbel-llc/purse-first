use crate::nix_runner::run_nix_command_in_dir;
use crate::output::{limit_stderr, limit_text_output, OutputLimits, TruncationInfo};
use crate::tools::NixEvalParams;
use crate::validators::{validate_installable, validate_nix_expr, validate_path};
use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct NixEvalResult {
    pub success: bool,
    pub value: serde_json::Value,
    pub stderr: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_eval(params: NixEvalParams) -> Result<NixEvalResult, String> {
    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    let mut args = vec!["eval", "--json"];

    let installable: Option<String>;
    let expr: Option<String>;
    let apply: Option<String>;

    if let Some(ref i) = params.installable {
        validate_installable(i).map_err(|e| e.to_string())?;
        installable = Some(i.clone());
    } else {
        installable = None;
    }

    if let Some(ref e) = params.expr {
        validate_nix_expr(e).map_err(|e| e.to_string())?;
        expr = Some(e.clone());
    } else {
        expr = None;
    }

    if let Some(ref a) = params.apply {
        validate_nix_expr(a).map_err(|e| e.to_string())?;
        apply = Some(a.clone());
    } else {
        apply = None;
    }

    if let Some(ref i) = installable {
        args.push(i);
    }

    if let Some(ref e) = expr {
        args.push("--expr");
        args.push(e);
    }

    if let Some(ref a) = apply {
        args.push("--apply");
        args.push(a);
    }

    if installable.is_none() && expr.is_none() {
        return Err("Either 'installable' or 'expr' must be provided".to_string());
    }

    let result = run_nix_command_in_dir(&args, flake_dir)
        .await
        .map_err(|e| e.to_string())?;

    let limited_stderr = limit_stderr(&result.stderr);

    if !result.success {
        return Ok(NixEvalResult {
            success: false,
            value: serde_json::Value::Null,
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

    let value =
        serde_json::from_str(&limited.content).unwrap_or(serde_json::Value::String(limited.content));

    let truncated = limited.truncated || limited_stderr.truncated;

    Ok(NixEvalResult {
        success: true,
        value,
        stderr: limited_stderr.content,
        truncated: if truncated { Some(true) } else { None },
        truncation_info: limited.truncation_info.or(limited_stderr.truncation_info),
    })
}
