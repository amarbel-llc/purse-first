mod spawned;

pub use spawned::SpawnedLspClient;

use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use thiserror::Error;

#[derive(Debug, Error)]
pub enum LspError {
    #[error("Failed to spawn LSP process: {0}")]
    SpawnFailed(String),
    #[error("LSP communication error: {0}")]
    Communication(String),
    #[error("LSP protocol error: {0}")]
    Protocol(String),
    #[error("LSP request timeout after {0} seconds")]
    Timeout(u64),
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
    #[error("File not found: {0}")]
    FileNotFound(String),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Diagnostic {
    pub range: Range,
    pub severity: Option<DiagnosticSeverity>,
    pub message: String,
    pub source: Option<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
pub struct Range {
    pub start: Position,
    pub end: Position,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
pub struct Position {
    pub line: u32,
    pub character: u32,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(transparent)]
pub struct DiagnosticSeverity(pub u32);

impl DiagnosticSeverity {
    pub const ERROR: Self = Self(1);
    pub const WARNING: Self = Self(2);
    pub const INFORMATION: Self = Self(3);
    pub const HINT: Self = Self(4);

    pub fn as_str(&self) -> &'static str {
        match self.0 {
            1 => "error",
            2 => "warning",
            3 => "information",
            4 => "hint",
            _ => "unknown",
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompletionItem {
    pub label: String,
    pub kind: Option<u32>,
    pub detail: Option<String>,
    pub documentation: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HoverResult {
    pub contents: String,
    pub range: Option<Range>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Location {
    pub uri: String,
    pub range: Range,
}

#[async_trait]
pub trait LspClient: Send + Sync {
    async fn initialize(&mut self, root_uri: Option<&str>) -> Result<(), LspError>;
    async fn did_open(&mut self, uri: &str, text: &str) -> Result<(), LspError>;
    async fn diagnostics(&mut self, uri: &str) -> Result<Vec<Diagnostic>, LspError>;
    async fn completion(
        &mut self,
        uri: &str,
        line: u32,
        character: u32,
    ) -> Result<Vec<CompletionItem>, LspError>;
    async fn hover(
        &mut self,
        uri: &str,
        line: u32,
        character: u32,
    ) -> Result<Option<HoverResult>, LspError>;
    async fn goto_definition(
        &mut self,
        uri: &str,
        line: u32,
        character: u32,
    ) -> Result<Vec<Location>, LspError>;
    async fn shutdown(&mut self) -> Result<(), LspError>;
}

pub async fn create_nil_client() -> Result<impl LspClient, LspError> {
    SpawnedLspClient::new("nil").await
}
