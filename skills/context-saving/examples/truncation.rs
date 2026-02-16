// Minimal truncation example for an MCP tool returning text or JSON output.
//
// This shows the core pattern: capture raw output, apply line/byte limits,
// attempt JSON parse with string fallback, conditionally include TruncationInfo.

use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct TruncationInfo {
    pub original_bytes: usize,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub original_lines: Option<usize>,
    pub kept_bytes: usize,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub kept_lines: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub position: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct EvalResult {
    pub success: bool,
    pub value: serde_json::Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncated: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub truncation_info: Option<TruncationInfo>,
}

pub fn truncate_output(
    raw: &str,
    head: Option<usize>,
    tail: Option<usize>,
    max_bytes: Option<usize>,
) -> (String, bool, Option<TruncationInfo>) {
    let original_bytes = raw.len();
    let lines: Vec<&str> = raw.lines().collect();
    let original_lines = lines.len();

    // Apply head/tail (mutually exclusive, head wins)
    let mut result_lines = lines.clone();
    let mut position = None;

    if let Some(h) = head {
        if h < result_lines.len() {
            result_lines = result_lines.into_iter().take(h).collect();
            position = Some("head".to_string());
        }
    } else if let Some(t) = tail {
        if t < result_lines.len() {
            result_lines = result_lines.into_iter().rev().take(t).rev().collect();
            position = Some("tail".to_string());
        }
    }

    let mut content = result_lines.join("\n");

    // Apply max_bytes (truncate at line boundary)
    if let Some(mb) = max_bytes {
        if content.len() > mb {
            if let Some(nl) = content[..mb].rfind('\n') {
                content = content[..nl].to_string();
            } else {
                content = content[..mb].to_string();
            }
            if position.is_none() {
                position = Some("head".to_string());
            }
        }
    }

    let kept_bytes = content.len();
    let kept_lines = content.lines().count();
    let truncated = kept_bytes < original_bytes;

    let info = if truncated {
        Some(TruncationInfo {
            original_bytes,
            original_lines: Some(original_lines),
            kept_bytes,
            kept_lines: Some(kept_lines),
            position,
        })
    } else {
        None
    };

    (content, truncated, info)
}

/// Apply truncation to JSON output, falling back to string on broken JSON.
pub fn eval_with_truncation(
    raw_stdout: &str,
    head: Option<usize>,
    tail: Option<usize>,
    max_bytes: Option<usize>,
) -> EvalResult {
    let (content, truncated, info) = truncate_output(raw_stdout, head, tail, max_bytes);

    let value = serde_json::from_str(&content)
        .unwrap_or(serde_json::Value::String(content));

    EvalResult {
        success: true,
        value,
        truncated: if truncated { Some(true) } else { None },
        truncation_info: info,
    }
}
