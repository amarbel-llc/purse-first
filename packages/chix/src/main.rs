mod background;
mod config;
mod lsp_client;
mod nix_runner;
mod output;
mod resources;
mod server;
mod tools;
mod validators;

use clap::{Parser, Subcommand};
use server::Server;
use std::process::Command;
use tokio::io::{stdin, stdout, AsyncBufReadExt, AsyncWriteExt, BufReader};

#[derive(Parser)]
#[command(name = "chix")]
#[command(about = "Nix MCP server and skills for Claude Code")]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}

#[derive(Subcommand)]
enum Commands {
    /// Install chix as MCP server in Claude Code
    InstallClaude,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Some(Commands::InstallClaude) => install_claude(),
        None => run_server().await,
    }
}

fn install_claude() -> anyhow::Result<()> {
    let exe_path = std::env::current_exe()?;

    // Remove existing chix MCP server (ignore errors if it doesn't exist)
    let _ = Command::new("claude")
        .args(["mcp", "remove", "chix"])
        .status();

    // Add chix MCP server
    let status = Command::new("claude")
        .args(["mcp", "add", "chix", "--", exe_path.to_str().unwrap()])
        .status()?;

    if status.success() {
        println!("Successfully installed chix as MCP server 'chix'");
        println!("To verify, run: claude mcp list");
        Ok(())
    } else {
        anyhow::bail!("Failed to install MCP server");
    }
}

async fn run_server() -> anyhow::Result<()> {
    let server = Server::new();
    let stdin = BufReader::new(stdin());
    let mut stdout = stdout();
    let mut lines = stdin.lines();

    while let Some(line) = lines.next_line().await? {
        if line.is_empty() {
            continue;
        }

        let response = server.handle_request(&line).await;
        let response_json = serde_json::to_string(&response)?;
        stdout.write_all(response_json.as_bytes()).await?;
        stdout.write_all(b"\n").await?;
        stdout.flush().await?;
    }

    Ok(())
}
