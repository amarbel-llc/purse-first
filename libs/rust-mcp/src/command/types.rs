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
}

pub struct App {
    pub name: String,
    pub description: Description,
    pub version: String,
    commands: Vec<Command>,
}

impl App {
    pub fn new(name: impl Into<String>, short_desc: impl Into<String>) -> Self {
        App {
            name: name.into(),
            description: Description::short(short_desc),
            version: String::new(),
            commands: Vec::new(),
        }
    }

    pub fn version(mut self, version: impl Into<String>) -> Self {
        self.version = version.into();
        self
    }

    pub fn add_command(&mut self, cmd: Command) {
        self.commands.push(cmd);
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
        });
        app.add_command(Command {
            name: "hidden".to_string(),
            description: Description::short("Hidden cmd"),
            params: vec![],
            hidden: true,
            aliases: vec![],
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
        });
        app.add_command(Command {
            name: "alpha".to_string(),
            description: Description::short("A cmd"),
            params: vec![],
            hidden: false,
            aliases: vec![],
        });

        let visible = app.visible_commands();
        assert_eq!(visible[0].name, "alpha");
        assert_eq!(visible[1].name, "zebra");
    }
}
