use mcp_server::hooks::{HookHandler, ToolMapping};

pub fn make_hook_handler() -> HookHandler {
    HookHandler::new("chix")
        .add_mapping(
            "build",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix build".into()],
                extensions: vec![],
                use_when: "building nix packages".into(),
            },
        )
        .add_mapping(
            "flake_show",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix flake show".into()],
                extensions: vec![],
                use_when: "listing flake outputs".into(),
            },
        )
        .add_mapping(
            "flake_check",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix flake check".into()],
                extensions: vec![],
                use_when: "running flake checks".into(),
            },
        )
        .add_mapping(
            "flake_metadata",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix flake metadata".into()],
                extensions: vec![],
                use_when: "getting flake metadata".into(),
            },
        )
        .add_mapping(
            "flake_update",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix flake update".into()],
                extensions: vec![],
                use_when: "updating flake inputs".into(),
            },
        )
        .add_mapping(
            "flake_lock",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix flake lock".into()],
                extensions: vec![],
                use_when: "locking flake inputs".into(),
            },
        )
        .add_mapping(
            "log",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix log".into()],
                extensions: vec![],
                use_when: "getting build logs".into(),
            },
        )
        .add_mapping(
            "run",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix run".into()],
                extensions: vec![],
                use_when: "running flake apps".into(),
            },
        )
        .add_mapping(
            "develop_run",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix develop".into()],
                extensions: vec![],
                use_when: "running commands in a devShell".into(),
            },
        )
        .add_mapping(
            "store_path_info",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix path-info".into()],
                extensions: vec![],
                use_when: "querying store path info".into(),
            },
        )
        .add_mapping(
            "store_gc",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix store gc".into()],
                extensions: vec![],
                use_when: "running nix garbage collection".into(),
            },
        )
        .add_mapping(
            "derivation_show",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix derivation show".into()],
                extensions: vec![],
                use_when: "showing derivation contents".into(),
            },
        )
        .add_mapping(
            "eval",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["nix eval".into()],
                extensions: vec![],
                use_when: "evaluating nix expressions".into(),
            },
        )
        .add_mapping(
            "fh_search",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["fh search".into()],
                extensions: vec![],
                use_when: "searching FlakeHub".into(),
            },
        )
        .add_mapping(
            "fh_add",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["fh add".into()],
                extensions: vec![],
                use_when: "adding FlakeHub inputs".into(),
            },
        )
        .add_mapping(
            "fh_list_flakes",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["fh list".into()],
                extensions: vec![],
                use_when: "listing FlakeHub flakes".into(),
            },
        )
        .add_mapping(
            "fh_resolve",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["fh resolve".into()],
                extensions: vec![],
                use_when: "resolving FlakeHub references".into(),
            },
        )
        .add_mapping(
            "fh_status",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["fh status".into()],
                extensions: vec![],
                use_when: "checking FlakeHub status".into(),
            },
        )
        .add_mapping(
            "fh_fetch",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["fh fetch".into()],
                extensions: vec![],
                use_when: "fetching from FlakeHub".into(),
            },
        )
        .add_mapping(
            "fh_login",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["fh login".into()],
                extensions: vec![],
                use_when: "logging into FlakeHub".into(),
            },
        )
        .add_mapping(
            "cachix_push",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["cachix push".into()],
                extensions: vec![],
                use_when: "pushing to Cachix".into(),
            },
        )
        .add_mapping(
            "cachix_use",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["cachix use".into()],
                extensions: vec![],
                use_when: "configuring Cachix substituter".into(),
            },
        )
        .add_mapping(
            "cachix_status",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec!["cachix status".into()],
                extensions: vec![],
                use_when: "checking Cachix status".into(),
            },
        )
}
