# rust-mcp Shell Completion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add CLI shell completion generation and MCP protocol `completion/complete` handler to `libs/rust-mcp`.

**Architecture:** Two independent Cargo features: `command` (CLI framework with shell completion generation) and `completions` (MCP protocol completion handler). Both follow rust-mcp's existing trait → registry → dispatcher pattern. The `command` module is purely synchronous — no async needed.

**Tech Stack:** Rust, serde, async-trait, tokio (for completions handler tests)

---

### Task 1: Protocol types for MCP completions

**Files:**
- Create: `libs/rust-mcp/src/protocol/completions.rs`
- Modify: `libs/rust-mcp/src/protocol/mod.rs:8` (add module declaration)

**Step 1: Write the failing test**

Add to the bottom of `libs/rust-mcp/src/protocol/mod.rs` inside `mod tests`:

```rust
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
            has_more: true,
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
    assert!(decoded.completion.has_more);
}

#[test]
fn completion_result_minimal() {
    use completions::*;

    let result = CompletionResult {
        completion: CompletionValues {
            values: vec!["only".to_string()],
            total: None,
            has_more: false,
        },
    };

    let json = serde_json::to_string(&result).unwrap();
    assert!(!json.contains("total"));
    assert!(json.contains("\"hasMore\":false"));
}
```

**Step 2: Run test to verify it fails**

Run: `cargo test -p mcp-server --lib protocol::tests::completion_params_serialization`
Expected: FAIL — module `completions` not found

**Step 3: Write the implementation**

Create `libs/rust-mcp/src/protocol/completions.rs`:

```rust
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CompletionReference {
    #[serde(rename = "type")]
    pub ref_type: String,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub uri: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CompletionArgument {
    pub name: String,
    pub value: String,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CompletionCompleteParams {
    pub r#ref: CompletionReference,
    pub argument: CompletionArgument,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CompletionResult {
    pub completion: CompletionValues,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct CompletionValues {
    pub values: Vec<String>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub total: Option<usize>,

    #[serde(rename = "hasMore")]
    pub has_more: bool,
}
```

Add to `libs/rust-mcp/src/protocol/mod.rs` after `pub mod pagination;` (line 18):

```rust
pub mod completions;
```

Add re-export after the existing V1 re-exports (after line 36):

```rust
pub use completions::{
    CompletionArgument, CompletionCompleteParams, CompletionReference, CompletionResult,
    CompletionValues,
};
```

**Step 4: Run tests to verify they pass**

Run: `cargo test -p mcp-server --lib protocol::tests::completion`
Expected: All 3 completion tests PASS

**Step 5: Commit**

```
feat(rust-mcp): add MCP completion protocol types
```

---

### Task 2: CompletionProvider trait and registry

**Files:**
- Create: `libs/rust-mcp/src/completions/mod.rs`
- Create: `libs/rust-mcp/src/completions/handler.rs`
- Create: `libs/rust-mcp/src/completions/registry.rs`
- Modify: `libs/rust-mcp/src/lib.rs:74` (add module declaration)

**Step 1: Write the failing test**

Add to `libs/rust-mcp/src/completions/registry.rs`:

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use crate::protocol::completions::*;
    use crate::server::Context;

    struct TestProvider;

    #[async_trait::async_trait]
    impl CompletionProvider for TestProvider {
        async fn complete(
            &self,
            params: CompletionCompleteParams,
            _ctx: &Context,
        ) -> Result<CompletionResult, CompletionError> {
            let prefix = &params.argument.value;
            let values: Vec<String> = vec!["foo", "foobar", "baz"]
                .into_iter()
                .filter(|v| v.starts_with(prefix))
                .map(String::from)
                .collect();

            Ok(CompletionResult {
                completion: CompletionValues {
                    values,
                    total: None,
                    has_more: false,
                },
            })
        }
    }

    #[tokio::test]
    async fn registry_delegates_to_provider() {
        let mut registry = CompletionRegistry::new();
        registry.register(TestProvider);

        let params = CompletionCompleteParams {
            r#ref: CompletionReference {
                ref_type: "ref/prompt".to_string(),
                name: Some("test".to_string()),
                uri: None,
            },
            argument: CompletionArgument {
                name: "arg".to_string(),
                value: "fo".to_string(),
            },
        };

        let ctx = Context::new();
        let result = registry.complete(params, &ctx).await.unwrap();
        assert_eq!(result.completion.values, vec!["foo", "foobar"]);
    }

    #[tokio::test]
    async fn registry_empty_returns_empty() {
        let registry = CompletionRegistry::new();

        let params = CompletionCompleteParams {
            r#ref: CompletionReference {
                ref_type: "ref/prompt".to_string(),
                name: Some("missing".to_string()),
                uri: None,
            },
            argument: CompletionArgument {
                name: "arg".to_string(),
                value: "x".to_string(),
            },
        };

        let ctx = Context::new();
        let result = registry.complete(params, &ctx).await.unwrap();
        assert!(result.completion.values.is_empty());
    }
}
```

**Step 2: Run test to verify it fails**

Run: `cargo test -p mcp-server --features completions --lib completions::registry::tests`
Expected: FAIL — module `completions` not found

**Step 3: Write the implementation**

Create `libs/rust-mcp/src/completions/handler.rs`:

```rust
use crate::protocol::completions::{CompletionCompleteParams, CompletionResult};
use crate::server::Context;
use async_trait::async_trait;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum CompletionError {
    #[error("Completion failed: {0}")]
    Failed(String),

    #[error("{0}")]
    Other(String),
}

impl From<String> for CompletionError {
    fn from(s: String) -> Self {
        CompletionError::Other(s)
    }
}

#[async_trait]
pub trait CompletionProvider: Send + Sync {
    async fn complete(
        &self,
        params: CompletionCompleteParams,
        ctx: &Context,
    ) -> Result<CompletionResult, CompletionError>;
}
```

Create `libs/rust-mcp/src/completions/registry.rs`:

```rust
use super::handler::{CompletionError, CompletionProvider};
use crate::protocol::completions::{
    CompletionCompleteParams, CompletionResult, CompletionValues,
};
use crate::server::Context;
use std::sync::Arc;

pub struct CompletionRegistry {
    provider: Option<Arc<dyn CompletionProvider>>,
}

impl CompletionRegistry {
    pub fn new() -> Self {
        CompletionRegistry { provider: None }
    }

    pub fn register<P: CompletionProvider + 'static>(&mut self, provider: P) {
        self.provider = Some(Arc::new(provider));
    }

    pub fn has_provider(&self) -> bool {
        self.provider.is_some()
    }

    pub async fn complete(
        &self,
        params: CompletionCompleteParams,
        ctx: &Context,
    ) -> Result<CompletionResult, CompletionError> {
        match &self.provider {
            Some(provider) => provider.complete(params, ctx).await,
            None => Ok(CompletionResult {
                completion: CompletionValues {
                    values: vec![],
                    total: None,
                    has_more: false,
                },
            }),
        }
    }
}

impl Default for CompletionRegistry {
    fn default() -> Self {
        Self::new()
    }
}
```

Create `libs/rust-mcp/src/completions/mod.rs`:

```rust
pub mod handler;
pub mod registry;

pub use handler::{CompletionError, CompletionProvider};
pub use registry::CompletionRegistry;
```

Add to `libs/rust-mcp/src/lib.rs` after the `sampling` module (after line 73):

```rust
#[cfg(feature = "completions")]
pub mod completions;
```

Add re-exports after the sampling re-exports (after line 103):

```rust
#[cfg(feature = "completions")]
pub use completions::{CompletionError, CompletionProvider, CompletionRegistry};
```

**Step 4: Run tests to verify they pass**

Run: `cargo test -p mcp-server --features completions --lib completions::registry::tests`
Expected: Both tests PASS

**Step 5: Commit**

```
feat(rust-mcp): add CompletionProvider trait and registry
```

---

### Task 3: Wire completions into McpServerBuilder and dispatcher

**Files:**
- Modify: `libs/rust-mcp/src/server/mod.rs:28-31` (add completion_registry field to builder)
- Modify: `libs/rust-mcp/src/server/mod.rs:134-231` (add to build method)
- Modify: `libs/rust-mcp/src/server/dispatcher.rs:26-48` (add field to McpServer)
- Modify: `libs/rust-mcp/src/server/dispatcher.rs:86-131` (add dispatch arm)

**Step 1: Write the failing test**

Add to the bottom of `libs/rust-mcp/src/server/dispatcher.rs`:

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use crate::server::McpServerBuilder;

    fn init_v1(server: &McpServer) -> Context {
        let mut ctx = Context::new();
        ctx.negotiated_version = PROTOCOL_VERSION_V1.to_string();
        ctx
    }

    #[cfg(feature = "completions")]
    mod completion_tests {
        use super::*;
        use crate::completions::CompletionProvider;
        use crate::protocol::completions::*;

        struct TestCompleter;

        #[async_trait::async_trait]
        impl CompletionProvider for TestCompleter {
            async fn complete(
                &self,
                params: CompletionCompleteParams,
                _ctx: &Context,
            ) -> Result<CompletionResult, crate::completions::CompletionError> {
                Ok(CompletionResult {
                    completion: CompletionValues {
                        values: vec![format!("completed-{}", params.argument.value)],
                        total: None,
                        has_more: false,
                    },
                })
            }
        }

        #[tokio::test]
        async fn completion_complete_dispatches() {
            let server = McpServerBuilder::new("test", "0.1.0")
                .with_completion_provider(TestCompleter)
                .build();

            let mut ctx = init_v1(&server);

            let request = r#"{"jsonrpc":"2.0","id":1,"method":"completion/complete","params":{"ref":{"type":"ref/prompt","name":"p"},"argument":{"name":"a","value":"hello"}}}"#;
            let response = server.handle_request(request, &mut ctx).await;

            let result = response.get("result").expect("should have result");
            let values = result["completion"]["values"]
                .as_array()
                .expect("should have values");
            assert_eq!(values[0].as_str().unwrap(), "completed-hello");
        }

        #[tokio::test]
        async fn completion_complete_without_provider_returns_empty() {
            let server = McpServerBuilder::new("test", "0.1.0")
                .instructions("test")
                .build();

            let mut ctx = init_v1(&server);

            let request = r#"{"jsonrpc":"2.0","id":1,"method":"completion/complete","params":{"ref":{"type":"ref/prompt","name":"p"},"argument":{"name":"a","value":"x"}}}"#;
            let response = server.handle_request(request, &mut ctx).await;

            let error = response.get("error");
            assert!(error.is_some(), "should return method_not_found without completions feature registered");
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `cargo test -p mcp-server --features completions --lib server::dispatcher::tests::completion_tests`
Expected: FAIL — `with_completion_provider` not found

**Step 3: Write the implementation**

In `libs/rust-mcp/src/server/mod.rs`:

Add import after the sampling import (after line 28):

```rust
#[cfg(feature = "completions")]
use crate::completions::CompletionRegistry;
```

Add field to `McpServerBuilder` struct (after the sampling_handler field, line 48):

```rust
    #[cfg(feature = "completions")]
    completion_registry: CompletionRegistry,
```

Add initialization in `McpServerBuilder::new()` (after the sampling_handler init, line 70):

```rust
            #[cfg(feature = "completions")]
            completion_registry: CompletionRegistry::new(),
```

Add builder method (after `with_sampling_handler`, after line 132):

```rust
    #[cfg(feature = "completions")]
    pub fn with_completion_provider<P: crate::completions::CompletionProvider + 'static>(
        mut self,
        provider: P,
    ) -> Self {
        self.completion_registry.register(provider);
        self.enable_v1 = true;
        self
    }
```

In the `build()` method, add has_completions check (after has_sampling, around line 159):

```rust
        #[cfg(feature = "completions")]
        let has_completions = self.completion_registry.has_provider();

        #[cfg(not(feature = "completions"))]
        let has_completions = false;
```

Add capability setting in the V1 capabilities block (after has_sampling check, around line 203):

```rust
            if has_completions {
                caps = caps.with_completions();
            }
```

Add field to the `McpServer` struct construction (after sampling_handler, around line 229):

```rust
            #[cfg(feature = "completions")]
            completion_registry: self.completion_registry,
```

In `libs/rust-mcp/src/server/dispatcher.rs`:

Add import at top (after the sampling import, around line 20):

```rust
#[cfg(feature = "completions")]
use crate::completions::CompletionRegistry;
```

Add field to `McpServer` struct (after sampling_handler, around line 47):

```rust
    #[cfg(feature = "completions")]
    pub(crate) completion_registry: CompletionRegistry,
```

Add dispatch arm in `dispatch()` method (after the prompts/get arm, before the notifications, around line 110):

```rust
            #[cfg(feature = "completions")]
            "completion/complete" => self.handle_completion_complete(req.params, ctx).await,
```

Add handler method (after `handle_prompts_get`, before the closing `}` of `impl McpServer`):

```rust
    #[cfg(feature = "completions")]
    async fn handle_completion_complete(
        &self,
        params: Option<Value>,
        ctx: &Context,
    ) -> Result<Value, JsonRpcError> {
        let params = params.ok_or_else(|| JsonRpcError::invalid_params("Missing params"))?;

        let complete_params: crate::protocol::completions::CompletionCompleteParams =
            serde_json::from_value(params)
                .map_err(|e| JsonRpcError::invalid_params(format!("Invalid params: {}", e)))?;

        let result = self
            .completion_registry
            .complete(complete_params, ctx)
            .await
            .map_err(|e| JsonRpcError::internal_error(e.to_string()))?;

        serde_json::to_value(result).map_err(|e| JsonRpcError::internal_error(e.to_string()))
    }
```

**Step 4: Run tests to verify they pass**

Run: `cargo test -p mcp-server --features completions --lib server::dispatcher::tests::completion_tests`
Expected: Both tests PASS

**Step 5: Commit**

```
feat(rust-mcp): wire completion/complete into builder and dispatcher
```

---

### Task 4: Command framework types

**Files:**
- Create: `libs/rust-mcp/src/command/mod.rs`
- Create: `libs/rust-mcp/src/command/types.rs`
- Modify: `libs/rust-mcp/Cargo.toml:17` (add `command` feature)
- Modify: `libs/rust-mcp/src/lib.rs:76` (add module declaration)

**Step 1: Write the failing test**

Add to `libs/rust-mcp/src/command/types.rs`:

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn param_type_json_schema() {
        assert_eq!(ParamType::String.json_schema_type(), "string");
        assert_eq!(ParamType::Int.json_schema_type(), "integer");
        assert_eq!(ParamType::Bool.json_schema_type(), "boolean");
        assert_eq!(ParamType::Float.json_schema_type(), "number");
        assert_eq!(ParamType::Array.json_schema_type(), "array");
    }

    #[test]
    fn app_add_and_list_commands() {
        let mut app = App::new("test", "A test app");
        app.add_command(Command {
            name: "status".to_string(),
            description: Description {
                short: "Show status".to_string(),
                long: String::new(),
            },
            params: vec![
                Param {
                    name: "verbose".to_string(),
                    short: Some('v'),
                    param_type: ParamType::Bool,
                    description: "Verbose output".to_string(),
                    required: false,
                    default: None,
                },
            ],
            hidden: false,
            aliases: vec![],
        });
        app.add_command(Command {
            name: "hidden".to_string(),
            description: Description::short("Hidden cmd"),
            params: vec![],
            hidden: true,
            aliases: vec![],
        });

        assert_eq!(app.visible_commands().len(), 1);
        assert_eq!(app.visible_commands()[0].name, "status");
    }

    #[test]
    fn app_sorted_visible_commands() {
        let mut app = App::new("test", "A test app");
        app.add_command(Command {
            name: "zebra".to_string(),
            description: Description::short("Z cmd"),
            params: vec![],
            hidden: false,
            aliases: vec![],
        });
        app.add_command(Command {
            name: "alpha".to_string(),
            description: Description::short("A cmd"),
            params: vec![],
            hidden: false,
            aliases: vec![],
        });

        let visible = app.visible_commands();
        assert_eq!(visible[0].name, "alpha");
        assert_eq!(visible[1].name, "zebra");
    }
}
```

**Step 2: Run test to verify it fails**

Run: `cargo test -p mcp-server --features command --lib command::types::tests`
Expected: FAIL — module `command` not found

**Step 3: Write the implementation**

Add `command` feature to `libs/rust-mcp/Cargo.toml` (after the `completions` line):

```toml
command = []
```

Create `libs/rust-mcp/src/command/types.rs`:

```rust
use serde_json::Value;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ParamType {
    String,
    Int,
    Bool,
    Float,
    Array,
}

impl ParamType {
    pub fn json_schema_type(&self) -> &'static str {
        match self {
            ParamType::String => "string",
            ParamType::Int => "integer",
            ParamType::Bool => "boolean",
            ParamType::Float => "number",
            ParamType::Array => "array",
        }
    }
}

#[derive(Debug, Clone)]
pub struct Description {
    pub short: String,
    pub long: String,
}

impl Description {
    pub fn short(s: impl Into<String>) -> Self {
        Description {
            short: s.into(),
            long: String::new(),
        }
    }
}

#[derive(Debug, Clone)]
pub struct Param {
    pub name: String,
    pub short: Option<char>,
    pub param_type: ParamType,
    pub description: String,
    pub required: bool,
    pub default: Option<Value>,
}

#[derive(Debug, Clone)]
pub struct Command {
    pub name: String,
    pub description: Description,
    pub params: Vec<Param>,
    pub hidden: bool,
    pub aliases: Vec<String>,
}

pub struct App {
    pub name: String,
    pub description: Description,
    pub version: String,
    commands: Vec<Command>,
}

impl App {
    pub fn new(name: impl Into<String>, short_desc: impl Into<String>) -> Self {
        App {
            name: name.into(),
            description: Description::short(short_desc),
            version: String::new(),
            commands: Vec::new(),
        }
    }

    pub fn version(mut self, version: impl Into<String>) -> Self {
        self.version = version.into();
        self
    }

    pub fn add_command(&mut self, cmd: Command) {
        self.commands.push(cmd);
    }

    pub fn visible_commands(&self) -> Vec<&Command> {
        let mut cmds: Vec<&Command> = self
            .commands
            .iter()
            .filter(|c| !c.hidden)
            .collect();
        cmds.sort_by(|a, b| a.name.cmp(&b.name));
        cmds
    }
}
```

Create `libs/rust-mcp/src/command/mod.rs`:

```rust
pub mod types;

pub use types::{App, Command, Description, Param, ParamType};
```

Add to `libs/rust-mcp/src/lib.rs` after the `completions` module:

```rust
#[cfg(feature = "command")]
pub mod command;
```

Add re-exports:

```rust
#[cfg(feature = "command")]
pub use command::{App, Command as CliCommand, Description, Param, ParamType};
```

**Step 4: Run tests to verify they pass**

Run: `cargo test -p mcp-server --features command --lib command::types::tests`
Expected: All 3 tests PASS

**Step 5: Commit**

```
feat(rust-mcp): add command framework types (App, Command, Param)
```

---

### Task 5: Bash completion generation

**Files:**
- Create: `libs/rust-mcp/src/command/completions.rs`
- Modify: `libs/rust-mcp/src/command/mod.rs:1` (add module)

**Step 1: Write the failing test**

Add to `libs/rust-mcp/src/command/completions.rs`:

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use crate::command::types::*;

    fn test_app() -> App {
        let mut app = App::new("grit", "Git operations");
        app.add_command(Command {
            name: "status".to_string(),
            description: Description::short("Show status"),
            params: vec![
                Param {
                    name: "repo_path".to_string(),
                    short: None,
                    param_type: ParamType::String,
                    description: "Path to repo".to_string(),
                    required: true,
                    default: None,
                },
            ],
            hidden: false,
            aliases: vec![],
        });
        app.add_command(Command {
            name: "diff".to_string(),
            description: Description::short("Show changes"),
            params: vec![],
            hidden: false,
            aliases: vec![],
        });
        app.add_command(Command {
            name: "hidden".to_string(),
            description: Description::short("Hidden cmd"),
            params: vec![],
            hidden: true,
            aliases: vec![],
        });
        app
    }

    #[test]
    fn bash_completion_contains_commands() {
        let app = test_app();
        let dir = tempfile::tempdir().unwrap();
        app.generate_completions(dir.path().to_str().unwrap()).unwrap();

        let path = dir.path()
            .join("share/bash-completion/completions/grit");
        let content = std::fs::read_to_string(&path).unwrap();

        assert!(content.contains("status"), "missing status command");
        assert!(content.contains("diff"), "missing diff command");
        assert!(!content.contains("hidden"), "should not contain hidden commands");
        assert!(content.contains("--repo_path"), "missing repo_path flag");
    }

    #[test]
    fn bash_completion_short_flags() {
        let mut app = App::new("grit", "Git operations");
        app.add_command(Command {
            name: "status".to_string(),
            description: Description::short("Show status"),
            params: vec![
                Param {
                    name: "verbose".to_string(),
                    short: Some('v'),
                    param_type: ParamType::Bool,
                    description: "Verbose output".to_string(),
                    required: false,
                    default: None,
                },
            ],
            hidden: false,
            aliases: vec![],
        });

        let dir = tempfile::tempdir().unwrap();
        app.generate_completions(dir.path().to_str().unwrap()).unwrap();

        let path = dir.path()
            .join("share/bash-completion/completions/grit");
        let content = std::fs::read_to_string(&path).unwrap();

        assert!(content.contains("-v"), "missing short flag -v");
        assert!(content.contains("--verbose"), "missing long flag --verbose");
    }
}
```

**Step 2: Run test to verify it fails**

Run: `cargo test -p mcp-server --features command --lib command::completions::tests::bash`
Expected: FAIL — module `completions` not found in `command`

**Step 3: Write the implementation**

Add `tempfile` to dev-dependencies in `libs/rust-mcp/Cargo.toml`:

```toml
[dev-dependencies]
tokio = { version = "1", features = ["full", "test-util"] }
tempfile = "3"
```

Create `libs/rust-mcp/src/command/completions.rs` (start with bash only; zsh and fish added in later tasks):

```rust
use super::types::App;
use std::fmt::Write;
use std::fs;
use std::io;
use std::path::Path;

impl App {
    pub fn generate_completions(&self, dir: &str) -> io::Result<()> {
        self.generate_bash_completion(dir)?;
        self.generate_zsh_completion(dir)?;
        self.generate_fish_completion(dir)
    }

    fn generate_bash_completion(&self, dir: &str) -> io::Result<()> {
        let bash_dir = Path::new(dir).join("share/bash-completion/completions");
        fs::create_dir_all(&bash_dir)?;

        let cmds = self.visible_commands();

        let mut b = String::new();
        writeln!(b, "# bash completion for {}", self.name).unwrap();
        writeln!(b).unwrap();
        writeln!(b, "_{}() {{", self.name).unwrap();
        writeln!(b, "    local cur prev commands").unwrap();
        writeln!(b, "    COMPREPLY=()").unwrap();
        writeln!(b, "    cur=\"${{COMP_WORDS[COMP_CWORD]}}\"").unwrap();
        writeln!(b, "    prev=\"${{COMP_WORDS[COMP_CWORD-1]}}\"").unwrap();
        writeln!(b).unwrap();

        let names: Vec<&str> = cmds.iter().map(|c| c.name.as_str()).collect();
        writeln!(b, "    commands=\"{}\"", names.join(" ")).unwrap();
        writeln!(b).unwrap();

        writeln!(b, "    if [[ ${{COMP_CWORD}} -eq 1 ]]; then").unwrap();
        writeln!(b, "        COMPREPLY=( $(compgen -W \"${{commands}}\" -- \"${{cur}}\") )").unwrap();
        writeln!(b, "        return 0").unwrap();
        writeln!(b, "    fi").unwrap();
        writeln!(b).unwrap();

        writeln!(b, "    local subcmd=\"${{COMP_WORDS[1]}}\"").unwrap();
        writeln!(b, "    case \"${{subcmd}}\" in").unwrap();
        for cmd in &cmds {
            let mut flags = Vec::new();
            for p in &cmd.params {
                flags.push(format!("--{}", p.name));
                if let Some(short) = p.short {
                    flags.push(format!("-{}", short));
                }
            }
            if !flags.is_empty() {
                writeln!(b, "        {})", cmd.name).unwrap();
                writeln!(b, "            COMPREPLY=( $(compgen -W \"{}\" -- \"${{cur}}\") )", flags.join(" ")).unwrap();
                writeln!(b, "            ;;").unwrap();
            }
        }
        writeln!(b, "    esac").unwrap();
        writeln!(b, "}}").unwrap();
        writeln!(b).unwrap();
        writeln!(b, "complete -F _{} {}", self.name, self.name).unwrap();

        fs::write(bash_dir.join(&self.name), b)
    }

    fn generate_zsh_completion(&self, _dir: &str) -> io::Result<()> {
        Ok(()) // placeholder — implemented in Task 6
    }

    fn generate_fish_completion(&self, _dir: &str) -> io::Result<()> {
        Ok(()) // placeholder — implemented in Task 7
    }
}
```

Add to `libs/rust-mcp/src/command/mod.rs`:

```rust
pub mod completions;
```

**Step 4: Run tests to verify they pass**

Run: `cargo test -p mcp-server --features command --lib command::completions::tests::bash`
Expected: Both bash tests PASS

**Step 5: Commit**

```
feat(rust-mcp): add bash shell completion generation
```

---

### Task 6: Zsh completion generation

**Files:**
- Modify: `libs/rust-mcp/src/command/completions.rs` (replace zsh placeholder)

**Step 1: Write the failing test**

Add to the `tests` module in `libs/rust-mcp/src/command/completions.rs`:

```rust
    #[test]
    fn zsh_completion_structure() {
        let app = test_app();
        let dir = tempfile::tempdir().unwrap();
        app.generate_completions(dir.path().to_str().unwrap()).unwrap();

        let path = dir.path()
            .join("share/zsh/site-functions/_grit");
        let content = std::fs::read_to_string(&path).unwrap();

        assert!(content.contains("#compdef grit"), "missing #compdef header");
        assert!(content.contains("status"), "missing status command");
        assert!(content.contains("Show status"), "missing description");
        assert!(!content.contains("hidden"), "should not contain hidden commands");
    }
```

**Step 2: Run test to verify it fails**

Run: `cargo test -p mcp-server --features command --lib command::completions::tests::zsh`
Expected: FAIL — file not found (zsh placeholder is a no-op)

**Step 3: Write the implementation**

Replace the `generate_zsh_completion` placeholder in `libs/rust-mcp/src/command/completions.rs`:

```rust
    fn generate_zsh_completion(&self, dir: &str) -> io::Result<()> {
        let zsh_dir = Path::new(dir).join("share/zsh/site-functions");
        fs::create_dir_all(&zsh_dir)?;

        let cmds = self.visible_commands();

        let mut b = String::new();
        writeln!(b, "#compdef {}", self.name).unwrap();
        writeln!(b).unwrap();
        writeln!(b, "_{}() {{", self.name).unwrap();
        writeln!(b, "    local -a commands").unwrap();
        writeln!(b, "    commands=(").unwrap();
        for cmd in &cmds {
            let desc = cmd.description.short.replace('\'', "'\\''");
            writeln!(b, "        '{}:{}'", cmd.name, desc).unwrap();
        }
        writeln!(b, "    )").unwrap();
        writeln!(b).unwrap();
        writeln!(b, "    _describe 'command' commands").unwrap();
        writeln!(b, "}}").unwrap();
        writeln!(b).unwrap();
        writeln!(b, "_{}", self.name).unwrap();

        fs::write(zsh_dir.join(format!("_{}", self.name)), b)
    }
```

**Step 4: Run tests to verify they pass**

Run: `cargo test -p mcp-server --features command --lib command::completions::tests::zsh`
Expected: PASS

**Step 5: Commit**

```
feat(rust-mcp): add zsh shell completion generation
```

---

### Task 7: Fish completion generation

**Files:**
- Modify: `libs/rust-mcp/src/command/completions.rs` (replace fish placeholder)

**Step 1: Write the failing test**

Add to the `tests` module in `libs/rust-mcp/src/command/completions.rs`:

```rust
    #[test]
    fn fish_completion_structure() {
        let app = test_app();
        let dir = tempfile::tempdir().unwrap();
        app.generate_completions(dir.path().to_str().unwrap()).unwrap();

        let path = dir.path()
            .join("share/fish/vendor_completions.d/grit.fish");
        let content = std::fs::read_to_string(&path).unwrap();

        assert!(content.contains("complete -c grit"), "missing complete -c header");
        assert!(content.contains("status"), "missing status command");
        assert!(!content.contains("hidden"), "should not contain hidden commands");
    }

    #[test]
    fn fish_completion_short_flags() {
        let mut app = App::new("grit", "Git operations");
        app.add_command(Command {
            name: "status".to_string(),
            description: Description::short("Show status"),
            params: vec![
                Param {
                    name: "verbose".to_string(),
                    short: Some('v'),
                    param_type: ParamType::Bool,
                    description: "Verbose output".to_string(),
                    required: false,
                    default: None,
                },
            ],
            hidden: false,
            aliases: vec![],
        });

        let dir = tempfile::tempdir().unwrap();
        app.generate_completions(dir.path().to_str().unwrap()).unwrap();

        let path = dir.path()
            .join("share/fish/vendor_completions.d/grit.fish");
        let content = std::fs::read_to_string(&path).unwrap();

        assert!(content.contains("-s v"), "missing short flag -s v");
    }
```

**Step 2: Run test to verify it fails**

Run: `cargo test -p mcp-server --features command --lib command::completions::tests::fish`
Expected: FAIL — file not found (fish placeholder is a no-op)

**Step 3: Write the implementation**

Replace the `generate_fish_completion` placeholder in `libs/rust-mcp/src/command/completions.rs`:

```rust
    fn generate_fish_completion(&self, dir: &str) -> io::Result<()> {
        let fish_dir = Path::new(dir).join("share/fish/vendor_completions.d");
        fs::create_dir_all(&fish_dir)?;

        let cmds = self.visible_commands();

        let mut b = String::new();
        writeln!(b, "# fish completion for {}", self.name).unwrap();
        writeln!(b).unwrap();
        writeln!(b, "complete -c {} -f", self.name).unwrap();
        writeln!(b).unwrap();

        for cmd in &cmds {
            let desc = cmd.description.short.replace('\'', "\\'");
            writeln!(
                b,
                "complete -c {} -n '__fish_use_subcommand' -a {} -d '{}'",
                self.name, cmd.name, desc
            ).unwrap();
        }

        for cmd in &cmds {
            for p in &cmd.params {
                let desc = p.description.replace('\'', "\\'");
                let short_opt = match p.short {
                    Some(c) => format!(" -s {}", c),
                    None => String::new(),
                };
                writeln!(
                    b,
                    "complete -c {} -n '__fish_seen_subcommand_from {}' -l {}{} -d '{}'",
                    self.name, cmd.name, p.name, short_opt, desc
                ).unwrap();
            }
        }

        fs::write(fish_dir.join(format!("{}.fish", self.name)), b)
    }
```

**Step 4: Run tests to verify they pass**

Run: `cargo test -p mcp-server --features command --lib command::completions::tests::fish`
Expected: Both fish tests PASS

**Step 5: Run all tests**

Run: `cargo test -p mcp-server --features "command,completions"`
Expected: All tests PASS

**Step 6: Commit**

```
feat(rust-mcp): add fish shell completion generation
```

---

### Task 8: Full build verification

**Files:** None (verification only)

**Step 1: Run all rust-mcp tests with all features**

Run: `cargo test -p mcp-server --all-features`
Expected: All tests PASS

**Step 2: Run nix flake check**

Run: `nix flake check` (from the worktree root)
Expected: PASS (or at least no rust-mcp-related failures)

**Step 3: Commit (if any formatting fixes needed)**

```
chore(rust-mcp): format and fix any lints
```
