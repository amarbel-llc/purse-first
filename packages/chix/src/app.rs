use mcp_server::command::{App, Command, Description, PostToolUseHook};
use mcp_server::hooks::ToolMapping;

pub fn make_app() -> App {
    let mut app = App::new("chix", "Nix MCP server and skills for Claude Code");
    app = app
        .version("0.1.0")
        .plugin_description("Nix MCP server providing tools for building, evaluating, and managing Nix flakes, packages, and store paths. Includes FlakeHub and Cachix integration, Nix language diagnostics via nil LSP, and background task management.")
        .plugin_author("friedenberg");

    app.add_command(Command {
        name: "build".into(),
        description: Description::short("Build a nix flake package"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix build".into()],
            extensions: vec![],
            use_when: "building nix packages".into(),
        }],
    });

    app.add_command(Command {
        name: "flake_show".into(),
        description: Description::short("List outputs of a nix flake"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix flake show".into()],
            extensions: vec![],
            use_when: "listing flake outputs".into(),
        }],
    });

    app.add_command(Command {
        name: "flake_check".into(),
        description: Description::short("Run flake checks and tests"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix flake check".into()],
            extensions: vec![],
            use_when: "running flake checks".into(),
        }],
    });

    app.add_command(Command {
        name: "flake_metadata".into(),
        description: Description::short("Get metadata for a flake"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix flake metadata".into()],
            extensions: vec![],
            use_when: "getting flake metadata".into(),
        }],
    });

    app.add_command(Command {
        name: "flake_update".into(),
        description: Description::short("Update flake.lock file"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix flake update".into()],
            extensions: vec![],
            use_when: "updating flake inputs".into(),
        }],
    });

    app.add_command(Command {
        name: "flake_lock".into(),
        description: Description::short("Lock flake inputs without building"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix flake lock".into()],
            extensions: vec![],
            use_when: "locking flake inputs".into(),
        }],
    });

    app.add_command(Command {
        name: "log".into(),
        description: Description::short("Get build logs for a derivation"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix log".into()],
            extensions: vec![],
            use_when: "getting build logs".into(),
        }],
    });

    app.add_command(Command {
        name: "run".into(),
        description: Description::short("Run a flake app"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix run".into()],
            extensions: vec![],
            use_when: "running flake apps".into(),
        }],
    });

    app.add_command(Command {
        name: "develop_run".into(),
        description: Description::short("Run a command inside a flake's devShell"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix develop".into()],
            extensions: vec![],
            use_when: "running commands in a devShell".into(),
        }],
    });

    app.add_command(Command {
        name: "store_path_info".into(),
        description: Description::short("Get information about a store path"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix path-info".into()],
            extensions: vec![],
            use_when: "querying store path info".into(),
        }],
    });

    app.add_command(Command {
        name: "store_gc".into(),
        description: Description::short("Run garbage collection on the Nix store"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix store gc".into()],
            extensions: vec![],
            use_when: "running nix garbage collection".into(),
        }],
    });

    app.add_command(Command {
        name: "derivation_show".into(),
        description: Description::short("Show the contents of a derivation"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix derivation show".into()],
            extensions: vec![],
            use_when: "showing derivation contents".into(),
        }],
    });

    app.add_command(Command {
        name: "eval".into(),
        description: Description::short("Evaluate a nix expression"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["nix eval".into()],
            extensions: vec![],
            use_when: "evaluating nix expressions".into(),
        }],
    });

    app.add_command(Command {
        name: "fh_search".into(),
        description: Description::short("Search FlakeHub for flakes"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["fh search".into()],
            extensions: vec![],
            use_when: "searching FlakeHub".into(),
        }],
    });

    app.add_command(Command {
        name: "fh_add".into(),
        description: Description::short("Add a flake input from FlakeHub"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["fh add".into()],
            extensions: vec![],
            use_when: "adding FlakeHub inputs".into(),
        }],
    });

    app.add_command(Command {
        name: "fh_list_flakes".into(),
        description: Description::short("List public flakes on FlakeHub"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["fh list".into()],
            extensions: vec![],
            use_when: "listing FlakeHub flakes".into(),
        }],
    });

    app.add_command(Command {
        name: "fh_resolve".into(),
        description: Description::short("Resolve a FlakeHub flake reference"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["fh resolve".into()],
            extensions: vec![],
            use_when: "resolving FlakeHub references".into(),
        }],
    });

    app.add_command(Command {
        name: "fh_status".into(),
        description: Description::short("Check FlakeHub login and cache status"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["fh status".into()],
            extensions: vec![],
            use_when: "checking FlakeHub status".into(),
        }],
    });

    app.add_command(Command {
        name: "fh_fetch".into(),
        description: Description::short("Fetch a flake output from FlakeHub cache"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["fh fetch".into()],
            extensions: vec![],
            use_when: "fetching from FlakeHub".into(),
        }],
    });

    app.add_command(Command {
        name: "fh_login".into(),
        description: Description::short("Initiate FlakeHub OAuth login flow"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["fh login".into()],
            extensions: vec![],
            use_when: "logging into FlakeHub".into(),
        }],
    });

    app.add_command(Command {
        name: "cachix_push".into(),
        description: Description::short("Push store paths to a Cachix binary cache"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["cachix push".into()],
            extensions: vec![],
            use_when: "pushing to Cachix".into(),
        }],
    });

    app.add_command(Command {
        name: "cachix_use".into(),
        description: Description::short("Configure Nix to use a Cachix binary cache"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["cachix use".into()],
            extensions: vec![],
            use_when: "configuring Cachix substituter".into(),
        }],
    });

    app.add_command(Command {
        name: "cachix_status".into(),
        description: Description::short("Check Cachix authentication status"),
        params: vec![],
        hidden: false,
        aliases: vec![],
        maps_tools: vec![ToolMapping {
            replaces: "Bash".into(),
            command_prefixes: vec!["cachix status".into()],
            extensions: vec![],
            use_when: "checking Cachix status".into(),
        }],
    });

    app.add_post_tool_use_hook(PostToolUseHook {
        matcher: "Edit|Write".into(),
        command: "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix".into(),
        timeout: 30,
    });

    app
}
