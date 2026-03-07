use super::types::App;
use crate::hooks::HookHandler;

impl App {
    pub fn handle_hook(&self, input: &[u8]) -> Result<Option<Vec<u8>>, anyhow::Error> {
        let mut handler = HookHandler::new(&self.name);
        for cmd in self.commands() {
            if cmd.hidden {
                continue;
            }
            for tm in &cmd.maps_tools {
                handler = handler.add_mapping(&cmd.name, tm.clone());
            }
        }
        handler.handle_hook(input)
    }
}

#[cfg(test)]
mod tests {
    use crate::command::types::*;
    use crate::hooks::ToolMapping;

    fn test_app() -> App {
        let mut app = App::new("chix", "Nix MCP server");
        app.add_command(Command {
            name: "build".to_string(),
            description: Description::short("Build nix packages"),
            params: vec![],
            hidden: false,
            aliases: vec![],
            maps_tools: vec![ToolMapping {
                replaces: "Bash".to_string(),
                command_prefixes: vec!["nix build".to_string()],
                extensions: vec![],
                use_when: "building nix packages".to_string(),
            }],
        });
        app.add_command(Command {
            name: "hidden_cmd".to_string(),
            description: Description::short("Hidden"),
            params: vec![],
            hidden: true,
            aliases: vec![],
            maps_tools: vec![ToolMapping {
                replaces: "Bash".to_string(),
                command_prefixes: vec!["nix secret".to_string()],
                extensions: vec![],
                use_when: "secret".to_string(),
            }],
        });
        app
    }

    #[test]
    fn handle_hook_denies_mapped_tool() {
        let app = test_app();
        let input = br#"{"tool_name":"Bash","tool_input":{"command":"nix build .#foo"}}"#;
        let result = app.handle_hook(input).unwrap();
        assert!(result.is_some());
        let output: serde_json::Value = serde_json::from_slice(&result.unwrap()).unwrap();
        assert_eq!(
            output["hookSpecificOutput"]["permissionDecision"],
            "deny"
        );
    }

    #[test]
    fn handle_hook_allows_unmapped_tool() {
        let app = test_app();
        let input = br#"{"tool_name":"Bash","tool_input":{"command":"echo hello"}}"#;
        let result = app.handle_hook(input).unwrap();
        assert!(result.is_none());
    }

    #[test]
    fn handle_hook_skips_hidden_commands() {
        let app = test_app();
        let input = br#"{"tool_name":"Bash","tool_input":{"command":"nix secret stuff"}}"#;
        let result = app.handle_hook(input).unwrap();
        assert!(result.is_none());
    }
}
