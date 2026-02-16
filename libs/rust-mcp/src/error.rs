use thiserror::Error;

#[derive(Error, Debug)]
pub enum McpError {
    #[error("JSON-RPC error: {0}")]
    JsonRpc(String),

    #[error("Protocol error: {0}")]
    Protocol(String),

    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("{0}")]
    Other(String),
}

#[derive(Error, Debug)]
pub enum ServerError {
    #[error("Server initialization failed: {0}")]
    Initialization(String),

    #[error("Request handling failed: {0}")]
    RequestHandling(#[from] McpError),

    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Serialization error: {0}")]
    Serialization(#[from] serde_json::Error),
}

impl From<String> for McpError {
    fn from(s: String) -> Self {
        McpError::Other(s)
    }
}

impl From<&str> for McpError {
    fn from(s: &str) -> Self {
        McpError::Other(s.to_string())
    }
}
