use super::types::App;
use std::fmt::Write;
use std::fs;
use std::io;
use std::path::Path;

impl App {
    pub fn generate_completions(&self, dir: &str) -> io::Result<()> {
        self.generate_bash_completion(dir)?;
        self.generate_zsh_completion(dir)?;
        self.generate_fish_completion(dir)
    }

    fn generate_bash_completion(&self, dir: &str) -> io::Result<()> {
        let bash_dir = Path::new(dir).join("share/bash-completion/completions");
        fs::create_dir_all(&bash_dir)?;

        let cmds = self.visible_commands();

        let mut b = String::new();
        writeln!(b, "# bash completion for {}", self.name).unwrap();
        writeln!(b).unwrap();
        writeln!(b, "_{}() {{", self.name).unwrap();
        writeln!(b, "    local cur prev commands").unwrap();
        writeln!(b, "    COMPREPLY=()").unwrap();
        writeln!(b, "    cur=\"${{COMP_WORDS[COMP_CWORD]}}\"").unwrap();
        writeln!(b, "    prev=\"${{COMP_WORDS[COMP_CWORD-1]}}\"").unwrap();
        writeln!(b).unwrap();

        let names: Vec<&str> = cmds.iter().map(|c| c.name.as_str()).collect();
        writeln!(b, "    commands=\"{}\"", names.join(" ")).unwrap();
        writeln!(b).unwrap();

        writeln!(b, "    if [[ ${{COMP_CWORD}} -eq 1 ]]; then").unwrap();
        writeln!(
            b,
            "        COMPREPLY=( $(compgen -W \"${{commands}}\" -- \"${{cur}}\") )"
        )
        .unwrap();
        writeln!(b, "        return 0").unwrap();
        writeln!(b, "    fi").unwrap();
        writeln!(b).unwrap();

        writeln!(b, "    local subcmd=\"${{COMP_WORDS[1]}}\"").unwrap();
        writeln!(b, "    case \"${{subcmd}}\" in").unwrap();
        for cmd in &cmds {
            let mut flags = Vec::new();
            for p in &cmd.params {
                flags.push(format!("--{}", p.name));
                if let Some(short) = p.short {
                    flags.push(format!("-{}", short));
                }
            }
            if !flags.is_empty() {
                writeln!(b, "        {})", cmd.name).unwrap();
                writeln!(
                    b,
                    "            COMPREPLY=( $(compgen -W \"{}\" -- \"${{cur}}\") )",
                    flags.join(" ")
                )
                .unwrap();
                writeln!(b, "            ;;").unwrap();
            }
        }
        writeln!(b, "    esac").unwrap();
        writeln!(b, "}}").unwrap();
        writeln!(b).unwrap();
        writeln!(b, "complete -F _{} {}", self.name, self.name).unwrap();

        fs::write(bash_dir.join(&self.name), b)
    }

    fn generate_zsh_completion(&self, dir: &str) -> io::Result<()> {
        let zsh_dir = Path::new(dir).join("share/zsh/site-functions");
        fs::create_dir_all(&zsh_dir)?;

        let cmds = self.visible_commands();

        let mut b = String::new();
        writeln!(b, "#compdef {}", self.name).unwrap();
        writeln!(b).unwrap();
        writeln!(b, "_{}() {{", self.name).unwrap();
        writeln!(b, "    local -a commands").unwrap();
        writeln!(b, "    commands=(").unwrap();
        for cmd in &cmds {
            let desc = cmd.description.short.replace('\'', "'\\''");
            writeln!(b, "        '{}:{}'", cmd.name, desc).unwrap();
        }
        writeln!(b, "    )").unwrap();
        writeln!(b).unwrap();
        writeln!(b, "    _describe 'command' commands").unwrap();
        writeln!(b, "}}").unwrap();
        writeln!(b).unwrap();
        writeln!(b, "_{}", self.name).unwrap();

        fs::write(zsh_dir.join(format!("_{}", self.name)), b)
    }

    fn generate_fish_completion(&self, dir: &str) -> io::Result<()> {
        let fish_dir = Path::new(dir).join("share/fish/vendor_completions.d");
        fs::create_dir_all(&fish_dir)?;

        let cmds = self.visible_commands();

        let mut b = String::new();
        writeln!(b, "# fish completion for {}", self.name).unwrap();
        writeln!(b).unwrap();
        writeln!(b, "complete -c {} -f", self.name).unwrap();
        writeln!(b).unwrap();

        for cmd in &cmds {
            let desc = cmd.description.short.replace('\'', "\\'");
            writeln!(
                b,
                "complete -c {} -n '__fish_use_subcommand' -a {} -d '{}'",
                self.name, cmd.name, desc
            )
            .unwrap();
        }

        for cmd in &cmds {
            for p in &cmd.params {
                let desc = p.description.replace('\'', "\\'");
                let short_opt = match p.short {
                    Some(c) => format!(" -s {}", c),
                    None => String::new(),
                };
                writeln!(
                    b,
                    "complete -c {} -n '__fish_seen_subcommand_from {}' -l {}{} -d '{}'",
                    self.name, cmd.name, p.name, short_opt, desc
                )
                .unwrap();
            }
        }

        fs::write(fish_dir.join(format!("{}.fish", self.name)), b)
    }
}

#[cfg(test)]
mod tests {
    use crate::command::types::*;

    fn test_app() -> App {
        let mut app = App::new("grit", "Git operations");
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
        });
        app.add_command(Command {
            name: "diff".to_string(),
            description: Description::short("Show changes"),
            params: vec![],
            hidden: false,
            aliases: vec![],
        });
        app.add_command(Command {
            name: "hidden".to_string(),
            description: Description::short("Hidden cmd"),
            params: vec![],
            hidden: true,
            aliases: vec![],
        });
        app
    }

    #[test]
    fn bash_completion_contains_commands() {
        let app = test_app();
        let dir = tempfile::tempdir().unwrap();
        app.generate_completions(dir.path().to_str().unwrap())
            .unwrap();

        let path = dir
            .path()
            .join("share/bash-completion/completions/grit");
        let content = std::fs::read_to_string(&path).unwrap();

        assert!(content.contains("status"), "missing status command");
        assert!(content.contains("diff"), "missing diff command");
        assert!(
            !content.contains("hidden"),
            "should not contain hidden commands"
        );
        assert!(content.contains("--repo_path"), "missing repo_path flag");
    }

    #[test]
    fn bash_completion_short_flags() {
        let mut app = App::new("grit", "Git operations");
        app.add_command(Command {
            name: "status".to_string(),
            description: Description::short("Show status"),
            params: vec![Param {
                name: "verbose".to_string(),
                short: Some('v'),
                param_type: ParamType::Bool,
                description: "Verbose output".to_string(),
                required: false,
                default: None,
            }],
            hidden: false,
            aliases: vec![],
        });

        let dir = tempfile::tempdir().unwrap();
        app.generate_completions(dir.path().to_str().unwrap())
            .unwrap();

        let path = dir
            .path()
            .join("share/bash-completion/completions/grit");
        let content = std::fs::read_to_string(&path).unwrap();

        assert!(content.contains("-v"), "missing short flag -v");
        assert!(content.contains("--verbose"), "missing long flag --verbose");
    }

    #[test]
    fn zsh_completion_structure() {
        let app = test_app();
        let dir = tempfile::tempdir().unwrap();
        app.generate_completions(dir.path().to_str().unwrap())
            .unwrap();

        let path = dir.path().join("share/zsh/site-functions/_grit");
        let content = std::fs::read_to_string(&path).unwrap();

        assert!(content.contains("#compdef grit"), "missing #compdef header");
        assert!(content.contains("status"), "missing status command");
        assert!(
            content.contains("Show status"),
            "missing description"
        );
        assert!(
            !content.contains("hidden"),
            "should not contain hidden commands"
        );
    }

    #[test]
    fn fish_completion_structure() {
        let app = test_app();
        let dir = tempfile::tempdir().unwrap();
        app.generate_completions(dir.path().to_str().unwrap())
            .unwrap();

        let path = dir
            .path()
            .join("share/fish/vendor_completions.d/grit.fish");
        let content = std::fs::read_to_string(&path).unwrap();

        assert!(
            content.contains("complete -c grit"),
            "missing complete -c header"
        );
        assert!(content.contains("status"), "missing status command");
        assert!(
            !content.contains("hidden"),
            "should not contain hidden commands"
        );
    }

    #[test]
    fn fish_completion_short_flags() {
        let mut app = App::new("grit", "Git operations");
        app.add_command(Command {
            name: "status".to_string(),
            description: Description::short("Show status"),
            params: vec![Param {
                name: "verbose".to_string(),
                short: Some('v'),
                param_type: ParamType::Bool,
                description: "Verbose output".to_string(),
                required: false,
                default: None,
            }],
            hidden: false,
            aliases: vec![],
        });

        let dir = tempfile::tempdir().unwrap();
        app.generate_completions(dir.path().to_str().unwrap())
            .unwrap();

        let path = dir
            .path()
            .join("share/fish/vendor_completions.d/grit.fish");
        let content = std::fs::read_to_string(&path).unwrap();

        assert!(content.contains("-s v"), "missing short flag -s v");
    }
}
