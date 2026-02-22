use serde::Serialize;
use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::time::Instant;
use tokio::process::Child;
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, PartialEq)]
pub enum TaskStatus {
    Running,
    Completed,
    Failed,
}

#[derive(Debug)]
pub struct BackgroundTaskHandle {
    pub id: String,
    pub command: String,
    pub status: TaskStatus,
    pub started_at: Instant,
    pub child: Option<Child>,
    pub exit_code: Option<i32>,
    pub stdout: String,
    pub stderr: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct TaskInfo {
    pub id: String,
    pub command: String,
    pub status: TaskStatus,
    pub elapsed_secs: u64,
    pub exit_code: Option<i32>,
}

lazy_static::lazy_static! {
    static ref BACKGROUND_TASKS: Arc<Mutex<HashMap<String, BackgroundTaskHandle>>> =
        Arc::new(Mutex::new(HashMap::new()));
}

pub fn generate_task_id() -> String {
    Uuid::new_v4().to_string()
}

pub fn register_task(id: String, command: String, child: Child) {
    let handle = BackgroundTaskHandle {
        id: id.clone(),
        command,
        status: TaskStatus::Running,
        started_at: Instant::now(),
        child: Some(child),
        exit_code: None,
        stdout: String::new(),
        stderr: String::new(),
    };

    let mut tasks = BACKGROUND_TASKS.lock().unwrap();
    tasks.insert(id, handle);
}

pub fn get_task_info(id: &str) -> Option<TaskInfo> {
    let tasks = BACKGROUND_TASKS.lock().unwrap();
    tasks.get(id).map(|handle| TaskInfo {
        id: handle.id.clone(),
        command: handle.command.clone(),
        status: handle.status.clone(),
        elapsed_secs: handle.started_at.elapsed().as_secs(),
        exit_code: handle.exit_code,
    })
}

pub fn list_tasks() -> Vec<TaskInfo> {
    let tasks = BACKGROUND_TASKS.lock().unwrap();
    tasks
        .values()
        .map(|handle| TaskInfo {
            id: handle.id.clone(),
            command: handle.command.clone(),
            status: handle.status.clone(),
            elapsed_secs: handle.started_at.elapsed().as_secs(),
            exit_code: handle.exit_code,
        })
        .collect()
}

pub fn update_task_status(id: &str, status: TaskStatus, exit_code: Option<i32>) {
    let mut tasks = BACKGROUND_TASKS.lock().unwrap();
    if let Some(handle) = tasks.get_mut(id) {
        handle.status = status;
        handle.exit_code = exit_code;
        handle.child = None; // Drop the child handle
    }
}

pub fn remove_task(id: &str) -> Option<BackgroundTaskHandle> {
    let mut tasks = BACKGROUND_TASKS.lock().unwrap();
    tasks.remove(id)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_generate_task_id() {
        let id1 = generate_task_id();
        let id2 = generate_task_id();
        assert_ne!(id1, id2);
        assert_eq!(id1.len(), 36); // UUID v4 format
    }

    #[test]
    fn test_task_info_serialization() {
        let info = TaskInfo {
            id: "test-id".to_string(),
            command: "test command".to_string(),
            status: TaskStatus::Running,
            elapsed_secs: 10,
            exit_code: None,
        };

        let json = serde_json::to_string(&info).unwrap();
        assert!(json.contains("test-id"));
        assert!(json.contains("Running"));
    }
}
