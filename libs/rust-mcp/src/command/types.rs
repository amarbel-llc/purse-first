use crate::hooks::ToolMapping;
use serde_json::Value;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ParamType {
    String,
    Int,
    Bool,
    Float,
    Array,
}

impl ParamType {
    pub fn json_schema_type(&self) -> &'static str {
        match self {
            ParamType::String => "string",
            ParamType::Int => "integer",
            ParamType::Bool => "boolean",
            ParamType::Float => "number",
            ParamType::Array => "array",
        }
    }
}

#[derive(Debug, Clone)]
pub struct Description {
    pub short: String,
    pub long: String,
}

impl Description {
    pub fn short(s: impl Into<String>) -> Self {
        Description {
            short: s.into(),
            long: String::new(),
        }
    }
}

#[derive(Debug, Clone)]
pub struct Param {
    pub name: String,
    pub short: Option<char>,
    pub param_type: ParamType,
    pub description: String,
    pub required: bool,
    pub default: Option<Value>,
}

#[derive(Debug, Clone)]
pub struct Command {
    pub name: String,
    pub description: Description,
    pub params: Vec<Param>,
    pub hidden: bool,
    pub aliases: Vec<String>,
    pub maps_tools: Vec<ToolMapping>,
}

#[derive(Debug, Clone)]
pub struct PostToolUseHook {
    pub matcher: String,
    pub command: String,
    pub timeout: u32,
}

pub struct App {
    pub name: String,
    pub description: Description,
    pub version: String,
    pub plugin_description: String,
    pub plugin_author: String,
    pub mcp_binary: String,
    pub mcp_args: Vec<String>,
    plugin_skills: Vec<String>,
    commands: Vec<Command>,
    pub post_tool_use_hooks: Vec<PostToolUseHook>,
}

impl App {
    pub fn new(name: impl Into<String>, short_desc: impl Into<String>) -> Self {
        App {
            name: name.into(),
            description: Description::short(short_desc),
            version: String::new(),
            plugin_description: String::new(),
            plugin_author: String::new(),
            mcp_binary: String::new(),
            mcp_args: Vec::new(),
            plugin_skills: Vec::new(),
            commands: Vec::new(),
            post_tool_use_hooks: Vec::new(),
        }
    }

    pub fn version(mut self, version: impl Into<String>) -> Self {
        self.version = version.into();
        self
    }

    pub fn plugin_description(mut self, desc: impl Into<String>) -> Self {
        self.plugin_description = desc.into();
        self
    }

    pub fn plugin_author(mut self, author: impl Into<String>) -> Self {
        self.plugin_author = author.into();
        self
    }

    pub fn mcp_binary(mut self, binary: impl Into<String>) -> Self {
        self.mcp_binary = binary.into();
        self
    }

    pub fn mcp_args(mut self, args: Vec<String>) -> Self {
        self.mcp_args = args;
        self
    }

    pub fn add_post_tool_use_hook(&mut self, hook: PostToolUseHook) -> &mut Self {
        self.post_tool_use_hooks.push(hook);
        self
    }

    pub fn add_command(&mut self, cmd: Command) {
        self.commands.push(cmd);
    }

    pub fn commands(&self) -> &[Command] {
        &self.commands
    }

    pub fn plugin_skills(&self) -> &[String] {
        &self.plugin_skills
    }

    pub fn set_plugin_skills(&mut self, skills: Vec<String>) {
        self.plugin_skills = skills;
    }

    pub fn visible_commands(&self) -> Vec<&Command> {
        let mut cmds: Vec<&Command> = self.commands.iter().filter(|c| !c.hidden).collect();
        cmds.sort_by(|a, b| a.name.cmp(&b.name));
        cmds
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn param_type_json_schema() {
        assert_eq!(ParamType::String.json_schema_type(), "string");
        assert_eq!(ParamType::Int.json_schema_type(), "integer");
        assert_eq!(ParamType::Bool.json_schema_type(), "boolean");
        assert_eq!(ParamType::Float.json_schema_type(), "number");
        assert_eq!(ParamType::Array.json_schema_type(), "array");
    }

    #[test]
    fn app_add_and_list_commands() {
        let mut app = App::new("test", "A test app");
        app.add_command(Command {
            name: "status".to_string(),
            description: Description {
                short: "Show status".to_string(),
                long: String::new(),
            },
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
            maps_tools: vec![],
        });
        app.add_command(Command {
            name: "hidden".to_string(),
            description: Description::short("Hidden cmd"),
            params: vec![],
            hidden: true,
            aliases: vec![],
            maps_tools: vec![],
        });

        assert_eq!(app.visible_commands().len(), 1);
        assert_eq!(app.visible_commands()[0].name, "status");
    }

    #[test]
    fn app_sorted_visible_commands() {
        let mut app = App::new("test", "A test app");
        app.add_command(Command {
            name: "zebra".to_string(),
            description: Description::short("Z cmd"),
            params: vec![],
            hidden: false,
            aliases: vec![],
            maps_tools: vec![],
        });
        app.add_command(Command {
            name: "alpha".to_string(),
            description: Description::short("A cmd"),
            params: vec![],
            hidden: false,
            aliases: vec![],
            maps_tools: vec![],
        });

        let visible = app.visible_commands();
        assert_eq!(visible[0].name, "alpha");
        assert_eq!(visible[1].name, "zebra");
    }
}
