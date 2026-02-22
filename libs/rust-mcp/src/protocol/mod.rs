/// Protocol version constants.
pub const PROTOCOL_VERSION_V0: &str = "2024-11-05";
pub const PROTOCOL_VERSION_V1: &str = "2025-11-25";
pub const PROTOCOL_VERSION: &str = PROTOCOL_VERSION_V0;

// V0 modules (original types).
pub mod capabilities_v0;
pub mod content_v0;
pub mod initialize_v0;
pub mod jsonrpc;

// V1 modules (new types).
pub mod capabilities_v1;
pub mod content_v1;
pub mod icons;
pub mod initialize_v1;
pub mod methods;
pub mod completions;
pub mod pagination;

// Re-export V0 types as the default types for backward compatibility.
pub use capabilities_v0 as capabilities;
pub use content_v0 as content;
pub use initialize_v0 as initialize;

pub use capabilities_v0::{Capabilities, ResourcesCapability, ToolsCapability};
pub use content_v0::{Content, ContentType};
pub use initialize_v0::{ClientCapabilities, InitializeResult, ServerInfo};
pub use jsonrpc::{JsonRpcError, JsonRpcRequest, JsonRpcResponse};

// V1 re-exports for convenience.
pub use capabilities_v1::CapabilitiesV1;
pub use content_v1::{ContentAnnotations, ContentV1};
pub use icons::Icon;
pub use initialize_v1::{InitializeResultV1, ServerInfoV1};
pub use methods::*;
pub use pagination::{PaginatedResult, PaginationParams};
pub use completions::{
    CompletionArgument, CompletionCompleteParams, CompletionReference, CompletionResult,
    CompletionValues,
};

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn version_constants() {
        assert_eq!(PROTOCOL_VERSION_V0, "2024-11-05");
        assert_eq!(PROTOCOL_VERSION_V1, "2025-11-25");
        assert_eq!(PROTOCOL_VERSION, PROTOCOL_VERSION_V0);
    }

    #[test]
    fn v0_content_text_serialization() {
        let content = Content::text("hello");
        let json = serde_json::to_string(&content).unwrap();
        assert!(json.contains("\"text\""));
        assert!(json.contains("hello"));

        let decoded: Content = serde_json::from_str(&json).unwrap();
        match decoded {
            Content::Text { text } => assert_eq!(text, "hello"),
            _ => panic!("expected Text variant"),
        }
    }

    #[test]
    fn v1_content_text_serialization() {
        let content = ContentV1::text("hello");
        let json = serde_json::to_string(&content).unwrap();
        assert!(json.contains("hello"));

        let decoded: ContentV1 = serde_json::from_str(&json).unwrap();
        match decoded {
            ContentV1::Text { text, annotations } => {
                assert_eq!(text, "hello");
                assert!(annotations.is_none());
            }
            _ => panic!("expected Text variant"),
        }
    }

    #[test]
    fn v1_content_with_annotations() {
        let content = ContentV1::Text {
            text: "annotated".to_string(),
            annotations: Some(ContentAnnotations {
                audience: Some(vec!["user".to_string()]),
                priority: Some(0.8),
                last_modified: None,
            }),
        };

        let json = serde_json::to_string(&content).unwrap();
        assert!(json.contains("audience"));
        assert!(json.contains("priority"));

        let decoded: ContentV1 = serde_json::from_str(&json).unwrap();
        match decoded {
            ContentV1::Text { annotations, .. } => {
                let ann = annotations.unwrap();
                assert_eq!(ann.audience.unwrap(), vec!["user"]);
                assert!((ann.priority.unwrap() - 0.8).abs() < f64::EPSILON);
            }
            _ => panic!("expected Text variant"),
        }
    }

    #[test]
    fn v0_to_v1_content_upgrade() {
        let v0 = Content::text("hello");
        let v1 = ContentV1::from_v0(v0);
        match v1 {
            ContentV1::Text { text, annotations } => {
                assert_eq!(text, "hello");
                assert!(annotations.is_none());
            }
            _ => panic!("expected Text variant"),
        }
    }

    #[test]
    fn capabilities_v0_builder() {
        let caps = Capabilities::new().with_tools().with_resources();
        assert!(caps.tools.is_some());
        assert!(caps.resources.is_some());
        assert!(caps.prompts.is_none());
    }

    #[test]
    fn capabilities_v1_builder() {
        let caps = CapabilitiesV1::new()
            .with_tools()
            .with_logging()
            .with_completions();
        assert!(caps.tools.is_some());
        assert!(caps.logging.is_some());
        assert!(caps.completions.is_some());
        assert!(caps.resources.is_none());
        assert!(caps.tasks.is_none());
    }

    #[test]
    fn initialize_result_v1_serialization() {
        let result = InitializeResultV1 {
            protocol_version: PROTOCOL_VERSION_V1.to_string(),
            capabilities: CapabilitiesV1::new().with_tools(),
            server_info: ServerInfoV1 {
                name: "test".to_string(),
                version: "1.0".to_string(),
                title: Some("Test Server".to_string()),
                description: None,
                icons: None,
                website_url: None,
            },
            instructions: Some("Use this server".to_string()),
        };

        let json = serde_json::to_string(&result).unwrap();
        assert!(json.contains(PROTOCOL_VERSION_V1));
        assert!(json.contains("Test Server"));
        assert!(json.contains("Use this server"));
    }

    #[test]
    fn icon_serialization() {
        let icon = Icon {
            src: "https://example.com/icon.png".to_string(),
            mime_type: Some("image/png".to_string()),
            sizes: Some(vec!["64x64".to_string()]),
            theme: None,
        };

        let json = serde_json::to_string(&icon).unwrap();
        assert!(json.contains("https://example.com/icon.png"));
        assert!(json.contains("image/png"));
        assert!(!json.contains("theme")); // should be omitted
    }

    #[test]
    fn pagination_params_serialization() {
        let params = PaginationParams {
            cursor: Some("cursor123".to_string()),
        };
        let json = serde_json::to_string(&params).unwrap();
        assert!(json.contains("cursor123"));

        let decoded: PaginationParams = serde_json::from_str(&json).unwrap();
        assert_eq!(decoded.cursor.unwrap(), "cursor123");
    }

    #[test]
    fn completion_params_serialization() {
        use completions::*;

        let params = CompletionCompleteParams {
            r#ref: CompletionReference {
                ref_type: "ref/prompt".to_string(),
                name: Some("my-prompt".to_string()),
                uri: None,
            },
            argument: CompletionArgument {
                name: "arg1".to_string(),
                value: "partial".to_string(),
            },
        };

        let json = serde_json::to_string(&params).unwrap();
        assert!(json.contains("ref/prompt"));
        assert!(json.contains("my-prompt"));
        assert!(json.contains("partial"));

        let decoded: CompletionCompleteParams = serde_json::from_str(&json).unwrap();
        assert_eq!(decoded.r#ref.ref_type, "ref/prompt");
        assert_eq!(decoded.argument.name, "arg1");
    }

    #[test]
    fn completion_result_serialization() {
        use completions::*;

        let result = CompletionResult {
            completion: CompletionValues {
                values: vec!["foo".to_string(), "foobar".to_string()],
                total: Some(10),
                has_more: Some(true),
            },
        };

        let json = serde_json::to_string(&result).unwrap();
        assert!(json.contains("foo"));
        assert!(json.contains("foobar"));
        assert!(json.contains("\"total\":10"));
        assert!(json.contains("\"hasMore\":true"));

        let decoded: CompletionResult = serde_json::from_str(&json).unwrap();
        assert_eq!(decoded.completion.values.len(), 2);
        assert_eq!(decoded.completion.total, Some(10));
        assert_eq!(decoded.completion.has_more, Some(true));
    }

    #[test]
    fn completion_result_minimal() {
        use completions::*;

        let result = CompletionResult {
            completion: CompletionValues {
                values: vec!["only".to_string()],
                total: None,
                has_more: None,
            },
        };

        let json = serde_json::to_string(&result).unwrap();
        assert!(!json.contains("total"));
        assert!(!json.contains("hasMore"));
    }
}
