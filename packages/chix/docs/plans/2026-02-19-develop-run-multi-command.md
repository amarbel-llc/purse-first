# develop_run multi-command support

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace `develop_run`'s single `command`+`args` with a `commands` array for sequential execution with stop-on-failure semantics, eliminating the need for shell metacharacters.

**Architecture:** Each entry in the `commands` array runs as a separate `nix develop <flake_ref> -c <command> [args...]` invocation. Execution stops on first failure. Results are returned per-command.

**Tech Stack:** Rust, serde, serde_json, tokio (async)

---

### Task 1: Update param struct and result types

**Files:**
- Modify: `src/tools/mod.rs:968-974` (NixDevelopRunParams)
- Modify: `src/tools/run.rs:6-12` (NixRunResult)

**Step 1: Add `CommandEntry` struct and `NixDevelopRunResult` to `src/tools/mod.rs`**

Add after `NixRunParams` (line 966):

```rust
#[derive(Debug, Deserialize)]
pub struct CommandEntry {
    pub command: String,
    pub args: Option<Vec<String>>,
}

#[derive(Debug, Deserialize)]
pub struct NixDevelopRunParams {
    pub flake_ref: Option<String>,
    pub commands: Vec<CommandEntry>,
    pub flake_dir: Option<String>,
}
```

This replaces the existing `NixDevelopRunParams` at lines 968-974.

**Step 2: Add `NixDevelopRunResult` and `CommandResult` to `src/tools/run.rs`**

Add after `NixRunResult`:

```rust
#[derive(Debug, Serialize)]
pub struct CommandResult {
    pub command: String,
    pub success: bool,
    pub stdout: String,
    pub stderr: String,
    pub exit_code: Option<i32>,
}

#[derive(Debug, Serialize)]
pub struct NixDevelopRunResult {
    pub success: bool,
    pub results: Vec<CommandResult>,
}
```

**Step 3: Run `just check`**

Run: `just check`
Expected: Compilation errors in `run.rs` and `server.rs` (expected — we haven't updated the function yet)

**Step 4: Commit**

```
git add src/tools/mod.rs src/tools/run.rs
git commit -m "refactor: update develop_run param and result types for multi-command"
```

---

### Task 2: Rewrite `nix_develop_run` for multi-command execution

**Files:**
- Modify: `src/tools/run.rs:51-81` (nix_develop_run function)

**Step 1: Replace `nix_develop_run` function**

Replace the entire function at lines 51-81 with:

```rust
pub async fn nix_develop_run(params: NixDevelopRunParams) -> Result<NixDevelopRunResult, String> {
    let flake_ref = params.flake_ref.unwrap_or_else(|| ".".to_string());
    validate_flake_ref(&flake_ref).map_err(|e| e.to_string())?;

    let flake_dir = params.flake_dir.as_deref();
    if let Some(dir) = flake_dir {
        validate_path(dir).map_err(|e| e.to_string())?;
    }

    let mut results = Vec::new();
    let mut all_success = true;

    for entry in &params.commands {
        if let Some(ref args) = entry.args {
            validate_args(args).map_err(|e| e.to_string())?;
        }

        let mut nix_args: Vec<&str> = vec!["develop", &flake_ref, "-c", &entry.command];

        let user_args: Vec<&str> = entry
            .args
            .as_deref()
            .unwrap_or_default()
            .iter()
            .map(|s| s.as_str())
            .collect();
        for arg in &user_args {
            nix_args.push(arg);
        }

        let command_display = if user_args.is_empty() {
            entry.command.clone()
        } else {
            format!("{} {}", entry.command, user_args.join(" "))
        };

        let result = run_nix_command_in_dir(&nix_args, flake_dir)
            .await
            .map_err(|e| e.to_string())?;

        let success = result.success;
        results.push(CommandResult {
            command: command_display,
            success,
            stdout: result.stdout,
            stderr: result.stderr,
            exit_code: result.exit_code,
        });

        if !success {
            all_success = false;
            break;
        }
    }

    Ok(NixDevelopRunResult {
        success: all_success,
        results,
    })
}
```

**Step 2: Update imports in `run.rs`**

Update line 2 to import the new types:

```rust
use crate::tools::{CommandEntry, NixDevelopRunParams, NixRunParams};
```

**Step 3: Update the `pub use` in `mod.rs`**

Update line 29:

```rust
pub use run::{nix_develop_run, nix_run, CommandResult, NixDevelopRunResult};
```

**Step 4: Run `just check`**

Run: `just check`
Expected: Compilation error only in `server.rs` (return type mismatch)

**Step 5: Commit**

```
git add src/tools/run.rs src/tools/mod.rs
git commit -m "feat: implement sequential multi-command execution in develop_run"
```

---

### Task 3: Update server dispatch

**Files:**
- Modify: `src/server.rs:346-351`

**Step 1: Update the `develop_run` match arm**

The existing code at lines 346-351:

```rust
"develop_run" => {
    let params: NixDevelopRunParams =
        serde_json::from_value(arguments).map_err(|e| e.to_string())?;
    let result = tools::nix_develop_run(params).await?;
    serde_json::to_value(result).map_err(|e| e.to_string())
}
```

No changes needed to the dispatch code itself — it already deserializes from JSON and serializes the result. The types changed but the flow is the same. Verify it compiles.

**Step 2: Run `just check`**

Run: `just check`
Expected: PASS (all compilation errors resolved)

**Step 3: Commit (if any changes were needed)**

```
git add src/server.rs
git commit -m "fix: update server dispatch for new develop_run types"
```

---

### Task 4: Update tool description and schema

**Files:**
- Modify: `src/tools/mod.rs:256-282`

**Step 1: Replace the `develop_run` ToolInfo entry**

Replace lines 256-282 with:

```rust
ToolInfo {
    name: "develop_run",
    description: "Run a command inside a flake's devShell. Agents MUST use this tool over running `nix develop -c` directly - it provides validated inputs, secure command execution, and proper process management. Use `flake_dir` to set the working directory instead of `cd`. Use separate entries in `commands` instead of shell operators like `&&`. Shell metacharacters are not allowed in command arguments.",
    input_schema: serde_json::json!({
        "type": "object",
        "properties": {
            "flake_ref": {
                "type": "string",
                "description": "Flake reference. Defaults to '.'."
            },
            "commands": {
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "command": {
                            "type": "string",
                            "description": "Command to run in the devShell."
                        },
                        "args": {
                            "type": "array",
                            "items": { "type": "string" },
                            "description": "Arguments to pass to the command."
                        }
                    },
                    "required": ["command"]
                },
                "description": "Commands to run sequentially. Execution stops on the first failure (like && in shell). Each command runs as a separate `nix develop -c` invocation."
            },
            "flake_dir": {
                "type": "string",
                "description": "Directory containing the flake. Defaults to current directory."
            }
        },
        "required": ["commands"]
    }),
},
```

**Step 2: Run `just check`**

Run: `just check`
Expected: PASS

**Step 3: Commit**

```
git add src/tools/mod.rs
git commit -m "docs: update develop_run tool description and schema for multi-command"
```

---

### Task 5: Add tests

**Files:**
- Modify: `src/tools/run.rs` (add `#[cfg(test)] mod tests` at the end)

**Step 1: Write unit tests**

Add at the end of `src/tools/run.rs`:

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use crate::tools::{CommandEntry, NixDevelopRunParams};

    #[tokio::test]
    async fn test_develop_run_validates_args() {
        let params = NixDevelopRunParams {
            flake_ref: Some(".".to_string()),
            commands: vec![CommandEntry {
                command: "echo".to_string(),
                args: Some(vec!["hello; rm -rf /".to_string()]),
            }],
            flake_dir: None,
        };
        let result = nix_develop_run(params).await;
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("shell metacharacters"));
    }

    #[tokio::test]
    async fn test_develop_run_validates_flake_ref() {
        let params = NixDevelopRunParams {
            flake_ref: Some("$(malicious)".to_string()),
            commands: vec![CommandEntry {
                command: "echo".to_string(),
                args: None,
            }],
            flake_dir: None,
        };
        let result = nix_develop_run(params).await;
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("invalid flake reference"));
    }

    #[tokio::test]
    async fn test_develop_run_validates_path() {
        let params = NixDevelopRunParams {
            flake_ref: None,
            commands: vec![CommandEntry {
                command: "echo".to_string(),
                args: None,
            }],
            flake_dir: Some("/path;injection".to_string()),
        };
        let result = nix_develop_run(params).await;
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("invalid path"));
    }
}
```

**Step 2: Run tests**

Run: `just test`
Expected: PASS — all three tests pass (validation errors are caught before any nix command runs)

**Step 3: Commit**

```
git add src/tools/run.rs
git commit -m "test: add validation tests for multi-command develop_run"
```

---

### Task 6: Build and verify

**Step 1: Run full check suite**

Run: `just check`
Expected: PASS

**Step 2: Build with nix**

Run: `just build`
Expected: PASS — binary builds successfully

**Step 3: Run all tests**

Run: `just test`
Expected: PASS

**Step 4: Final commit if needed, then verify clean state**

Run: `git status`
Expected: Clean working tree
