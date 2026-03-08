use serde_json::{json, Value};
use std::collections::BTreeSet;
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::Path;

#[derive(Debug, Clone)]
pub struct ToolMapping {
    pub replaces: String,
    pub command_prefixes: Vec<String>,
    pub extensions: Vec<String>,
    pub use_when: String,
}

struct CommandMapping {
    command_name: String,
    mapping: ToolMapping,
}

pub struct HookHandler {
    app_name: String,
    mappings: Vec<CommandMapping>,
}

impl HookHandler {
    pub fn new(app_name: impl Into<String>) -> Self {
        Self {
            app_name: app_name.into(),
            mappings: Vec::new(),
        }
    }

    pub fn add_mapping(mut self, command_name: impl Into<String>, mapping: ToolMapping) -> Self {
        self.mappings.push(CommandMapping {
            command_name: command_name.into(),
            mapping,
        });
        self
    }

    /// Read hook input from stdin JSON, check against all mappings, and return
    /// deny output bytes if a match is found. Returns Ok(None) for implicit
    /// allow or on any error (fail-open).
    pub fn handle_hook(&self, input: &[u8]) -> Result<Option<Vec<u8>>, anyhow::Error> {
        let hi: Value = match serde_json::from_slice(input) {
            Ok(v) => v,
            Err(_) => return Ok(None),
        };

        let tool_name = match hi.get("tool_name").and_then(|v| v.as_str()) {
            Some(s) => s,
            None => return Ok(None),
        };

        let tool_input = hi.get("tool_input");

        let file_path = extract_string(tool_input, &["file_path", "path", "pattern"]);
        let command = extract_string(tool_input, &["command"]);

        let commands = if tool_name == "Bash" && !command.is_empty() {
            extract_simple_commands(&command)
        } else {
            vec![command]
        };

        for cm in &self.mappings {
            if cm.mapping.replaces != tool_name {
                continue;
            }

            for cmd in &commands {
                if matches_mapping(&cm.mapping, &file_path, cmd) {
                    let mcp_tool = format!(
                        "mcp__plugin_{name}_{name}__{cmd}",
                        name = self.app_name,
                        cmd = cm.command_name
                    );

                    let reason = format!(
                        "Use the MCP tool instead:\n- {}: {}",
                        mcp_tool, cm.mapping.use_when
                    );

                    let output = json!({
                        "hookSpecificOutput": {
                            "hookEventName": "PreToolUse",
                            "permissionDecision": "deny",
                            "permissionDecisionReason": reason,
                        }
                    });

                    let bytes = serde_json::to_vec(&output)?;
                    return Ok(Some(bytes));
                }
            }
        }

        Ok(None)
    }

    /// Merge PreToolUse hooks into an existing plugin.json and write the
    /// pre-tool-use hook script into the hooks/ directory next to it.
    pub fn generate_hooks(
        &self,
        plugin_json_path: &Path,
        binary_path: &Path,
    ) -> Result<(), anyhow::Error> {
        let mut replaces_set = BTreeSet::new();
        for cm in &self.mappings {
            replaces_set.insert(cm.mapping.replaces.clone());
        }

        if replaces_set.is_empty() {
            return Ok(());
        }

        let matcher: String = replaces_set.into_iter().collect::<Vec<_>>().join("|");

        let data = fs::read_to_string(plugin_json_path)?;
        let mut plugin: Value = serde_json::from_str(&data)?;

        let hooks = plugin
            .as_object_mut()
            .ok_or_else(|| anyhow::anyhow!("plugin.json is not an object"))?
            .entry("hooks")
            .or_insert_with(|| json!({}));

        let hooks_obj = hooks
            .as_object_mut()
            .ok_or_else(|| anyhow::anyhow!("hooks is not an object"))?;

        hooks_obj.insert(
            "PreToolUse".into(),
            json!([{
                "matcher": matcher,
                "hooks": [{
                    "type": "command",
                    "command": "${CLAUDE_PLUGIN_ROOT}/hooks/pre-tool-use",
                    "timeout": 5,
                }]
            }]),
        );

        let out = serde_json::to_string_pretty(&plugin)? + "\n";
        fs::write(plugin_json_path, out)?;

        // Write the hook script
        let hooks_dir = plugin_json_path.parent().unwrap().join("hooks");
        fs::create_dir_all(&hooks_dir)?;

        let script = format!("#!/bin/sh\nexec '{}' hook\n", binary_path.display());
        let script_path = hooks_dir.join("pre-tool-use");
        fs::write(&script_path, script)?;
        fs::set_permissions(&script_path, fs::Permissions::from_mode(0o755))?;

        Ok(())
    }
}

fn extract_string(tool_input: Option<&Value>, keys: &[&str]) -> String {
    let obj = match tool_input {
        Some(v) => v,
        None => return String::new(),
    };

    for key in keys {
        if let Some(s) = obj.get(*key).and_then(|v| v.as_str()) {
            if !s.is_empty() {
                return s.to_string();
            }
        }
    }

    String::new()
}

fn matches_mapping(mapping: &ToolMapping, file_path: &str, command: &str) -> bool {
    let has_extensions = !mapping.extensions.is_empty();
    let has_prefixes = !mapping.command_prefixes.is_empty();

    // Catch-all: no extensions and no prefixes means match everything.
    if !has_extensions && !has_prefixes {
        return true;
    }

    if has_extensions && !file_path.is_empty() {
        let ext = file_path
            .rsplit_once('.')
            .map(|(_, e)| format!(".{}", e.to_lowercase()))
            .unwrap_or_default();
        for e in &mapping.extensions {
            if e.to_lowercase() == ext {
                return true;
            }
        }
    }

    if has_prefixes && !command.is_empty() {
        for prefix in &mapping.command_prefixes {
            if command.starts_with(prefix.as_str()) {
                return true;
            }
        }
    }

    false
}

/// Split a bash command on &&, ||, ;, | to get simple sub-commands.
/// Falls back to the original command on any edge case.
fn extract_simple_commands(command: &str) -> Vec<String> {
    if command.is_empty() {
        return vec![String::new()];
    }

    let mut commands = Vec::new();
    let mut current = String::new();
    let mut chars = command.chars().peekable();
    let mut in_single_quote = false;
    let mut in_double_quote = false;
    let mut escape_next = false;

    while let Some(ch) = chars.next() {
        if escape_next {
            current.push(ch);
            escape_next = false;
            continue;
        }

        if ch == '\\' && !in_single_quote {
            current.push(ch);
            escape_next = true;
            continue;
        }

        if ch == '\'' && !in_double_quote {
            in_single_quote = !in_single_quote;
            current.push(ch);
            continue;
        }

        if ch == '"' && !in_single_quote {
            in_double_quote = !in_double_quote;
            current.push(ch);
            continue;
        }

        if in_single_quote || in_double_quote {
            current.push(ch);
            continue;
        }

        match ch {
            '&' if chars.peek() == Some(&'&') => {
                chars.next();
                let trimmed = current.trim().to_string();
                if !trimmed.is_empty() {
                    commands.push(trimmed);
                }
                current.clear();
            }
            '|' if chars.peek() == Some(&'|') => {
                chars.next();
                let trimmed = current.trim().to_string();
                if !trimmed.is_empty() {
                    commands.push(trimmed);
                }
                current.clear();
            }
            '|' => {
                let trimmed = current.trim().to_string();
                if !trimmed.is_empty() {
                    commands.push(trimmed);
                }
                current.clear();
            }
            ';' => {
                let trimmed = current.trim().to_string();
                if !trimmed.is_empty() {
                    commands.push(trimmed);
                }
                current.clear();
            }
            _ => {
                current.push(ch);
            }
        }
    }

    let trimmed = current.trim().to_string();
    if !trimmed.is_empty() {
        commands.push(trimmed);
    }

    if commands.is_empty() {
        vec![command.to_string()]
    } else {
        commands
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn test_handler() -> HookHandler {
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
                "eval",
                ToolMapping {
                    replaces: "Bash".into(),
                    command_prefixes: vec!["nix eval".into()],
                    extensions: vec![],
                    use_when: "evaluating nix expressions".into(),
                },
            )
            .add_mapping(
                "nil_diagnostics",
                ToolMapping {
                    replaces: "Read".into(),
                    command_prefixes: vec![],
                    extensions: vec![".nix".into()],
                    use_when: "reading nix files".into(),
                },
            )
    }

    #[test]
    fn violation_produces_deny() {
        let handler = test_handler();
        let input = br#"{"tool_name":"Bash","tool_input":{"command":"nix build .#foo"}}"#;
        let result = handler.handle_hook(input).unwrap();
        assert!(result.is_some());
        let output: Value = serde_json::from_slice(&result.unwrap()).unwrap();
        assert_eq!(
            output["hookSpecificOutput"]["hookEventName"],
            "PreToolUse"
        );
        assert_eq!(
            output["hookSpecificOutput"]["permissionDecision"],
            "deny"
        );
        let reason = output["hookSpecificOutput"]["permissionDecisionReason"]
            .as_str()
            .unwrap();
        assert!(reason.contains("mcp__plugin_chix_chix__build"));
        assert!(reason.contains("building nix packages"));
    }

    #[test]
    fn no_violation_produces_none() {
        let handler = test_handler();
        let input = br#"{"tool_name":"Bash","tool_input":{"command":"echo hello"}}"#;
        let result = handler.handle_hook(input).unwrap();
        assert!(result.is_none());
    }

    #[test]
    fn catch_all_mapping() {
        let handler = HookHandler::new("test").add_mapping(
            "catch_all",
            ToolMapping {
                replaces: "Bash".into(),
                command_prefixes: vec![],
                extensions: vec![],
                use_when: "always".into(),
            },
        );
        let input = br#"{"tool_name":"Bash","tool_input":{"command":"anything"}}"#;
        let result = handler.handle_hook(input).unwrap();
        assert!(result.is_some());
    }

    #[test]
    fn extension_matching() {
        let handler = test_handler();
        let input =
            br#"{"tool_name":"Read","tool_input":{"file_path":"/foo/bar.nix"}}"#;
        let result = handler.handle_hook(input).unwrap();
        assert!(result.is_some());
        let output: Value = serde_json::from_slice(&result.unwrap()).unwrap();
        assert_eq!(
            output["hookSpecificOutput"]["permissionDecision"],
            "deny"
        );
    }

    #[test]
    fn extension_no_match() {
        let handler = test_handler();
        let input =
            br#"{"tool_name":"Read","tool_input":{"file_path":"/foo/bar.rs"}}"#;
        let result = handler.handle_hook(input).unwrap();
        assert!(result.is_none());
    }

    #[test]
    fn prefix_matching_compound_command() {
        let handler = test_handler();
        let input =
            br#"{"tool_name":"Bash","tool_input":{"command":"cd /foo && nix build .#bar"}}"#;
        let result = handler.handle_hook(input).unwrap();
        assert!(result.is_some());
    }

    #[test]
    fn decode_error_returns_none() {
        let handler = test_handler();
        let result = handler.handle_hook(b"not json").unwrap();
        assert!(result.is_none());
    }

    #[test]
    fn generate_hooks_merges_into_plugin_json() {
        let dir = tempfile::tempdir().unwrap();
        let plugin_json_path = dir.path().join("plugin.json");

        // Start with an existing plugin.json that has PostToolUse hooks
        let existing = json!({
            "name": "chix",
            "hooks": {
                "PostToolUse": [{
                    "matcher": "Edit|Write",
                    "hooks": [{"type": "command", "command": "format-nix", "timeout": 30}]
                }]
            }
        });
        fs::write(&plugin_json_path, serde_json::to_string_pretty(&existing).unwrap()).unwrap();

        let handler = test_handler();
        let binary = Path::new("/nix/store/fake-hash-chix/bin/chix");
        handler.generate_hooks(&plugin_json_path, binary).unwrap();

        let result: Value =
            serde_json::from_str(&fs::read_to_string(&plugin_json_path).unwrap()).unwrap();

        // PreToolUse was added
        let matcher = result["hooks"]["PreToolUse"][0]["matcher"]
            .as_str()
            .unwrap();
        assert_eq!(matcher, "Bash|Read");

        // PostToolUse was preserved
        assert_eq!(
            result["hooks"]["PostToolUse"][0]["matcher"],
            "Edit|Write"
        );

        // pre-tool-use script was written
        let script_path = dir.path().join("hooks/pre-tool-use");
        assert!(script_path.exists());
        let script = fs::read_to_string(&script_path).unwrap();
        assert!(script.starts_with("#!/bin/sh\n"));
        assert!(script.contains("chix"));
    }

    #[test]
    fn extract_simple_commands_splits_operators() {
        assert_eq!(
            extract_simple_commands("cd /foo && nix build .#bar"),
            vec!["cd /foo", "nix build .#bar"]
        );
        assert_eq!(
            extract_simple_commands("nix build .#foo || echo fail"),
            vec!["nix build .#foo", "echo fail"]
        );
        assert_eq!(
            extract_simple_commands("nix log .#foo | head -5"),
            vec!["nix log .#foo", "head -5"]
        );
        assert_eq!(
            extract_simple_commands("nix build .#a; nix build .#b"),
            vec!["nix build .#a", "nix build .#b"]
        );
    }

    #[test]
    fn extract_simple_commands_preserves_quoted() {
        assert_eq!(
            extract_simple_commands(r#"echo "a && b""#),
            vec![r#"echo "a && b""#]
        );
    }
}
