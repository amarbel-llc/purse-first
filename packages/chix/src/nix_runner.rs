use std::time::Duration;
use thiserror::Error;
use tokio::process::Command;
use tokio::time::timeout;

#[derive(Error, Debug)]
pub enum NixError {
    #[error("nix command failed: {0}")]
    CommandFailed(String),

    #[error("nix command timed out after {0} seconds")]
    Timeout(u64),

    #[error("io error: {0}")]
    Io(#[from] std::io::Error),
}

#[derive(Debug)]
pub struct NixOutput {
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
    pub exit_code: Option<i32>,
}

const DEFAULT_TIMEOUT_SECS: u64 = 300;

pub async fn run_nix_command(args: &[&str]) -> Result<NixOutput, NixError> {
    run_nix_command_with_options(args, None, DEFAULT_TIMEOUT_SECS).await
}

pub async fn run_nix_command_in_dir(
    args: &[&str],
    cwd: Option<&str>,
) -> Result<NixOutput, NixError> {
    run_nix_command_with_options(args, cwd, DEFAULT_TIMEOUT_SECS).await
}

pub async fn run_nix_command_with_timeout(
    args: &[&str],
    timeout_secs: u64,
) -> Result<NixOutput, NixError> {
    run_nix_command_with_options(args, None, timeout_secs).await
}

pub async fn run_nix_command_with_options(
    args: &[&str],
    cwd: Option<&str>,
    timeout_secs: u64,
) -> Result<NixOutput, NixError> {
    let mut cmd = Command::new("nix");
    cmd.args(args);
    cmd.kill_on_drop(true);

    if let Some(dir) = cwd {
        cmd.current_dir(dir);
    }

    let result = timeout(Duration::from_secs(timeout_secs), cmd.output()).await;

    match result {
        Ok(output_result) => {
            let output = output_result?;
            Ok(NixOutput {
                success: output.status.success(),
                stdout: String::from_utf8_lossy(&output.stdout).to_string(),
                stderr: String::from_utf8_lossy(&output.stderr).to_string(),
                exit_code: output.status.code(),
            })
        }
        Err(_) => Err(NixError::Timeout(timeout_secs)),
    }
}

pub fn parse_store_paths(stdout: &str) -> Vec<String> {
    stdout
        .lines()
        .filter(|line| line.starts_with("/nix/store/"))
        .map(|s| s.to_string())
        .collect()
}

pub fn parse_json_store_paths(stdout: &str) -> Vec<String> {
    if let Ok(value) = serde_json::from_str::<serde_json::Value>(stdout) {
        if let Some(arr) = value.as_array() {
            return arr
                .iter()
                .filter_map(|v| v.get("outputs"))
                .filter_map(|o| o.get("out"))
                .filter_map(|s| s.as_str())
                .map(|s| s.to_string())
                .collect();
        }
    }
    Vec::new()
}

pub async fn run_fh_command(args: &[&str]) -> Result<NixOutput, NixError> {
    run_fh_command_with_options(args, None, DEFAULT_TIMEOUT_SECS).await
}

pub async fn run_fh_command_in_dir(args: &[&str], cwd: Option<&str>) -> Result<NixOutput, NixError> {
    run_fh_command_with_options(args, cwd, DEFAULT_TIMEOUT_SECS).await
}

pub async fn run_fh_command_with_timeout(
    args: &[&str],
    timeout_secs: u64,
) -> Result<NixOutput, NixError> {
    run_fh_command_with_options(args, None, timeout_secs).await
}

pub async fn run_fh_command_with_options(
    args: &[&str],
    cwd: Option<&str>,
    timeout_secs: u64,
) -> Result<NixOutput, NixError> {
    let mut cmd = Command::new("fh");
    cmd.args(args);
    cmd.kill_on_drop(true);

    if let Some(dir) = cwd {
        cmd.current_dir(dir);
    }

    let result = timeout(Duration::from_secs(timeout_secs), cmd.output()).await;

    match result {
        Ok(output_result) => {
            let output = output_result?;
            Ok(NixOutput {
                success: output.status.success(),
                stdout: String::from_utf8_lossy(&output.stdout).to_string(),
                stderr: String::from_utf8_lossy(&output.stderr).to_string(),
                exit_code: output.status.code(),
            })
        }
        Err(_) => Err(NixError::Timeout(timeout_secs)),
    }
}

pub async fn run_cachix_command(args: &[&str]) -> Result<NixOutput, NixError> {
    run_cachix_command_with_env(args, &[], DEFAULT_TIMEOUT_SECS).await
}

pub async fn run_cachix_command_with_env(
    args: &[&str],
    env_vars: &[(&str, &str)],
    timeout_secs: u64,
) -> Result<NixOutput, NixError> {
    let mut cmd = Command::new("cachix");
    cmd.args(args);
    cmd.kill_on_drop(true);

    for (key, value) in env_vars {
        cmd.env(key, value);
    }

    let result = timeout(Duration::from_secs(timeout_secs), cmd.output()).await;

    match result {
        Ok(output_result) => {
            let output = output_result?;
            Ok(NixOutput {
                success: output.status.success(),
                stdout: String::from_utf8_lossy(&output.stdout).to_string(),
                stderr: String::from_utf8_lossy(&output.stderr).to_string(),
                exit_code: output.status.code(),
            })
        }
        Err(_) => Err(NixError::Timeout(timeout_secs)),
    }
}
