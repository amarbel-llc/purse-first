use async_trait::async_trait;
use mcp_server::server::Context;
use mcp_server::tools::{Tool, ToolError, ToolResult};
use serde_json::Value;

use crate::background::{get_task_info, list_tasks};

pub struct TaskStatusTool;

#[async_trait]
impl Tool for TaskStatusTool {
    fn name(&self) -> &str {
        "task_status"
    }

    fn description(&self) -> &str {
        "Check status of background tasks. If no task_id provided, lists all tasks."
    }

    fn input_schema(&self) -> Value {
        serde_json::json!({
            "type": "object",
            "properties": {
                "task_id": {
                    "type": "string",
                    "description": "Task ID to check. If omitted, returns all tasks."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: super::TaskStatusParams =
            serde_json::from_value(arguments).unwrap_or_default();

        let result = match params.task_id {
            Some(id) => {
                if let Some(info) = get_task_info(&id) {
                    serde_json::json!({ "task": info })
                } else {
                    serde_json::json!({ "error": format!("Task not found: {}", id) })
                }
            }
            None => {
                let tasks = list_tasks();
                serde_json::json!({ "tasks": tasks })
            }
        };

        let json = serde_json::to_string_pretty(&result)
            .map_err(|e| ToolError::Serialization(e))?;
        Ok(ToolResult::text(json))
    }
}
