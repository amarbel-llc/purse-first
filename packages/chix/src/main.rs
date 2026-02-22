mod background;
mod config;
mod lsp_client;
mod nix_runner;
mod output;
mod resources;
mod tools;
mod validators;

use clap::{Parser, Subcommand};
use mcp_server::server::{McpServerBuilder, run_stdio_server};
use std::process::Command;

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
    let server = McpServerBuilder::new("chix", "0.1.0")
        // Tools
        .with_tool(tools::BuildTool)
        .with_tool(tools::FlakeShowTool)
        .with_tool(tools::FlakeCheckTool)
        .with_tool(tools::FlakeMetadataTool)
        .with_tool(tools::FlakeUpdateTool)
        .with_tool(tools::FlakeLockTool)
        .with_tool(tools::FlakeInitTool)
        .with_tool(tools::RunTool)
        .with_tool(tools::DevelopRunTool)
        .with_tool(tools::LogTool)
        .with_tool(tools::SearchTool)
        .with_tool(tools::StorePathInfoTool)
        .with_tool(tools::StoreGcTool)
        .with_tool(tools::StoreLsTool)
        .with_tool(tools::StoreCatTool)
        .with_tool(tools::DerivationShowTool)
        .with_tool(tools::HashPathTool)
        .with_tool(tools::HashFileTool)
        .with_tool(tools::CopyTool)
        .with_tool(tools::EvalTool)
        .with_tool(tools::FhSearchTool)
        .with_tool(tools::FhAddTool)
        .with_tool(tools::FhListFlakesTool)
        .with_tool(tools::FhListReleasesTool)
        .with_tool(tools::FhListVersionsTool)
        .with_tool(tools::FhResolveTool)
        .with_tool(tools::CachixPushTool)
        .with_tool(tools::CachixUseTool)
        .with_tool(tools::CachixStatusTool)
        .with_tool(tools::FhStatusTool)
        .with_tool(tools::FhFetchTool)
        .with_tool(tools::FhLoginTool)
        .with_tool(tools::TaskStatusTool)
        .with_tool(tools::NilDiagnosticsTool)
        .with_tool(tools::NilCompletionsTool)
        .with_tool(tools::NilHoverTool)
        .with_tool(tools::NilDefinitionTool)
        // Resources
        .with_resource(resources::BuildLogResource)
        .with_resource(resources::DerivationResource)
        .with_resource(resources::ClosureResource)
        .build();

    run_stdio_server(server).await?;
    Ok(())
}
