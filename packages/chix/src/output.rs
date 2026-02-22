use serde::{Deserialize, Serialize};
use serde_json::Value;

/// Configuration for output limiting, loaded from config file
#[derive(Debug, Clone, Deserialize, Default)]
pub struct OutputLimitsConfig {
    /// Default maximum bytes for text output (default: 100_000)
    pub default_max_bytes: Option<usize>,
    /// Default maximum lines for text output (default: 2000)
    pub default_max_lines: Option<usize>,
    /// Default maximum items for JSON arrays (default: 100)
    pub default_max_items: Option<usize>,
    /// Default tail lines for log output (default: 500)
    pub log_tail_default: Option<usize>,
    /// Default limit for search results (default: 50)
    pub search_limit_default: Option<usize>,
}

impl OutputLimitsConfig {
    pub fn default_max_bytes(&self) -> usize {
        self.default_max_bytes.unwrap_or(100_000)
    }

    pub fn default_max_lines(&self) -> usize {
        self.default_max_lines.unwrap_or(2000)
    }

    pub fn default_max_items(&self) -> usize {
        self.default_max_items.unwrap_or(100)
    }

    pub fn log_tail_default(&self) -> usize {
        self.log_tail_default.unwrap_or(500)
    }

    pub fn search_limit_default(&self) -> usize {
        self.search_limit_default.unwrap_or(50)
    }
}

/// Parameters for limiting text output
#[derive(Debug, Clone, Default, Deserialize)]
pub struct OutputLimits {
    /// Return only first N lines
    pub head: Option<usize>,
    /// Return only last N lines
    pub tail: Option<usize>,
    /// Maximum output size in bytes
    pub max_bytes: Option<usize>,
    /// Maximum lines to return (applied after head/tail)
    pub max_lines: Option<usize>,
}

/// Parameters for limiting JSON array output
#[derive(Debug, Clone, Default, Deserialize)]
pub struct ArrayLimits {
    /// Maximum items to return
    pub limit: Option<usize>,
    /// Skip first N items (for pagination)
    pub offset: Option<usize>,
}

/// Information about truncation that occurred
#[derive(Debug, Clone, Serialize)]
pub struct TruncationInfo {
    /// Original size in bytes
    pub original_bytes: usize,
    /// Original line count (if applicable)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub original_lines: Option<usize>,
    /// Original item count (if applicable)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub original_items: Option<usize>,
    /// Bytes kept after truncation
    pub kept_bytes: usize,
    /// Lines kept after truncation
    #[serde(skip_serializing_if = "Option::is_none")]
    pub kept_lines: Option<usize>,
    /// Items kept after truncation
    #[serde(skip_serializing_if = "Option::is_none")]
    pub kept_items: Option<usize>,
    /// Position of kept content: "head", "tail", or "middle"
    #[serde(skip_serializing_if = "Option::is_none")]
    pub position: Option<String>,
}

/// Result of limiting text output
#[derive(Debug, Clone)]
pub struct LimitedOutput {
    /// The (possibly truncated) content
    pub content: String,
    /// Whether truncation occurred
    pub truncated: bool,
    /// Details about the truncation
    pub truncation_info: Option<TruncationInfo>,
}

/// Result of limiting a JSON array
#[derive(Debug, Clone)]
pub struct LimitedArray {
    /// The (possibly truncated) items
    pub items: Vec<Value>,
    /// Whether truncation occurred
    pub truncated: bool,
    /// Total count of items before limiting
    pub total_count: usize,
    /// Pagination info
    pub pagination: PaginationInfo,
}

/// Pagination metadata for array results
#[derive(Debug, Clone, Serialize)]
pub struct PaginationInfo {
    /// Current offset
    pub offset: usize,
    /// Limit applied
    pub limit: usize,
    /// Total available items
    pub total: usize,
    /// Whether more items are available
    pub has_more: bool,
}

/// Apply default max_bytes truncation to stderr
pub fn limit_stderr(input: &str) -> LimitedOutput {
    let config = OutputLimitsConfig::default();
    let limits = OutputLimits {
        max_bytes: Some(config.default_max_bytes()),
        ..Default::default()
    };
    limit_text_output(input, &limits)
}

/// Apply limits to text output
pub fn limit_text_output(input: &str, limits: &OutputLimits) -> LimitedOutput {
    let original_bytes = input.len();
    let lines: Vec<&str> = input.lines().collect();
    let original_lines = lines.len();

    // Start with all lines
    let mut result_lines: Vec<&str> = lines.clone();
    let mut position = None;

    // Apply head/tail first (mutually exclusive, head takes priority)
    if let Some(head) = limits.head {
        if head < result_lines.len() {
            result_lines = result_lines.into_iter().take(head).collect();
            position = Some("head".to_string());
        }
    } else if let Some(tail) = limits.tail {
        if tail < result_lines.len() {
            result_lines = result_lines.into_iter().rev().take(tail).rev().collect();
            position = Some("tail".to_string());
        }
    }

    // Apply max_lines limit
    if let Some(max_lines) = limits.max_lines {
        if max_lines < result_lines.len() {
            result_lines = result_lines.into_iter().take(max_lines).collect();
            if position.is_none() {
                position = Some("head".to_string());
            }
        }
    }

    let mut content = result_lines.join("\n");
    let kept_lines = result_lines.len();

    // Apply max_bytes limit (truncate at byte boundary, trying to preserve whole lines)
    if let Some(max_bytes) = limits.max_bytes {
        if content.len() > max_bytes {
            // Try to truncate at a line boundary
            let truncated = &content[..max_bytes];
            if let Some(last_newline) = truncated.rfind('\n') {
                content = truncated[..last_newline].to_string();
            } else {
                // No newline found, just truncate at byte boundary
                // But ensure we don't break a UTF-8 character
                content = truncated
                    .char_indices()
                    .take_while(|(i, _)| *i < max_bytes)
                    .map(|(_, c)| c)
                    .collect();
            }
            if position.is_none() {
                position = Some("head".to_string());
            }
        }
    }

    let kept_bytes = content.len();
    let truncated = kept_bytes < original_bytes || kept_lines < original_lines;

    let truncation_info = if truncated {
        Some(TruncationInfo {
            original_bytes,
            original_lines: Some(original_lines),
            original_items: None,
            kept_bytes,
            kept_lines: Some(content.lines().count()),
            kept_items: None,
            position,
        })
    } else {
        None
    };

    LimitedOutput {
        content,
        truncated,
        truncation_info,
    }
}

/// Apply limits to a JSON array
pub fn limit_json_array(items: Vec<Value>, limits: &ArrayLimits) -> LimitedArray {
    let total_count = items.len();
    let offset = limits.offset.unwrap_or(0);
    let limit = limits.limit.unwrap_or(total_count);

    let result_items: Vec<Value> = items.into_iter().skip(offset).take(limit).collect();

    let kept_count = result_items.len();
    let has_more = offset + kept_count < total_count;

    LimitedArray {
        items: result_items,
        truncated: kept_count < total_count || offset > 0,
        total_count,
        pagination: PaginationInfo {
            offset,
            limit,
            total: total_count,
            has_more,
        },
    }
}

/// Merge user-provided limits with config defaults
pub fn merge_limits_with_config(
    user_limits: Option<OutputLimits>,
    config: &OutputLimitsConfig,
) -> OutputLimits {
    match user_limits {
        Some(limits) => OutputLimits {
            head: limits.head,
            tail: limits.tail,
            max_bytes: limits.max_bytes.or(Some(config.default_max_bytes())),
            max_lines: limits.max_lines.or(Some(config.default_max_lines())),
        },
        None => OutputLimits {
            head: None,
            tail: None,
            max_bytes: Some(config.default_max_bytes()),
            max_lines: Some(config.default_max_lines()),
        },
    }
}

/// Merge user-provided array limits with config defaults
pub fn merge_array_limits_with_config(
    user_limits: Option<ArrayLimits>,
    default_limit: usize,
) -> ArrayLimits {
    match user_limits {
        Some(limits) => ArrayLimits {
            limit: limits.limit.or(Some(default_limit)),
            offset: limits.offset,
        },
        None => ArrayLimits {
            limit: Some(default_limit),
            offset: Some(0),
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_limit_text_no_truncation() {
        let input = "line1\nline2\nline3";
        let limits = OutputLimits::default();
        let result = limit_text_output(input, &limits);

        assert!(!result.truncated);
        assert_eq!(result.content, input);
        assert!(result.truncation_info.is_none());
    }

    #[test]
    fn test_limit_text_head() {
        let input = "line1\nline2\nline3\nline4\nline5";
        let limits = OutputLimits {
            head: Some(2),
            ..Default::default()
        };
        let result = limit_text_output(input, &limits);

        assert!(result.truncated);
        assert_eq!(result.content, "line1\nline2");
        let info = result.truncation_info.unwrap();
        assert_eq!(info.original_lines, Some(5));
        assert_eq!(info.kept_lines, Some(2));
        assert_eq!(info.position, Some("head".to_string()));
    }

    #[test]
    fn test_limit_text_tail() {
        let input = "line1\nline2\nline3\nline4\nline5";
        let limits = OutputLimits {
            tail: Some(2),
            ..Default::default()
        };
        let result = limit_text_output(input, &limits);

        assert!(result.truncated);
        assert_eq!(result.content, "line4\nline5");
        let info = result.truncation_info.unwrap();
        assert_eq!(info.position, Some("tail".to_string()));
    }

    #[test]
    fn test_limit_text_max_bytes() {
        let input = "line1\nline2\nline3";
        let limits = OutputLimits {
            max_bytes: Some(10),
            ..Default::default()
        };
        let result = limit_text_output(input, &limits);

        assert!(result.truncated);
        // Should truncate at line boundary
        assert_eq!(result.content, "line1");
    }

    #[test]
    fn test_limit_text_max_lines() {
        let input = "line1\nline2\nline3\nline4\nline5";
        let limits = OutputLimits {
            max_lines: Some(3),
            ..Default::default()
        };
        let result = limit_text_output(input, &limits);

        assert!(result.truncated);
        assert_eq!(result.content, "line1\nline2\nline3");
    }

    #[test]
    fn test_limit_json_array_no_truncation() {
        let items = vec![
            serde_json::json!({"a": 1}),
            serde_json::json!({"a": 2}),
            serde_json::json!({"a": 3}),
        ];
        let limits = ArrayLimits::default();
        let result = limit_json_array(items.clone(), &limits);

        assert!(!result.truncated);
        assert_eq!(result.items.len(), 3);
        assert_eq!(result.total_count, 3);
        assert!(!result.pagination.has_more);
    }

    #[test]
    fn test_limit_json_array_with_limit() {
        let items = vec![
            serde_json::json!({"a": 1}),
            serde_json::json!({"a": 2}),
            serde_json::json!({"a": 3}),
            serde_json::json!({"a": 4}),
            serde_json::json!({"a": 5}),
        ];
        let limits = ArrayLimits {
            limit: Some(2),
            offset: None,
        };
        let result = limit_json_array(items, &limits);

        assert!(result.truncated);
        assert_eq!(result.items.len(), 2);
        assert_eq!(result.total_count, 5);
        assert!(result.pagination.has_more);
        assert_eq!(result.pagination.offset, 0);
        assert_eq!(result.pagination.limit, 2);
    }

    #[test]
    fn test_limit_json_array_with_offset() {
        let items = vec![
            serde_json::json!({"a": 1}),
            serde_json::json!({"a": 2}),
            serde_json::json!({"a": 3}),
            serde_json::json!({"a": 4}),
            serde_json::json!({"a": 5}),
        ];
        let limits = ArrayLimits {
            limit: Some(2),
            offset: Some(2),
        };
        let result = limit_json_array(items, &limits);

        assert!(result.truncated);
        assert_eq!(result.items.len(), 2);
        assert_eq!(result.items[0], serde_json::json!({"a": 3}));
        assert_eq!(result.items[1], serde_json::json!({"a": 4}));
        assert!(result.pagination.has_more);
        assert_eq!(result.pagination.offset, 2);
    }

    #[test]
    fn test_limit_json_array_offset_at_end() {
        let items = vec![
            serde_json::json!({"a": 1}),
            serde_json::json!({"a": 2}),
            serde_json::json!({"a": 3}),
        ];
        let limits = ArrayLimits {
            limit: Some(2),
            offset: Some(2),
        };
        let result = limit_json_array(items, &limits);

        assert_eq!(result.items.len(), 1);
        assert!(!result.pagination.has_more);
    }

    #[test]
    fn test_config_defaults() {
        let config = OutputLimitsConfig::default();
        assert_eq!(config.default_max_bytes(), 100_000);
        assert_eq!(config.default_max_lines(), 2000);
        assert_eq!(config.default_max_items(), 100);
        assert_eq!(config.log_tail_default(), 500);
        assert_eq!(config.search_limit_default(), 50);
    }

    #[test]
    fn test_merge_limits_with_config() {
        let config = OutputLimitsConfig {
            default_max_bytes: Some(50_000),
            default_max_lines: Some(1000),
            ..Default::default()
        };

        // No user limits - use config defaults
        let result = merge_limits_with_config(None, &config);
        assert_eq!(result.max_bytes, Some(50_000));
        assert_eq!(result.max_lines, Some(1000));

        // User limits override config
        let user_limits = OutputLimits {
            max_bytes: Some(10_000),
            ..Default::default()
        };
        let result = merge_limits_with_config(Some(user_limits), &config);
        assert_eq!(result.max_bytes, Some(10_000));
        assert_eq!(result.max_lines, Some(1000)); // Falls back to config
    }
}
