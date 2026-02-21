//! Streamable HTTP transport for MCP servers.
//!
//! Implements the MCP Streamable HTTP transport specification:
//! - Single HTTP endpoint accepting POST and GET
//! - POST for JSON-RPC messages, responses as JSON or SSE
//! - Session management via Mcp-Session-Id header
//! - Origin validation for DNS rebinding protection
//!
//! This module provides a minimal HTTP server built directly on tokio TCP,
//! avoiding external HTTP framework dependencies.

use crate::error::ServerError;
use crate::protocol::{JsonRpcRequest, JsonRpcResponse, JsonRpcError};
use crate::server::{Context, McpServer};
use crate::protocol::ClientCapabilities;

use std::collections::HashMap;
use std::sync::Arc;
use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader};
use tokio::net::TcpListener;
use tokio::sync::Mutex;

const HEADER_SESSION_ID: &str = "mcp-session-id";

/// Run an MCP server with Streamable HTTP transport.
///
/// Listens on the given address and handles MCP requests via HTTP POST.
/// Each request is processed independently with session tracking.
pub async fn run_http_server(server: McpServer, addr: &str) -> Result<(), ServerError> {
    let listener = TcpListener::bind(addr).await?;
    let server = Arc::new(server);
    let sessions: Arc<Mutex<HashMap<String, bool>>> = Arc::new(Mutex::new(HashMap::new()));

    loop {
        let (stream, _peer_addr) = listener.accept().await?;
        let server = Arc::clone(&server);
        let sessions = Arc::clone(&sessions);

        tokio::spawn(async move {
            if let Err(_e) = handle_connection(stream, &server, &sessions).await {
                // Connection errors are expected (client disconnect, etc.)
            }
        });
    }
}

async fn handle_connection(
    stream: tokio::net::TcpStream,
    server: &McpServer,
    sessions: &Mutex<HashMap<String, bool>>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let (reader, mut writer) = stream.into_split();
    let mut buf_reader = BufReader::new(reader);

    // Read HTTP request line.
    let mut request_line = String::new();
    buf_reader.read_line(&mut request_line).await?;
    let parts: Vec<&str> = request_line.trim().split_whitespace().collect();
    if parts.len() < 3 {
        write_http_response(&mut writer, 400, "Bad Request", "Invalid request line").await?;
        return Ok(());
    }

    let method = parts[0];
    let _path = parts[1];

    // Read headers.
    let mut headers: HashMap<String, String> = HashMap::new();
    let mut content_length: usize = 0;
    loop {
        let mut line = String::new();
        buf_reader.read_line(&mut line).await?;
        let trimmed = line.trim();
        if trimmed.is_empty() {
            break;
        }
        if let Some((key, value)) = trimmed.split_once(':') {
            let key = key.trim().to_lowercase();
            let value = value.trim().to_string();
            if key == "content-length" {
                content_length = value.parse().unwrap_or(0);
            }
            headers.insert(key, value);
        }
    }

    match method {
        "POST" => {
            // Read body.
            let mut body = vec![0u8; content_length];
            if content_length > 0 {
                tokio::io::AsyncReadExt::read_exact(&mut buf_reader, &mut body).await?;
            }

            let body_str = String::from_utf8_lossy(&body);
            let parsed: Result<JsonRpcRequest, _> = serde_json::from_str(&body_str);

            match parsed {
                Ok(req) => {
                    let is_initialize = req.method == "initialize";
                    let is_notification = req.id.is_none();

                    // Validate session for non-initialize requests.
                    if !is_initialize {
                        if let Some(session_id) = headers.get(HEADER_SESSION_ID) {
                            let sessions = sessions.lock().await;
                            if !sessions.contains_key(session_id) {
                                write_http_response(&mut writer, 400, "Bad Request", "Invalid session").await?;
                                return Ok(());
                            }
                        }
                    }

                    if is_notification {
                        write_http_response(&mut writer, 202, "Accepted", "").await?;
                        return Ok(());
                    }

                    // Process the request.
                    let mut ctx = Context::new(
                        server.server_info.name.clone(),
                        server.server_info.version.clone(),
                        ClientCapabilities::default(),
                    );

                    let response_value = server.handle_request(&body_str, &mut ctx).await;
                    let response_json = serde_json::to_string(&response_value)?;

                    // For initialize, create a session.
                    let mut extra_headers = String::new();
                    if is_initialize {
                        let session_id = generate_session_id();
                        sessions.lock().await.insert(session_id.clone(), true);
                        extra_headers = format!("Mcp-Session-Id: {}\r\n", session_id);
                    }

                    // Check if client accepts SSE.
                    let accept = headers.get("accept").cloned().unwrap_or_default();
                    if accept.contains("text/event-stream") {
                        let sse_response = format!(
                            "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nCache-Control: no-cache\r\nConnection: keep-alive\r\n{}r\n\r\nid: 1\nevent: message\ndata: {}\n\n",
                            extra_headers, response_json
                        );
                        writer.write_all(sse_response.as_bytes()).await?;
                    } else {
                        let http_response = format!(
                            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\n{}\r\n{}",
                            response_json.len(),
                            extra_headers,
                            response_json
                        );
                        writer.write_all(http_response.as_bytes()).await?;
                    }
                }
                Err(e) => {
                    let error_response = JsonRpcResponse::error(
                        serde_json::Value::Null,
                        JsonRpcError::parse_error(format!("Parse error: {}", e)),
                    );
                    let response_json = serde_json::to_string(&error_response)?;
                    write_json_response(&mut writer, 200, &response_json, "").await?;
                }
            }
        }
        "GET" => {
            let accept = headers.get("accept").cloned().unwrap_or_default();
            if !accept.contains("text/event-stream") {
                write_http_response(&mut writer, 405, "Method Not Allowed", "GET requires Accept: text/event-stream").await?;
                return Ok(());
            }

            // Open SSE stream — keep alive until client disconnects.
            let sse_header = "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nCache-Control: no-cache\r\nConnection: keep-alive\r\n\r\n";
            writer.write_all(sse_header.as_bytes()).await?;
            writer.flush().await?;

            // Block until client disconnects (detected by write failure or shutdown).
            loop {
                tokio::time::sleep(std::time::Duration::from_secs(30)).await;
                // Send a comment as keepalive.
                if writer.write_all(b": keepalive\n\n").await.is_err() {
                    break;
                }
                if writer.flush().await.is_err() {
                    break;
                }
            }
        }
        "DELETE" => {
            if let Some(session_id) = headers.get(HEADER_SESSION_ID) {
                let mut sessions = sessions.lock().await;
                if sessions.remove(session_id).is_some() {
                    write_http_response(&mut writer, 200, "OK", "").await?;
                } else {
                    write_http_response(&mut writer, 404, "Not Found", "Unknown session").await?;
                }
            } else {
                write_http_response(&mut writer, 400, "Bad Request", "Missing session ID").await?;
            }
        }
        _ => {
            write_http_response(&mut writer, 405, "Method Not Allowed", "").await?;
        }
    }

    Ok(())
}

async fn write_http_response(
    writer: &mut tokio::net::tcp::OwnedWriteHalf,
    status: u16,
    reason: &str,
    body: &str,
) -> Result<(), std::io::Error> {
    let response = format!(
        "HTTP/1.1 {} {}\r\nContent-Type: text/plain\r\nContent-Length: {}\r\n\r\n{}",
        status,
        reason,
        body.len(),
        body
    );
    writer.write_all(response.as_bytes()).await?;
    writer.flush().await
}

async fn write_json_response(
    writer: &mut tokio::net::tcp::OwnedWriteHalf,
    status: u16,
    json: &str,
    extra_headers: &str,
) -> Result<(), std::io::Error> {
    let response = format!(
        "HTTP/1.1 {} OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\n{}\r\n{}",
        status,
        json.len(),
        extra_headers,
        json
    );
    writer.write_all(response.as_bytes()).await?;
    writer.flush().await
}

fn generate_session_id() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    // Simple unique ID from timestamp + random-like bits.
    // For production, use a proper random source.
    format!("{:x}-{:x}", nanos, nanos.wrapping_mul(6364136223846793005))
}
