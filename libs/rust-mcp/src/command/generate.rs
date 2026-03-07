use super::types::App;
use serde::Serialize;
use std::collections::BTreeSet;
use std::fs;
use std::io::{self, Write};
use std::path::Path;

#[derive(Serialize)]
struct PluginMcpServer {
    #[serde(rename = "type")]
    server_type: String,
    command: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    args: Vec<String>,
}

#[derive(Serialize)]
struct PluginAuthor {
    name: String,
}

#[derive(Serialize)]
struct PluginManifest {
    name: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    description: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    author: Option<PluginAuthor>,
    #[serde(rename = "mcpServers", skip_serializing_if = "std::collections::HashMap::is_empty")]
    mcp_servers: std::collections::HashMap<String, PluginMcpServer>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    skills: Vec<String>,
}

#[derive(Serialize)]
struct MappingToolSuggestion {
    name: String,
    use_when: String,
}

#[derive(Serialize)]
struct MappingEntry {
    replaces: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    extensions: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    command_prefixes: Vec<String>,
    tools: Vec<MappingToolSuggestion>,
    reason: String,
}

#[derive(Serialize)]
struct MappingFile {
    server: String,
    mappings: Vec<MappingEntry>,
}

#[derive(Serialize)]
struct HookCommand {
    #[serde(rename = "type")]
    hook_type: String,
    command: String,
    timeout: u32,
}

#[derive(Serialize)]
struct PreToolUseEntry {
    matcher: String,
    hooks: Vec<HookCommand>,
}

#[derive(Serialize)]
struct PostToolUseEntry {
    matcher: String,
    hooks: Vec<HookCommand>,
}

#[derive(Serialize)]
struct HooksInner {
    #[serde(rename = "PreToolUse")]
    pre_tool_use: Vec<PreToolUseEntry>,
    #[serde(rename = "PostToolUse", skip_serializing_if = "Vec::is_empty")]
    post_tool_use: Vec<PostToolUseEntry>,
}

#[derive(Serialize)]
struct HooksManifest {
    hooks: HooksInner,
}

impl App {
    fn build_plugin_manifest(&self) -> PluginManifest {
        let cmd_name = if self.mcp_binary.is_empty() {
            self.name.clone()
        } else {
            self.mcp_binary.clone()
        };

        let mut mcp_servers = std::collections::HashMap::new();
        mcp_servers.insert(
            self.name.clone(),
            PluginMcpServer {
                server_type: "stdio".to_string(),
                command: cmd_name,
                args: self.mcp_args.clone(),
            },
        );

        let author = if self.plugin_author.is_empty() {
            None
        } else {
            Some(PluginAuthor {
                name: self.plugin_author.clone(),
            })
        };

        PluginManifest {
            name: self.name.clone(),
            description: self.plugin_description.clone(),
            author,
            mcp_servers,
            skills: self.plugin_skills().to_vec(),
        }
    }

    pub fn write_plugin_json(&self, w: &mut dyn Write) -> io::Result<()> {
        let manifest = self.build_plugin_manifest();
        let mut data = serde_json::to_string_pretty(&manifest)
            .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;
        data.push('\n');
        w.write_all(data.as_bytes())
    }

    pub fn generate_plugin(&self, dir: &Path) -> io::Result<()> {
        let plugin_dir = dir.join(&self.name);
        fs::create_dir_all(&plugin_dir)?;

        let manifest = self.build_plugin_manifest();
        let mut data = serde_json::to_string_pretty(&manifest)
            .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;
        data.push('\n');

        fs::write(plugin_dir.join("plugin.json"), data)
    }

    pub fn generate_mappings(&self, dir: &Path) -> io::Result<()> {
        let mut entries = Vec::new();

        for cmd in self.commands() {
            if cmd.hidden {
                continue;
            }
            for tm in &cmd.maps_tools {
                entries.push(MappingEntry {
                    replaces: tm.replaces.clone(),
                    extensions: tm.extensions.clone(),
                    command_prefixes: tm.command_prefixes.clone(),
                    tools: vec![MappingToolSuggestion {
                        name: cmd.name.clone(),
                        use_when: tm.use_when.clone(),
                    }],
                    reason: format!("Use the {} MCP tool instead", self.name),
                });
            }
        }

        if entries.is_empty() {
            return Ok(());
        }

        let mf = MappingFile {
            server: self.name.clone(),
            mappings: entries,
        };

        let plugin_dir = dir.join(&self.name);
        fs::create_dir_all(&plugin_dir)?;

        let mut data = serde_json::to_string_pretty(&mf)
            .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;
        data.push('\n');

        fs::write(plugin_dir.join("mappings.json"), data)
    }

    pub fn generate_hooks(&self, dir: &Path) -> io::Result<()> {
        let mut replaces_set = BTreeSet::new();

        for cmd in self.commands() {
            if cmd.hidden {
                continue;
            }
            for tm in &cmd.maps_tools {
                replaces_set.insert(tm.replaces.clone());
            }
        }

        if replaces_set.is_empty() {
            return Ok(());
        }

        let matcher: String = replaces_set.into_iter().collect::<Vec<_>>().join("|");

        let hooks_dir = dir.join(&self.name).join("hooks");
        fs::create_dir_all(&hooks_dir)?;

        let post_tool_use: Vec<PostToolUseEntry> = self
            .post_tool_use_hooks
            .iter()
            .map(|h| PostToolUseEntry {
                matcher: h.matcher.clone(),
                hooks: vec![HookCommand {
                    hook_type: "command".to_string(),
                    command: h.command.clone(),
                    timeout: h.timeout,
                }],
            })
            .collect();

        let manifest = HooksManifest {
            hooks: HooksInner {
                pre_tool_use: vec![PreToolUseEntry {
                    matcher,
                    hooks: vec![HookCommand {
                        hook_type: "command".to_string(),
                        command: "${CLAUDE_PLUGIN_ROOT}/hooks/pre-tool-use".to_string(),
                        timeout: 5,
                    }],
                }],
                post_tool_use,
            },
        };

        let mut data = serde_json::to_string_pretty(&manifest)
            .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;
        data.push('\n');

        fs::write(hooks_dir.join("hooks.json"), data)?;

        let self_exe = std::env::current_exe()
            .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;
        let script = format!("#!/bin/sh\nexec '{}' hook\n", self_exe.display());
        let script_path = hooks_dir.join("pre-tool-use");
        fs::write(&script_path, script)?;

        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(&script_path, fs::Permissions::from_mode(0o755))?;
        }

        Ok(())
    }

    pub fn generate_all(&mut self, dir: &str) -> io::Result<()> {
        self.generate_all_with_skills(dir, "")
    }

    pub fn generate_all_with_skills(&mut self, dir: &str, skills_dir: &str) -> io::Result<()> {
        let base = Path::new(dir);
        let purse_dir = base.join("share").join("purse-first");

        if !skills_dir.is_empty() {
            let skills = discover_skills(skills_dir)?;
            self.set_plugin_skills(skills);

            let dst = purse_dir.join(&self.name).join("skills");
            copy_dir(Path::new(skills_dir), &dst)?;
        }

        self.generate_plugin(&purse_dir)?;
        self.generate_mappings(&purse_dir)?;
        self.generate_hooks(&purse_dir)?;
        self.generate_completions(dir)?;

        Ok(())
    }

    pub fn handle_generate_plugin(
        &mut self,
        args: &[String],
        stdout: &mut dyn Write,
    ) -> io::Result<()> {
        let mut skills_dir = String::new();
        let mut remaining = Vec::new();

        let mut i = 0;
        while i < args.len() {
            if args[i] == "--skills-dir" {
                i += 1;
                if i < args.len() {
                    skills_dir = args[i].clone();
                }
            } else {
                remaining.push(args[i].clone());
            }
            i += 1;
        }

        match remaining.len() {
            0 => self.generate_all_with_skills(".", &skills_dir),
            1 => {
                if remaining[0] == "-" {
                    self.write_plugin_json(stdout)
                } else {
                    self.generate_all_with_skills(&remaining[0], &skills_dir)
                }
            }
            _ => Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                format!(
                    "generate-plugin: expected 0 or 1 arguments, got {}",
                    remaining.len()
                ),
            )),
        }
    }
}

fn discover_skills(skills_dir: &str) -> io::Result<Vec<String>> {
    let dir = Path::new(skills_dir);
    let mut skills = Vec::new();

    if !dir.is_dir() {
        return Ok(skills);
    }

    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        if !entry.file_type()?.is_dir() {
            continue;
        }
        let skill_md = entry.path().join("SKILL.md");
        if skill_md.exists() {
            let name = entry.file_name().to_string_lossy().to_string();
            skills.push(format!("./skills/{}", name));
        }
    }

    skills.sort();
    Ok(skills)
}

fn copy_dir(src: &Path, dst: &Path) -> io::Result<()> {
    fs::create_dir_all(dst)?;

    for entry in fs::read_dir(src)? {
        let entry = entry?;
        let src_path = entry.path();
        let dst_path = dst.join(entry.file_name());

        if entry.file_type()?.is_dir() {
            copy_dir(&src_path, &dst_path)?;
        } else {
            fs::copy(&src_path, &dst_path)?;
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::command::types::*;
    use crate::hooks::ToolMapping;

    fn test_app() -> App {
        let mut app = App::new("grit", "Git operations");
        app.version = "0.1.0".to_string();
        app.add_command(Command {
            name: "status".to_string(),
            description: Description::short("Show status"),
            params: vec![Param {
                name: "repo_path".to_string(),
                short: None,
                param_type: ParamType::String,
                description: "Path to repo".to_string(),
                required: true,
                default: None,
            }],
            hidden: false,
            aliases: vec![],
            maps_tools: vec![ToolMapping {
                replaces: "Bash".to_string(),
                command_prefixes: vec!["git status".to_string()],
                extensions: vec![],
                use_when: "checking status".to_string(),
            }],
        });
        app
    }

    #[test]
    fn write_plugin_json_structure() {
        let app = test_app();
        let mut buf = Vec::new();
        app.write_plugin_json(&mut buf).unwrap();

        let v: serde_json::Value = serde_json::from_slice(&buf).unwrap();
        assert_eq!(v["name"], "grit");
        assert!(v["mcpServers"]["grit"]["command"].is_string());
        assert_eq!(v["mcpServers"]["grit"]["type"], "stdio");
        // Empty fields should be omitted
        assert!(v.get("description").is_none() || v["description"] == "");
        assert!(v.get("author").is_none());
        assert!(v.get("skills").is_none());
    }

    #[test]
    fn generate_plugin_writes_file() {
        let app = test_app();
        let dir = tempfile::tempdir().unwrap();
        let purse_dir = dir.path().join("share").join("purse-first");
        fs::create_dir_all(&purse_dir).unwrap();

        app.generate_plugin(&purse_dir).unwrap();

        let path = purse_dir.join("grit").join("plugin.json");
        assert!(path.exists(), "plugin.json should exist");

        let content = fs::read_to_string(&path).unwrap();
        let v: serde_json::Value = serde_json::from_str(&content).unwrap();
        assert_eq!(v["name"], "grit");
    }

    #[test]
    fn generate_mappings() {
        let app = test_app();
        let dir = tempfile::tempdir().unwrap();

        app.generate_mappings(dir.path()).unwrap();

        let path = dir.path().join("grit").join("mappings.json");
        assert!(path.exists(), "mappings.json should exist");

        let content = fs::read_to_string(&path).unwrap();
        let v: serde_json::Value = serde_json::from_str(&content).unwrap();
        assert_eq!(v["server"], "grit");
        assert_eq!(v["mappings"][0]["replaces"], "Bash");
        assert_eq!(v["mappings"][0]["command_prefixes"][0], "git status");
        assert_eq!(v["mappings"][0]["tools"][0]["name"], "status");
        assert_eq!(v["mappings"][0]["tools"][0]["use_when"], "checking status");
    }

    #[test]
    fn generate_mappings_skipped_when_no_mappings() {
        let mut app = App::new("grit", "Git operations");
        app.add_command(Command {
            name: "status".to_string(),
            description: Description::short("Show status"),
            params: vec![],
            hidden: false,
            aliases: vec![],
            maps_tools: vec![],
        });

        let dir = tempfile::tempdir().unwrap();
        app.generate_mappings(dir.path()).unwrap();

        let path = dir.path().join("grit").join("mappings.json");
        assert!(!path.exists(), "mappings.json should not exist when no mappings");
    }

    #[test]
    fn generate_hooks() {
        let app = test_app();
        let dir = tempfile::tempdir().unwrap();

        app.generate_hooks(dir.path()).unwrap();

        let hooks_json = dir.path().join("grit").join("hooks").join("hooks.json");
        assert!(hooks_json.exists(), "hooks.json should exist");

        let content = fs::read_to_string(&hooks_json).unwrap();
        let v: serde_json::Value = serde_json::from_str(&content).unwrap();
        assert_eq!(v["hooks"]["PreToolUse"][0]["matcher"], "Bash");
        assert_eq!(
            v["hooks"]["PreToolUse"][0]["hooks"][0]["command"],
            "${CLAUDE_PLUGIN_ROOT}/hooks/pre-tool-use"
        );
        assert_eq!(v["hooks"]["PreToolUse"][0]["hooks"][0]["timeout"], 5);

        let script_path = dir.path().join("grit").join("hooks").join("pre-tool-use");
        assert!(script_path.exists(), "pre-tool-use script should exist");
        let script = fs::read_to_string(&script_path).unwrap();
        assert!(script.starts_with("#!/bin/sh\n"));
        assert!(script.contains("hook"));
    }

    #[test]
    fn generate_hooks_skipped_when_no_mappings() {
        let mut app = App::new("grit", "Git operations");
        app.add_command(Command {
            name: "status".to_string(),
            description: Description::short("Show status"),
            params: vec![],
            hidden: false,
            aliases: vec![],
            maps_tools: vec![],
        });

        let dir = tempfile::tempdir().unwrap();
        app.generate_hooks(dir.path()).unwrap();

        let hooks_dir = dir.path().join("grit").join("hooks");
        assert!(!hooks_dir.exists(), "hooks/ should not exist when no mappings");
    }

    #[test]
    fn handle_generate_plugin_stdout_mode() {
        let mut app = test_app();
        let mut buf = Vec::new();

        app.handle_generate_plugin(&["-".to_string()], &mut buf)
            .unwrap();

        let v: serde_json::Value = serde_json::from_slice(&buf).unwrap();
        assert_eq!(v["name"], "grit");
    }

    #[test]
    fn handle_generate_plugin_directory_mode() {
        let mut app = test_app();
        let dir = tempfile::tempdir().unwrap();

        app.handle_generate_plugin(
            &[dir.path().to_str().unwrap().to_string()],
            &mut io::sink(),
        )
        .unwrap();

        let path = dir
            .path()
            .join("share")
            .join("purse-first")
            .join("grit")
            .join("plugin.json");
        assert!(path.exists(), "plugin.json should exist in directory mode");
    }

    #[test]
    fn generate_all_with_skills() {
        let mut app = App::new("chix", "Nix MCP server");
        app.version = "0.1.0".to_string();
        app.plugin_description = "Nix MCP server and skills".to_string();
        app.plugin_author = "friedenberg".to_string();

        app.add_command(Command {
            name: "eval".to_string(),
            description: Description::short("Evaluate nix expression"),
            params: vec![],
            hidden: false,
            aliases: vec![],
            maps_tools: vec![],
        });

        let skills_dir = tempfile::tempdir().unwrap();
        for name in &["nix-patterns", "flake-debugging"] {
            let skill_dir = skills_dir.path().join(name);
            fs::create_dir_all(&skill_dir).unwrap();
            fs::write(
                skill_dir.join("SKILL.md"),
                format!("# {}\n\nSkill content.\n", name),
            )
            .unwrap();
        }

        let out_dir = tempfile::tempdir().unwrap();
        app.generate_all_with_skills(
            out_dir.path().to_str().unwrap(),
            skills_dir.path().to_str().unwrap(),
        )
        .unwrap();

        let plugin_path = out_dir
            .path()
            .join("share")
            .join("purse-first")
            .join("chix")
            .join("plugin.json");
        let content = fs::read_to_string(&plugin_path).unwrap();
        let v: serde_json::Value = serde_json::from_str(&content).unwrap();

        let skills = v["skills"].as_array().unwrap();
        assert_eq!(skills.len(), 2);
        assert_eq!(skills[0], "./skills/flake-debugging");
        assert_eq!(skills[1], "./skills/nix-patterns");

        // Skills were physically copied
        for name in &["flake-debugging", "nix-patterns"] {
            let copied = out_dir
                .path()
                .join("share")
                .join("purse-first")
                .join("chix")
                .join("skills")
                .join(name)
                .join("SKILL.md");
            assert!(copied.exists(), "copied skill {} should exist", name);
        }
    }

    #[test]
    fn hidden_commands_excluded_from_mappings() {
        let mut app = App::new("test", "Test app");
        app.add_command(Command {
            name: "visible".to_string(),
            description: Description::short("Visible"),
            params: vec![],
            hidden: false,
            aliases: vec![],
            maps_tools: vec![ToolMapping {
                replaces: "Bash".to_string(),
                command_prefixes: vec!["test visible".to_string()],
                extensions: vec![],
                use_when: "visible".to_string(),
            }],
        });
        app.add_command(Command {
            name: "secret".to_string(),
            description: Description::short("Hidden"),
            params: vec![],
            hidden: true,
            aliases: vec![],
            maps_tools: vec![ToolMapping {
                replaces: "Bash".to_string(),
                command_prefixes: vec!["test secret".to_string()],
                extensions: vec![],
                use_when: "secret".to_string(),
            }],
        });

        let dir = tempfile::tempdir().unwrap();
        app.generate_mappings(dir.path()).unwrap();

        let path = dir.path().join("test").join("mappings.json");
        let content = fs::read_to_string(&path).unwrap();
        let v: serde_json::Value = serde_json::from_str(&content).unwrap();

        let mappings = v["mappings"].as_array().unwrap();
        assert_eq!(mappings.len(), 1);
        assert_eq!(mappings[0]["tools"][0]["name"], "visible");
    }
}
