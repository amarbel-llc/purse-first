use crate::error::ServerError;
use crate::protocol::ClientCapabilities;
use crate::server::{Context, McpServer};
use tokio::io::{stdin, stdout, AsyncBufReadExt, AsyncWriteExt, BufReader};

/// Run an MCP server on stdin/stdout
pub async fn run_stdio_server(server: McpServer) -> Result<(), ServerError> {
    let stdin = BufReader::new(stdin());
    let mut stdout = stdout();
    let mut lines = stdin.lines();

    // Create context - will be updated after initialize
    let mut ctx = Context::new(
        server.server_info.name.clone(),
        server.server_info.version.clone(),
        ClientCapabilities::default(),
    );

    while let Some(line) = lines.next_line().await? {
        if line.is_empty() {
            continue;
        }

        let response = server.handle_request(&line, &mut ctx).await;
        let response_json = serde_json::to_string(&response)?;

        stdout.write_all(response_json.as_bytes()).await?;
        stdout.write_all(b"\n").await?;
        stdout.flush().await?;
    }

    Ok(())
}
