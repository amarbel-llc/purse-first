//! # mcp-server
//!
//! A reusable Rust library for building MCP (Model Context Protocol) servers.
//!
//! This library provides mid-level components for implementing MCP servers with support for:
//! - **Tools**: Execute commands and operations with structured input/output
//! - **Resources**: URI-based access to data and content
//! - **Prompts**: Template-based prompt generation for LLM interactions
//! - **Sampling**: Server-initiated LLM requests (experimental)
//!
//! ## Example
//!
//! ```no_run
//! use mcp_server::{McpServer, Tool, ToolResult, Context, Content, ToolError};
//! use serde_json::{json, Value};
//!
//! struct HelloTool;
//!
//! #[async_trait::async_trait]
//! impl Tool for HelloTool {
//!     fn name(&self) -> &str { "hello" }
//!     fn description(&self) -> &str { "Says hello" }
//!     fn input_schema(&self) -> Value {
//!         json!({
//!             "type": "object",
//!             "properties": {
//!                 "name": { "type": "string" }
//!             }
//!         })
//!     }
//!
//!     async fn execute(&self, arguments: Value, _ctx: &Context)
//!         -> Result<ToolResult, ToolError>
//!     {
//!         let name = arguments.get("name")
//!             .and_then(|v| v.as_str())
//!             .unwrap_or("World");
//!
//!         Ok(ToolResult {
//!             content: vec![Content::Text {
//!                 text: format!("Hello, {}!", name),
//!             }],
//!             is_error: None,
//!         })
//!     }
//! }
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     let server = McpServer::builder("hello-server", "0.1.0")
//!         .with_tool(HelloTool)
//!         .build();
//!
//!     server.run_stdio().await?;
//!     Ok(())
//! }
//! ```

pub mod error;
pub mod protocol;
pub mod server;

#[cfg(feature = "tools")]
pub mod tools;

#[cfg(feature = "resources")]
pub mod resources;

#[cfg(feature = "prompts")]
pub mod prompts;

#[cfg(feature = "sampling")]
pub mod sampling;

#[cfg(feature = "completions")]
pub mod completions;

#[cfg(feature = "command-executor")]
pub mod executor;

#[cfg(feature = "command")]
pub mod command;

pub mod hooks;
pub mod validation;

// Re-export main types
pub use error::{McpError, ServerError};
pub use protocol::{Content, ContentType};
pub use server::{Context, McpServer, McpServerBuilder};

#[cfg(feature = "tools")]
pub use tools::{Tool, ToolError, ToolRegistry, ToolResult};
#[cfg(feature = "tools")]
pub use tools::{ToolAnnotations, ToolExecution, ToolResultV1, ToolV1};

#[cfg(feature = "resources")]
pub use resources::{Resource, ResourceContent, ResourceError, ResourceRegistry};
#[cfg(feature = "resources")]
pub use resources::{ResourceInfoV1, ResourceV1};

#[cfg(feature = "prompts")]
pub use prompts::{Prompt, PromptError, PromptMessage, PromptRegistry};
#[cfg(feature = "prompts")]
pub use prompts::{PromptMessageV1, PromptV1};

#[cfg(feature = "sampling")]
pub use sampling::{
    CreateMessageRequest, CreateMessageResult, ModelPreferences, SamplingError, SamplingHandler,
};

#[cfg(feature = "completions")]
pub use completions::{CompletionError, CompletionProvider, CompletionRegistry};

#[cfg(feature = "command-executor")]
pub use executor::{CommandExecutor, CommandOutput, ExecuteArgs, ExecutorError, TokioExecutor};

#[cfg(feature = "command")]
pub use command::{App, Command as CliCommand, Description, Param, ParamType, PostToolUseHook};

pub use hooks::{HookHandler, ToolMapping};
