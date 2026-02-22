use super::{
    CompletionItem, Diagnostic, DiagnosticSeverity, HoverResult, Location, LspClient, LspError,
    Position, Range,
};
use async_trait::async_trait;
use serde_json::{json, Value};
use std::collections::HashMap;
use std::process::Stdio;
use std::sync::atomic::{AtomicI64, Ordering};
use std::sync::Arc;
use tokio::io::{AsyncBufReadExt, AsyncReadExt, AsyncWriteExt, BufReader};
use tokio::process::{Child, ChildStdin, ChildStdout, Command};
use tokio::sync::Mutex;
use tokio::time::{timeout, Duration};

const DEFAULT_TIMEOUT_SECS: u64 = 30;

pub struct SpawnedLspClient {
    command: String,
    process: Child,
    stdin: ChildStdin,
    stdout: Arc<Mutex<BufReader<ChildStdout>>>,
    request_id: AtomicI64,
    pending_diagnostics: Arc<Mutex<HashMap<String, Vec<Diagnostic>>>>,
}

impl SpawnedLspClient {
    pub async fn new(command: &str) -> Result<Self, LspError> {
        let mut process = Command::new(command)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .kill_on_drop(true)
            .spawn()
            .map_err(|e| LspError::SpawnFailed(format!("{}: {}", command, e)))?;

        let stdin = process
            .stdin
            .take()
            .ok_or_else(|| LspError::SpawnFailed("Failed to capture stdin".to_string()))?;
        let stdout = process
            .stdout
            .take()
            .ok_or_else(|| LspError::SpawnFailed("Failed to capture stdout".to_string()))?;

        Ok(Self {
            command: command.to_string(),
            process,
            stdin,
            stdout: Arc::new(Mutex::new(BufReader::new(stdout))),
            request_id: AtomicI64::new(1),
            pending_diagnostics: Arc::new(Mutex::new(HashMap::new())),
        })
    }

    fn next_id(&self) -> i64 {
        self.request_id.fetch_add(1, Ordering::SeqCst)
    }

    async fn send_request(&mut self, method: &str, params: Value) -> Result<Value, LspError> {
        let id = self.next_id();
        let request = json!({
            "jsonrpc": "2.0",
            "id": id,
            "method": method,
            "params": params
        });

        self.send_message(&request).await?;
        self.receive_response(id).await
    }

    async fn send_notification(&mut self, method: &str, params: Value) -> Result<(), LspError> {
        let notification = json!({
            "jsonrpc": "2.0",
            "method": method,
            "params": params
        });

        self.send_message(&notification).await
    }

    async fn send_message(&mut self, message: &Value) -> Result<(), LspError> {
        let content = serde_json::to_string(message)
            .map_err(|e| LspError::Protocol(format!("Failed to serialize message: {}", e)))?;

        let header = format!("Content-Length: {}\r\n\r\n", content.len());

        self.stdin
            .write_all(header.as_bytes())
            .await
            .map_err(|e| LspError::Communication(format!("Failed to write header: {}", e)))?;

        self.stdin
            .write_all(content.as_bytes())
            .await
            .map_err(|e| LspError::Communication(format!("Failed to write content: {}", e)))?;

        self.stdin
            .flush()
            .await
            .map_err(|e| LspError::Communication(format!("Failed to flush: {}", e)))?;

        Ok(())
    }

    async fn receive_response(&mut self, expected_id: i64) -> Result<Value, LspError> {
        let result = timeout(Duration::from_secs(DEFAULT_TIMEOUT_SECS), async {
            loop {
                let message = self.read_message().await?;

                if let Some(id) = message.get("id") {
                    if id.as_i64() == Some(expected_id) {
                        if let Some(error) = message.get("error") {
                            let msg = error
                                .get("message")
                                .and_then(|m| m.as_str())
                                .unwrap_or("Unknown error");
                            return Err(LspError::Protocol(msg.to_string()));
                        }
                        return Ok(message.get("result").cloned().unwrap_or(Value::Null));
                    }
                }

                // Handle notifications (like publishDiagnostics)
                if let Some(method) = message.get("method").and_then(|m| m.as_str()) {
                    self.handle_notification(method, &message).await?;
                }
            }
        })
        .await;

        match result {
            Ok(r) => r,
            Err(_) => Err(LspError::Timeout(DEFAULT_TIMEOUT_SECS)),
        }
    }

    async fn read_message(&mut self) -> Result<Value, LspError> {
        let mut stdout = self.stdout.lock().await;

        // Read headers
        let mut content_length: Option<usize> = None;
        loop {
            let mut line = String::new();
            stdout
                .read_line(&mut line)
                .await
                .map_err(|e| LspError::Communication(format!("Failed to read header: {}", e)))?;

            let line = line.trim();
            if line.is_empty() {
                break;
            }

            if let Some(len_str) = line.strip_prefix("Content-Length: ") {
                content_length = Some(len_str.parse().map_err(|e| {
                    LspError::Protocol(format!("Invalid Content-Length: {}", e))
                })?);
            }
        }

        let content_length = content_length
            .ok_or_else(|| LspError::Protocol("Missing Content-Length header".to_string()))?;

        // Read content
        let mut content = vec![0u8; content_length];
        stdout
            .read_exact(&mut content)
            .await
            .map_err(|e| LspError::Communication(format!("Failed to read content: {}", e)))?;

        serde_json::from_slice(&content)
            .map_err(|e| LspError::Protocol(format!("Invalid JSON: {}", e)))
    }

    async fn handle_notification(&self, method: &str, message: &Value) -> Result<(), LspError> {
        if method == "textDocument/publishDiagnostics" {
            if let Some(params) = message.get("params") {
                let uri = params
                    .get("uri")
                    .and_then(|u| u.as_str())
                    .unwrap_or("")
                    .to_string();

                let diagnostics: Vec<Diagnostic> = params
                    .get("diagnostics")
                    .and_then(|d| d.as_array())
                    .map(|arr| {
                        arr.iter()
                            .filter_map(|d| parse_diagnostic(d))
                            .collect()
                    })
                    .unwrap_or_default();

                let mut pending = self.pending_diagnostics.lock().await;
                pending.insert(uri, diagnostics);
            }
        }
        Ok(())
    }
}

fn parse_diagnostic(value: &Value) -> Option<Diagnostic> {
    let range = value.get("range")?;
    let start = range.get("start")?;
    let end = range.get("end")?;

    Some(Diagnostic {
        range: Range {
            start: Position {
                line: start.get("line")?.as_u64()? as u32,
                character: start.get("character")?.as_u64()? as u32,
            },
            end: Position {
                line: end.get("line")?.as_u64()? as u32,
                character: end.get("character")?.as_u64()? as u32,
            },
        },
        severity: value
            .get("severity")
            .and_then(|s| s.as_u64())
            .map(|s| DiagnosticSeverity(s as u32)),
        message: value.get("message")?.as_str()?.to_string(),
        source: value
            .get("source")
            .and_then(|s| s.as_str())
            .map(|s| s.to_string()),
    })
}

fn parse_completion_item(value: &Value) -> Option<CompletionItem> {
    Some(CompletionItem {
        label: value.get("label")?.as_str()?.to_string(),
        kind: value.get("kind").and_then(|k| k.as_u64()).map(|k| k as u32),
        detail: value
            .get("detail")
            .and_then(|d| d.as_str())
            .map(|s| s.to_string()),
        documentation: value.get("documentation").and_then(|d| {
            if let Some(s) = d.as_str() {
                Some(s.to_string())
            } else {
                d.get("value").and_then(|v| v.as_str()).map(|s| s.to_string())
            }
        }),
    })
}

fn parse_location(value: &Value) -> Option<Location> {
    let range = value.get("range")?;
    let start = range.get("start")?;
    let end = range.get("end")?;

    Some(Location {
        uri: value.get("uri")?.as_str()?.to_string(),
        range: Range {
            start: Position {
                line: start.get("line")?.as_u64()? as u32,
                character: start.get("character")?.as_u64()? as u32,
            },
            end: Position {
                line: end.get("line")?.as_u64()? as u32,
                character: end.get("character")?.as_u64()? as u32,
            },
        },
    })
}

#[async_trait]
impl LspClient for SpawnedLspClient {
    async fn initialize(&mut self, root_uri: Option<&str>) -> Result<(), LspError> {
        let params = json!({
            "processId": std::process::id(),
            "rootUri": root_uri,
            "capabilities": {
                "textDocument": {
                    "completion": {
                        "completionItem": {
                            "documentationFormat": ["plaintext", "markdown"]
                        }
                    },
                    "hover": {
                        "contentFormat": ["plaintext", "markdown"]
                    },
                    "publishDiagnostics": {}
                }
            }
        });

        self.send_request("initialize", params).await?;
        self.send_notification("initialized", json!({})).await?;

        Ok(())
    }

    async fn did_open(&mut self, uri: &str, text: &str) -> Result<(), LspError> {
        let params = json!({
            "textDocument": {
                "uri": uri,
                "languageId": "nix",
                "version": 1,
                "text": text
            }
        });

        self.send_notification("textDocument/didOpen", params).await
    }

    async fn diagnostics(&mut self, uri: &str) -> Result<Vec<Diagnostic>, LspError> {
        // Give the server a moment to send diagnostics after didOpen
        tokio::time::sleep(Duration::from_millis(100)).await;

        // Try to read any pending messages (diagnostics are sent as notifications)
        let result = timeout(Duration::from_millis(500), async {
            loop {
                match timeout(Duration::from_millis(100), self.read_message()).await {
                    Ok(Ok(message)) => {
                        if let Some(method) = message.get("method").and_then(|m| m.as_str()) {
                            self.handle_notification(method, &message).await?;
                        }
                    }
                    _ => break,
                }
            }
            Ok::<(), LspError>(())
        })
        .await;

        // Ignore timeout - just means no more messages
        let _ = result;

        let pending = self.pending_diagnostics.lock().await;
        Ok(pending.get(uri).cloned().unwrap_or_default())
    }

    async fn completion(
        &mut self,
        uri: &str,
        line: u32,
        character: u32,
    ) -> Result<Vec<CompletionItem>, LspError> {
        let params = json!({
            "textDocument": { "uri": uri },
            "position": { "line": line, "character": character }
        });

        let result = self
            .send_request("textDocument/completion", params)
            .await?;

        let items = if let Some(arr) = result.as_array() {
            arr.iter().filter_map(parse_completion_item).collect()
        } else if let Some(arr) = result.get("items").and_then(|i| i.as_array()) {
            arr.iter().filter_map(parse_completion_item).collect()
        } else {
            vec![]
        };

        Ok(items)
    }

    async fn hover(
        &mut self,
        uri: &str,
        line: u32,
        character: u32,
    ) -> Result<Option<HoverResult>, LspError> {
        let params = json!({
            "textDocument": { "uri": uri },
            "position": { "line": line, "character": character }
        });

        let result = self.send_request("textDocument/hover", params).await?;

        if result.is_null() {
            return Ok(None);
        }

        let contents = result.get("contents").map(|c| {
            if let Some(s) = c.as_str() {
                s.to_string()
            } else if let Some(arr) = c.as_array() {
                arr.iter()
                    .filter_map(|v| {
                        if let Some(s) = v.as_str() {
                            Some(s.to_string())
                        } else {
                            v.get("value").and_then(|v| v.as_str()).map(|s| s.to_string())
                        }
                    })
                    .collect::<Vec<_>>()
                    .join("\n")
            } else if let Some(value) = c.get("value").and_then(|v| v.as_str()) {
                value.to_string()
            } else {
                c.to_string()
            }
        });

        let range = result.get("range").and_then(|r| {
            let start = r.get("start")?;
            let end = r.get("end")?;
            Some(Range {
                start: Position {
                    line: start.get("line")?.as_u64()? as u32,
                    character: start.get("character")?.as_u64()? as u32,
                },
                end: Position {
                    line: end.get("line")?.as_u64()? as u32,
                    character: end.get("character")?.as_u64()? as u32,
                },
            })
        });

        Ok(contents.map(|contents| HoverResult { contents, range }))
    }

    async fn goto_definition(
        &mut self,
        uri: &str,
        line: u32,
        character: u32,
    ) -> Result<Vec<Location>, LspError> {
        let params = json!({
            "textDocument": { "uri": uri },
            "position": { "line": line, "character": character }
        });

        let result = self
            .send_request("textDocument/definition", params)
            .await?;

        let locations = if result.is_null() {
            vec![]
        } else if let Some(arr) = result.as_array() {
            arr.iter().filter_map(parse_location).collect()
        } else if let Some(loc) = parse_location(&result) {
            vec![loc]
        } else {
            vec![]
        };

        Ok(locations)
    }

    async fn shutdown(&mut self) -> Result<(), LspError> {
        // Send shutdown request
        let _ = self.send_request("shutdown", json!(null)).await;

        // Send exit notification
        let _ = self.send_notification("exit", json!(null)).await;

        // Wait for process to exit
        let _ = self.process.wait().await;

        Ok(())
    }
}

impl Drop for SpawnedLspClient {
    fn drop(&mut self) {
        // Process will be killed on drop due to kill_on_drop(true)
    }
}
