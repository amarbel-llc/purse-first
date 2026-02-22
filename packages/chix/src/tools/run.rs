use crate::nix_runner::run_nix_command_in_dir;
use crate::output::{limit_stderr, limit_text_output, OutputLimits, TruncationInfo};
use crate::tools::{NixDevelopRunParams, NixRunParams};
use crate::validators::{
    validate_args, validate_flake_ref, validate_installable, validate_no_shell_metacharacters,
    validate_path,
};
use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct NixRunResult {
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
    pub exit_code: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

#[derive(Debug, Serialize)]
pub struct CommandResult {
    pub command: String,
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
    pub exit_code: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

#[derive(Debug, Serialize)]
pub struct NixDevelopRunResult {
    pub success: bool,
    pub results: Vec<CommandResult>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub async fn nix_run(params: NixRunParams) -> Result<NixRunResult, String> {
    let installable = params
        .installable
        .unwrap_or_else(|| ".#default".to_string());
    validate_installable(&installable).map_err(|e| e.to_string())?;

    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    if let Some(ref args) = params.args {
        validate_args(args).map_err(|e| e.to_string())?;
    }

    let mut args: Vec<&str> = vec!["run", &installable];

    let user_args: Vec<String> = params.args.unwrap_or_default();
    if !user_args.is_empty() {
        args.push("--");
        for arg in &user_args {
            args.push(arg);
        }
    }

    let result = run_nix_command_in_dir(&args, flake_dir)
        .await
        .map_err(|e| e.to_string())?;

    let limited_stderr = limit_stderr(&result.stderr);

    Ok(NixRunResult {
        success: result.success,
        stdout: result.stdout,
        stderr: limited_stderr.content,
        exit_code: result.exit_code,
        truncated: if limited_stderr.truncated { Some(true) } else { None },
        truncation_info: limited_stderr.truncation_info,
    })
}

pub async fn nix_develop_run(params: NixDevelopRunParams) -> Result<NixDevelopRunResult, String> {
    let flake_ref = params.flake_ref.unwrap_or_else(|| ".".to_string());
    validate_flake_ref(&flake_ref).map_err(|e| e.to_string())?;

    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    if params.commands.is_empty() {
        return Err("commands array must not be empty".to_string());
    }

    let limits = OutputLimits {
        head: params.head,
        tail: params.tail,
        max_bytes: params.max_bytes,
        max_lines: None,
    };

    let mut results = Vec::new();
    let mut all_success = true;
    let mut any_truncated = false;

    for entry in &params.commands {
        validate_no_shell_metacharacters(&entry.command).map_err(|e| e.to_string())?;

        if let Some(ref args) = entry.args {
            validate_args(args).map_err(|e| e.to_string())?;
        }

        let mut nix_args: Vec<&str> = vec!["develop", &flake_ref, "-c", &entry.command];

        let user_args: Vec<&str> = entry
            .args
            .as_deref()
            .unwrap_or_default()
            .iter()
            .map(|s| s.as_str())
            .collect();
        for arg in &user_args {
            nix_args.push(arg);
        }

        let command_display = if user_args.is_empty() {
            entry.command.clone()
        } else {
            format!("{} {}", entry.command, user_args.join(" "))
        };

        let result = run_nix_command_in_dir(&nix_args, flake_dir)
            .await
            .map_err(|e| e.to_string())?;

        let limited_stdout = limit_text_output(&result.stdout, &limits);
        let limited_stderr = limit_text_output(&result.stderr, &limits);
        let truncated = limited_stdout.truncated || limited_stderr.truncated;
        if truncated {
            any_truncated = true;
        }

        // Use stdout truncation_info as the per-command info (primary output)
        let truncation_info = limited_stdout
            .truncation_info
            .or(limited_stderr.truncation_info);

        let success = result.success;
        results.push(CommandResult {
            command: command_display,
            success,
            stdout: limited_stdout.content,
            stderr: limited_stderr.content,
            exit_code: result.exit_code,
            truncated: if truncated { Some(true) } else { None },
            truncation_info,
        });

        if !success {
            all_success = false;
            break;
        }
    }

    Ok(NixDevelopRunResult {
        success: all_success,
        truncated: if any_truncated { Some(true) } else { None },
        truncation_info: None,
        results,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tools::{CommandEntry, NixDevelopRunParams};

    #[tokio::test]
    async fn test_develop_run_rejects_empty_commands() {
        let params = NixDevelopRunParams {
            flake_ref: None,
            commands: vec![],
            flake_dir: None,
            max_bytes: None,
            head: None,
            tail: None,
        };
        let result = nix_develop_run(params).await;
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("must not be empty"));
    }

    #[tokio::test]
    async fn test_develop_run_validates_command_metacharacters() {
        let params = NixDevelopRunParams {
            flake_ref: Some(".".to_string()),
            commands: vec![CommandEntry {
                command: "echo;rm".to_string(),
                args: None,
            }],
            flake_dir: None,
            max_bytes: None,
            head: None,
            tail: None,
        };
        let result = nix_develop_run(params).await;
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("shell metacharacters"));
    }

    #[tokio::test]
    async fn test_develop_run_validates_args() {
        let params = NixDevelopRunParams {
            flake_ref: Some(".".to_string()),
            commands: vec![CommandEntry {
                command: "echo".to_string(),
                args: Some(vec!["hello; rm -rf /".to_string()]),
            }],
            flake_dir: None,
            max_bytes: None,
            head: None,
            tail: None,
        };
        let result = nix_develop_run(params).await;
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("shell metacharacters"));
    }

    #[tokio::test]
    async fn test_develop_run_validates_flake_ref() {
        let params = NixDevelopRunParams {
            flake_ref: Some("$(malicious)".to_string()),
            commands: vec![CommandEntry {
                command: "echo".to_string(),
                args: None,
            }],
            flake_dir: None,
            max_bytes: None,
            head: None,
            tail: None,
        };
        let result = nix_develop_run(params).await;
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("invalid flake reference"));
    }

    #[tokio::test]
    async fn test_develop_run_validates_path() {
        let params = NixDevelopRunParams {
            flake_ref: None,
            commands: vec![CommandEntry {
                command: "echo".to_string(),
                args: None,
            }],
            flake_dir: Some("/path;injection".to_string()),
            max_bytes: None,
            head: None,
            tail: None,
        };
        let result = nix_develop_run(params).await;
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("invalid path"));
    }
}
