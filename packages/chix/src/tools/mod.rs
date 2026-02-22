mod build;
mod cachix;
mod derivation;
mod eval;
mod flake;
mod flakehub;
mod hash;
mod log;
mod lsp;
mod run;
mod search;
mod store;

pub use build::nix_build;
pub use cachix::{cachix_push, cachix_status, cachix_use};
pub use derivation::nix_derivation_show;
pub use eval::nix_eval;
pub use flake::{
    nix_flake_check, nix_flake_init, nix_flake_lock, nix_flake_metadata, nix_flake_show,
    nix_flake_update,
};
pub use flakehub::{
    fh_add, fh_fetch, fh_list_flakes, fh_list_releases, fh_list_versions, fh_login, fh_resolve,
    fh_search, fh_status,
};
pub use hash::{nix_hash_file, nix_hash_path};
pub use log::nix_log;
pub use lsp::{nil_completions, nil_definition, nil_diagnostics, nil_hover};
pub use run::{nix_develop_run, nix_run, CommandResult, NixDevelopRunResult};
pub use search::nix_search;
pub use store::{nix_copy, nix_store_cat, nix_store_gc, nix_store_ls, nix_store_path_info};

use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize)]
pub struct ToolInfo {
    pub name: &'static str,
    pub description: &'static str,
    pub input_schema: serde_json::Value,
}

pub fn list_tools() -> Vec<ToolInfo> {
    vec![
        ToolInfo {
            name: "build",
            description: "Build a nix flake package. Returns store paths on success. Agents MUST use this tool over running `nix build` directly - it provides validated inputs, structured output, and proper error handling.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "installable": {
                        "type": "string",
                        "description": "Flake installable (e.g., '.#default', 'nixpkgs#hello'). Defaults to '.#default'."
                    },
                    "print_build_logs": {
                        "type": "boolean",
                        "description": "Whether to print build logs (-L flag). Defaults to true."
                    },
                    "flake_dir": {
                        "type": "string",
                        "description": "Directory containing the flake. Defaults to current directory."
                    },
                    "max_log_bytes": {
                        "type": "integer",
                        "description": "Maximum bytes of build log output to return. Defaults to config value (100KB)."
                    },
                    "log_tail": {
                        "type": "integer",
                        "description": "Only return the last N lines of build log. Takes precedence over max_log_bytes."
                    }
                }
            }),
        },
        ToolInfo {
            name: "flake_show",
            description: "List outputs of a nix flake. Agents MUST use this tool over running `nix flake show` directly - it provides validated inputs and consistent JSON output.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "flake_ref": {
                        "type": "string",
                        "description": "Flake reference (e.g., '.', 'github:NixOS/nixpkgs'). Defaults to '.'."
                    },
                    "all_systems": {
                        "type": "boolean",
                        "description": "Show outputs for all systems. Defaults to false."
                    },
                    "flake_dir": {
                        "type": "string",
                        "description": "Directory containing the flake. Defaults to current directory."
                    },
                    "max_bytes": {
                        "type": "integer",
                        "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                    },
                    "head": {
                        "type": "integer",
                        "description": "Only return the first N lines of output."
                    },
                    "tail": {
                        "type": "integer",
                        "description": "Only return the last N lines of output."
                    }
                }
            }),
        },
        ToolInfo {
            name: "flake_check",
            description: "Run flake checks and tests. PREFER this tool over running `nix flake check` directly - it provides validated inputs, proper timeout handling, and structured results.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "flake_ref": {
                        "type": "string",
                        "description": "Flake reference. Defaults to '.'."
                    },
                    "keep_going": {
                        "type": "boolean",
                        "description": "Continue on error. Defaults to true."
                    },
                    "flake_dir": {
                        "type": "string",
                        "description": "Directory containing the flake. Defaults to current directory."
                    },
                    "max_bytes": {
                        "type": "integer",
                        "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                    },
                    "head": {
                        "type": "integer",
                        "description": "Only return the first N lines of output."
                    },
                    "tail": {
                        "type": "integer",
                        "description": "Only return the last N lines of output."
                    }
                }
            }),
        },
        ToolInfo {
            name: "flake_metadata",
            description: "Get metadata for a flake including inputs, locked revisions, and timestamps. PREFER this tool over running `nix flake metadata` directly - it provides validated inputs and consistent JSON output.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "flake_ref": {
                        "type": "string",
                        "description": "Flake reference (e.g., '.', 'github:NixOS/nixpkgs'). Defaults to '.'."
                    },
                    "flake_dir": {
                        "type": "string",
                        "description": "Directory containing the flake. Defaults to current directory."
                    },
                    "max_bytes": {
                        "type": "integer",
                        "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                    },
                    "head": {
                        "type": "integer",
                        "description": "Only return the first N lines of output."
                    },
                    "tail": {
                        "type": "integer",
                        "description": "Only return the last N lines of output."
                    }
                }
            }),
        },
        ToolInfo {
            name: "flake_update",
            description: "Update flake.lock file. PREFER this tool over running `nix flake update` directly - it provides validated inputs and proper error handling.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "flake_ref": {
                        "type": "string",
                        "description": "Flake reference. Defaults to '.'."
                    },
                    "inputs": {
                        "type": "array",
                        "items": { "type": "string" },
                        "description": "Specific inputs to update. If empty, updates all inputs."
                    },
                    "flake_dir": {
                        "type": "string",
                        "description": "Directory containing the flake. Defaults to current directory."
                    },
                    "max_bytes": {
                        "type": "integer",
                        "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                    },
                    "head": {
                        "type": "integer",
                        "description": "Only return the first N lines of output."
                    },
                    "tail": {
                        "type": "integer",
                        "description": "Only return the last N lines of output."
                    }
                }
            }),
        },
        ToolInfo {
            name: "flake_lock",
            description: "Lock flake inputs without building. PREFER this tool over running `nix flake lock` directly - it provides validated inputs and proper error handling.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "flake_ref": {
                        "type": "string",
                        "description": "Flake reference. Defaults to '.'."
                    },
                    "update_inputs": {
                        "type": "array",
                        "items": { "type": "string" },
                        "description": "Inputs to update."
                    },
                    "override_inputs": {
                        "type": "object",
                        "additionalProperties": { "type": "string" },
                        "description": "Map of input names to flake references to override."
                    },
                    "flake_dir": {
                        "type": "string",
                        "description": "Directory containing the flake. Defaults to current directory."
                    },
                    "max_bytes": {
                        "type": "integer",
                        "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                    },
                    "head": {
                        "type": "integer",
                        "description": "Only return the first N lines of output."
                    },
                    "tail": {
                        "type": "integer",
                        "description": "Only return the last N lines of output."
                    }
                }
            }),
        },
        ToolInfo {
            name: "flake_init",
            description: "Initialize a new flake in the specified directory. PREFER this tool over running `nix flake init` directly - it provides validated inputs and proper error handling.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "template": {
                        "type": "string",
                        "description": "Template flake reference (e.g., 'templates#rust'). If not specified, uses default template."
                    },
                    "flake_dir": {
                        "type": "string",
                        "description": "Directory to initialize the flake in. Defaults to current directory."
                    }
                }
            }),
        },
        ToolInfo {
            name: "run",
            description: "Run a flake app. Agents MUST use this tool over running `nix run` directly - it provides validated inputs, secure argument handling, and proper process management.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "installable": {
                        "type": "string",
                        "description": "Flake installable to run. Defaults to '.#default'."
                    },
                    "args": {
                        "type": "array",
                        "items": { "type": "string" },
                        "description": "Arguments to pass to the app."
                    },
                    "flake_dir": {
                        "type": "string",
                        "description": "Directory containing the flake. Defaults to current directory."
                    }
                }
            }),
        },
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
                        "minItems": 1,
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
                    },
                    "max_bytes": {
                        "type": "integer",
                        "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                    },
                    "head": {
                        "type": "integer",
                        "description": "Only return the first N lines of output."
                    },
                    "tail": {
                        "type": "integer",
                        "description": "Only return the last N lines of output."
                    }
                },
                "required": ["commands"]
            }),
        },
        ToolInfo {
            name: "log",
            description: "Get build logs for a derivation. Agents MUST use this tool over running `nix log` directly - it provides validated inputs and optional head/tail functionality.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "installable": {
                        "type": "string",
                        "description": "Flake installable or store path."
                    },
                    "head": {
                        "type": "integer",
                        "description": "Only return the first N lines."
                    },
                    "tail": {
                        "type": "integer",
                        "description": "Only return the last N lines."
                    },
                    "max_bytes": {
                        "type": "integer",
                        "description": "Maximum bytes of log output to return. Defaults to config value (100KB)."
                    }
                },
                "required": ["installable"]
            }),
        },
        ToolInfo {
            name: "search",
            description: "Search for packages in a flake. PREFER this tool over running `nix search` directly - it provides validated inputs, structured JSON output, and pagination.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "Search query (regex pattern)."
                    },
                    "flake_ref": {
                        "type": "string",
                        "description": "Flake to search (e.g., 'nixpkgs'). Defaults to 'nixpkgs'."
                    },
                    "exclude": {
                        "type": "array",
                        "items": { "type": "string" },
                        "description": "Regex patterns to exclude from results."
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of results to return. Defaults to config value (50)."
                    },
                    "offset": {
                        "type": "integer",
                        "description": "Skip first N results for pagination. Defaults to 0."
                    }
                },
                "required": ["query"]
            }),
        },
        ToolInfo {
            name: "store_path_info",
            description: "Get information about a store path or installable. PREFER this tool over running `nix path-info` directly - it provides validated inputs, structured JSON output, and closure limiting.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Store path or flake installable to query."
                    },
                    "closure": {
                        "type": "boolean",
                        "description": "Include closure (all dependencies). Defaults to false."
                    },
                    "derivation": {
                        "type": "boolean",
                        "description": "Show derivation path instead of output path. Defaults to false."
                    },
                    "closure_limit": {
                        "type": "integer",
                        "description": "Maximum number of closure entries to return. Defaults to config value (100)."
                    },
                    "closure_offset": {
                        "type": "integer",
                        "description": "Skip first N closure entries for pagination. Defaults to 0."
                    }
                },
                "required": ["path"]
            }),
        },
        ToolInfo {
            name: "store_gc",
            description: "Run garbage collection on the Nix store. PREFER this tool over running `nix store gc` directly - it provides validated inputs and proper error handling.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "dry_run": {
                        "type": "boolean",
                        "description": "Only print what would be deleted. Defaults to false."
                    },
                    "max_freed": {
                        "type": "string",
                        "description": "Stop after freeing this much space (e.g., '1G', '500M')."
                    }
                }
            }),
        },
        ToolInfo {
            name: "store_ls",
            description: "List directory contents of a path that resolves into /nix/store/. Accepts ./result, ./result/bin, /nix/store/..., etc. Resolves symlinks and validates the canonical path is within the Nix store.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Path to list (e.g., './result', './result/bin', '/nix/store/...-name/bin'). Symlinks are resolved before validation."
                    },
                    "long": {
                        "type": "boolean",
                        "description": "Include file sizes for regular files. Defaults to false."
                    },
                    "offset": {
                        "type": "integer",
                        "description": "Skip first N entries for pagination. Defaults to 0."
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of entries to return. Defaults to all entries."
                    }
                },
                "required": ["path"]
            }),
        },
        ToolInfo {
            name: "store_cat",
            description: "Read file contents from a path that resolves into /nix/store/. Accepts ./result, /nix/store/..., etc. Supports line-based pagination with offset and limit. Resolves symlinks and validates the canonical path is within the Nix store.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Path to the file to read (e.g., './result/bin/hello', '/nix/store/...-name/etc/config'). Symlinks are resolved before validation."
                    },
                    "offset": {
                        "type": "integer",
                        "description": "Number of lines to skip from the beginning. Defaults to 0."
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of lines to return. Defaults to all lines."
                    }
                },
                "required": ["path"]
            }),
        },
        ToolInfo {
            name: "derivation_show",
            description: "Show the contents of a derivation. PREFER this tool over running `nix derivation show` directly - it provides validated inputs, structured JSON output, and summary mode for large dependency trees.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "installable": {
                        "type": "string",
                        "description": "Flake installable or store path. Defaults to '.#default'."
                    },
                    "recursive": {
                        "type": "boolean",
                        "description": "Include derivations of dependencies. Defaults to false."
                    },
                    "flake_dir": {
                        "type": "string",
                        "description": "Directory containing the flake. Defaults to current directory."
                    },
                    "summary_only": {
                        "type": "boolean",
                        "description": "Return only derivation summary (name, path, outputs, input count) instead of full content. Useful for exploring large dependency trees."
                    },
                    "max_inputs": {
                        "type": "integer",
                        "description": "Maximum number of input derivations to include. Defaults to config value (100)."
                    },
                    "inputs_offset": {
                        "type": "integer",
                        "description": "Skip first N input derivations for pagination. Defaults to 0."
                    }
                }
            }),
        },
        ToolInfo {
            name: "hash_path",
            description: "Compute the hash of a path (NAR serialization). PREFER this tool over running `nix hash path` directly - it provides validated inputs and structured output.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Path to hash."
                    },
                    "hash_type": {
                        "type": "string",
                        "description": "Hash algorithm (sha256, sha512, sha1, md5). Defaults to sha256."
                    },
                    "base32": {
                        "type": "boolean",
                        "description": "Output in base32 format. Defaults to false (SRI format)."
                    },
                    "sri": {
                        "type": "boolean",
                        "description": "Output in SRI format. Defaults to true."
                    }
                },
                "required": ["path"]
            }),
        },
        ToolInfo {
            name: "hash_file",
            description: "Compute the hash of a file. PREFER this tool over running `nix hash file` directly - it provides validated inputs and structured output.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "File path to hash."
                    },
                    "hash_type": {
                        "type": "string",
                        "description": "Hash algorithm (sha256, sha512, sha1, md5). Defaults to sha256."
                    },
                    "base32": {
                        "type": "boolean",
                        "description": "Output in base32 format. Defaults to false (SRI format)."
                    },
                    "sri": {
                        "type": "boolean",
                        "description": "Output in SRI format. Defaults to true."
                    }
                },
                "required": ["path"]
            }),
        },
        ToolInfo {
            name: "copy",
            description: "Copy store paths between Nix stores. PREFER this tool over running `nix copy` directly - it provides validated inputs and proper error handling.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "installable": {
                        "type": "string",
                        "description": "Store path or flake installable to copy."
                    },
                    "to": {
                        "type": "string",
                        "description": "Destination store URI (e.g., 's3://bucket', 'ssh://host')."
                    },
                    "from": {
                        "type": "string",
                        "description": "Source store URI."
                    }
                },
                "required": ["installable"]
            }),
        },
        ToolInfo {
            name: "eval",
            description: "Evaluate a nix expression. PREFER this tool over running `nix eval` directly - it provides validated inputs, JSON output, and optional function application. The `expr` and `apply` parameters accept full Nix syntax including attribute sets ({ x = 1; }), string interpolation (${ }), let bindings, lambdas (x: x + 1), and all Nix operators. Shell metacharacters are safe here — expressions are passed directly to the nix process, not through a shell.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "installable": {
                        "type": "string",
                        "description": "Flake installable to evaluate (e.g., '.#packages.x86_64-linux')."
                    },
                    "expr": {
                        "type": "string",
                        "description": "Nix expression to evaluate (alternative to installable). Supports full Nix syntax: attribute sets ({ a = 1; }), string interpolation (\"hello ${name}\"), let bindings, lambdas, builtins, etc. All Nix operators and special characters are allowed."
                    },
                    "apply": {
                        "type": "string",
                        "description": "Nix function to apply to the result (e.g., 'builtins.attrNames', 'x: builtins.length (builtins.attrNames x)'). Supports full Nix syntax including lambdas and builtins."
                    },
                    "flake_dir": {
                        "type": "string",
                        "description": "Directory containing the flake. Defaults to current directory."
                    },
                    "max_bytes": {
                        "type": "integer",
                        "description": "Maximum bytes of output to return. Defaults to config value (100KB)."
                    },
                    "head": {
                        "type": "integer",
                        "description": "Only return the first N lines of output."
                    },
                    "tail": {
                        "type": "integer",
                        "description": "Only return the last N lines of output."
                    }
                }
            }),
        },
        ToolInfo {
            name: "fh_search",
            description: "Search FlakeHub for flakes matching a query. Agents MUST use this tool over running `fh search` directly - it provides structured JSON output.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "The search query."
                    },
                    "max_results": {
                        "type": "integer",
                        "description": "Maximum number of results to return from FlakeHub API. Defaults to 10."
                    },
                    "offset": {
                        "type": "integer",
                        "description": "Skip first N results for pagination. Defaults to 0."
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of results to return. Defaults to all."
                    }
                },
                "required": ["query"]
            }),
        },
        ToolInfo {
            name: "fh_add",
            description: "Add a flake input to your flake.nix from FlakeHub. Agents MUST use this tool over running `fh add` directly - it provides validated inputs and proper error handling.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "input_ref": {
                        "type": "string",
                        "description": "The flake reference to add (e.g., 'NixOS/nixpkgs' or 'NixOS/nixpkgs/0.2411.*')."
                    },
                    "flake_path": {
                        "type": "string",
                        "description": "Path to the flake.nix to modify. Defaults to './flake.nix'."
                    },
                    "input_name": {
                        "type": "string",
                        "description": "Name for the flake input. If not provided, inferred from the input URL."
                    }
                },
                "required": ["input_ref"]
            }),
        },
        ToolInfo {
            name: "fh_list_flakes",
            description: "List public flakes on FlakeHub. Agents MUST use this tool over running `fh list` directly - it provides structured JSON output.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of flakes to return."
                    },
                    "offset": {
                        "type": "integer",
                        "description": "Skip first N results for pagination. Defaults to 0."
                    }
                }
            }),
        },
        ToolInfo {
            name: "fh_list_releases",
            description: "List all releases for a specific flake on FlakeHub. Agents MUST use this tool over running `fh list releases` directly - it provides structured JSON output.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "flake": {
                        "type": "string",
                        "description": "The flake to list releases for (e.g., 'NixOS/nixpkgs')."
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of releases to return."
                    },
                    "offset": {
                        "type": "integer",
                        "description": "Skip first N results for pagination. Defaults to 0."
                    }
                },
                "required": ["flake"]
            }),
        },
        ToolInfo {
            name: "fh_list_versions",
            description: "List versions matching a constraint for a flake on FlakeHub. Agents MUST use this tool over running `fh list versions` directly - it provides structured JSON output.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "flake": {
                        "type": "string",
                        "description": "The flake to list versions for (e.g., 'NixOS/nixpkgs')."
                    },
                    "version_constraint": {
                        "type": "string",
                        "description": "Version constraint (e.g., '0.2411.*', '>=0.2405')."
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of versions to return."
                    },
                    "offset": {
                        "type": "integer",
                        "description": "Skip first N results for pagination. Defaults to 0."
                    }
                },
                "required": ["flake", "version_constraint"]
            }),
        },
        ToolInfo {
            name: "fh_resolve",
            description: "Resolve a FlakeHub flake reference to a store path. Agents MUST use this tool over running `fh resolve` directly - it provides validated inputs and structured output.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "flake_ref": {
                        "type": "string",
                        "description": "FlakeHub flake reference (e.g., 'NixOS/nixpkgs/0.2411.*#hello')."
                    }
                },
                "required": ["flake_ref"]
            }),
        },
        // Cachix tools
        ToolInfo {
            name: "cachix_push",
            description: "Push store paths to a Cachix binary cache. Requires CACHIX_AUTH_TOKEN env var or config in ~/.config/nix-mcp-server/config.toml.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "cache_name": {
                        "type": "string",
                        "description": "Cachix cache name. Uses default from config if not specified."
                    },
                    "store_paths": {
                        "type": "array",
                        "items": { "type": "string" },
                        "description": "Nix store paths to push (e.g., '/nix/store/...-hello')."
                    }
                },
                "required": ["store_paths"]
            }),
        },
        ToolInfo {
            name: "cachix_use",
            description: "Configure Nix to use a Cachix binary cache as a substituter.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "cache_name": {
                        "type": "string",
                        "description": "Cachix cache name to add as substituter."
                    }
                },
                "required": ["cache_name"]
            }),
        },
        ToolInfo {
            name: "cachix_status",
            description: "Check Cachix authentication status.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {}
            }),
        },
        // FlakeHub cache tools
        ToolInfo {
            name: "fh_status",
            description: "Check FlakeHub login and cache status.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {}
            }),
        },
        ToolInfo {
            name: "fh_fetch",
            description: "Fetch a flake output from FlakeHub cache and create a GC root symlink.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "flake_ref": {
                        "type": "string",
                        "description": "FlakeHub flake reference (e.g., 'NixOS/nixpkgs/0.2411.*#hello')."
                    },
                    "target_link": {
                        "type": "string",
                        "description": "Path for the symlink (GC root)."
                    }
                },
                "required": ["flake_ref", "target_link"]
            }),
        },
        ToolInfo {
            name: "fh_login",
            description: "Initiate FlakeHub OAuth login flow. Opens browser for authentication.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "token_file": {
                        "type": "string",
                        "description": "Optional path to token file for non-interactive auth."
                    }
                }
            }),
        },
        // Background task tools
        ToolInfo {
            name: "task_status",
            description: "Check status of background tasks. If no task_id provided, lists all tasks.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "task_id": {
                        "type": "string",
                        "description": "Task ID to check. If omitted, returns all tasks."
                    }
                }
            }),
        },
        // nil LSP tools
        ToolInfo {
            name: "nil_diagnostics",
            description: "Get Nix language diagnostics (errors, warnings, undefined names) for a file using the nil language server.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "file_path": {
                        "type": "string",
                        "description": "Absolute path to the .nix file to analyze."
                    },
                    "offset": {
                        "type": "integer",
                        "description": "Skip first N diagnostics for pagination. Defaults to 0."
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of diagnostics to return. Defaults to all."
                    }
                },
                "required": ["file_path"]
            }),
        },
        ToolInfo {
            name: "nil_completions",
            description: "Get Nix code completions at a specific position using the nil language server.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "file_path": {
                        "type": "string",
                        "description": "Absolute path to the .nix file."
                    },
                    "line": {
                        "type": "integer",
                        "description": "0-indexed line number."
                    },
                    "character": {
                        "type": "integer",
                        "description": "0-indexed character offset within the line."
                    },
                    "offset": {
                        "type": "integer",
                        "description": "Skip first N completions for pagination. Defaults to 0."
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of completions to return. Defaults to all."
                    }
                },
                "required": ["file_path", "line", "character"]
            }),
        },
        ToolInfo {
            name: "nil_hover",
            description: "Get hover information (documentation, type info) at a specific position using the nil language server.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "file_path": {
                        "type": "string",
                        "description": "Absolute path to the .nix file."
                    },
                    "line": {
                        "type": "integer",
                        "description": "0-indexed line number."
                    },
                    "character": {
                        "type": "integer",
                        "description": "0-indexed character offset within the line."
                    }
                },
                "required": ["file_path", "line", "character"]
            }),
        },
        ToolInfo {
            name: "nil_definition",
            description: "Go to definition for a symbol at a specific position using the nil language server.",
            input_schema: serde_json::json!({
                "type": "object",
                "properties": {
                    "file_path": {
                        "type": "string",
                        "description": "Absolute path to the .nix file."
                    },
                    "line": {
                        "type": "integer",
                        "description": "0-indexed line number."
                    },
                    "character": {
                        "type": "integer",
                        "description": "0-indexed character offset within the line."
                    }
                },
                "required": ["file_path", "line", "character"]
            }),
        },
    ]
}

#[derive(Debug, Deserialize, Default)]
pub struct NixBuildParams {
    pub installable: Option<String>,
    pub print_build_logs: Option<bool>,
    pub flake_dir: Option<String>,
    pub max_log_bytes: Option<usize>,
    pub log_tail: Option<usize>,
}

#[derive(Debug, Deserialize, Default)]
pub struct NixFlakeShowParams {
    pub flake_ref: Option<String>,
    pub all_systems: Option<bool>,
    pub flake_dir: Option<String>,
    pub max_bytes: Option<usize>,
    pub head: Option<usize>,
    pub tail: Option<usize>,
}

#[derive(Debug, Deserialize, Default)]
pub struct NixFlakeCheckParams {
    pub flake_ref: Option<String>,
    pub keep_going: Option<bool>,
    pub flake_dir: Option<String>,
    pub max_bytes: Option<usize>,
    pub head: Option<usize>,
    pub tail: Option<usize>,
}

#[derive(Debug, Deserialize, Default)]
pub struct NixFlakeMetadataParams {
    pub flake_ref: Option<String>,
    pub flake_dir: Option<String>,
    pub max_bytes: Option<usize>,
    pub head: Option<usize>,
    pub tail: Option<usize>,
}

#[derive(Debug, Deserialize, Default)]
pub struct NixFlakeUpdateParams {
    pub flake_ref: Option<String>,
    pub inputs: Option<Vec<String>>,
    pub flake_dir: Option<String>,
    pub max_bytes: Option<usize>,
    pub head: Option<usize>,
    pub tail: Option<usize>,
}

#[derive(Debug, Deserialize, Default)]
pub struct NixFlakeLockParams {
    pub flake_ref: Option<String>,
    pub update_inputs: Option<Vec<String>>,
    pub override_inputs: Option<std::collections::HashMap<String, String>>,
    pub flake_dir: Option<String>,
    pub max_bytes: Option<usize>,
    pub head: Option<usize>,
    pub tail: Option<usize>,
}

#[derive(Debug, Deserialize, Default)]
pub struct NixFlakeInitParams {
    pub template: Option<String>,
    pub flake_dir: Option<String>,
}

#[derive(Debug, Deserialize, Default)]
pub struct NixRunParams {
    pub installable: Option<String>,
    pub args: Option<Vec<String>>,
    pub flake_dir: Option<String>,
}

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
    pub max_bytes: Option<usize>,
    pub head: Option<usize>,
    pub tail: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct NixLogParams {
    pub installable: String,
    pub head: Option<usize>,
    pub tail: Option<usize>,
    pub max_bytes: Option<usize>,
}

#[derive(Debug, Deserialize, Default)]
pub struct NixEvalParams {
    pub installable: Option<String>,
    pub expr: Option<String>,
    pub apply: Option<String>,
    pub flake_dir: Option<String>,
    pub max_bytes: Option<usize>,
    pub head: Option<usize>,
    pub tail: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct NixSearchParams {
    pub query: String,
    pub flake_ref: Option<String>,
    pub exclude: Option<Vec<String>>,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct NixStorePathInfoParams {
    pub path: String,
    pub closure: Option<bool>,
    pub derivation: Option<bool>,
    pub closure_limit: Option<usize>,
    pub closure_offset: Option<usize>,
}

#[derive(Debug, Deserialize, Default)]
pub struct NixStoreGcParams {
    pub dry_run: Option<bool>,
    pub max_freed: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct NixStoreLsParams {
    pub path: String,
    pub long: Option<bool>,
    pub offset: Option<usize>,
    pub limit: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct NixStoreCatParams {
    pub path: String,
    pub offset: Option<usize>,
    pub limit: Option<usize>,
}

#[derive(Debug, Deserialize, Default)]
pub struct NixDerivationShowParams {
    pub installable: Option<String>,
    pub recursive: Option<bool>,
    pub flake_dir: Option<String>,
    pub summary_only: Option<bool>,
    pub max_inputs: Option<usize>,
    pub inputs_offset: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct NixHashPathParams {
    pub path: String,
    pub hash_type: Option<String>,
    pub base32: Option<bool>,
    pub sri: Option<bool>,
}

#[derive(Debug, Deserialize)]
pub struct NixHashFileParams {
    pub path: String,
    pub hash_type: Option<String>,
    pub base32: Option<bool>,
    pub sri: Option<bool>,
}

#[derive(Debug, Deserialize)]
pub struct NixCopyParams {
    pub installable: String,
    pub to: Option<String>,
    pub from: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct FhSearchParams {
    pub query: String,
    pub max_results: Option<usize>,
    pub offset: Option<usize>,
    pub limit: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct FhAddParams {
    pub input_ref: String,
    pub flake_path: Option<String>,
    pub input_name: Option<String>,
}

#[derive(Debug, Deserialize, Default)]
pub struct FhListFlakesParams {
    pub limit: Option<usize>,
    pub offset: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct FhListReleasesParams {
    pub flake: String,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct FhListVersionsParams {
    pub flake: String,
    pub version_constraint: String,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct FhResolveParams {
    pub flake_ref: String,
}

// Cachix params
#[derive(Debug, Deserialize, Default)]
pub struct CachixPushParams {
    pub cache_name: Option<String>,
    pub store_paths: Vec<String>,
}

#[derive(Debug, Deserialize)]
pub struct CachixUseParams {
    pub cache_name: String,
}

#[derive(Debug, Deserialize, Default)]
pub struct CachixStatusParams {}

// FlakeHub cache params
#[derive(Debug, Deserialize, Default)]
pub struct FhStatusParams {}

#[derive(Debug, Deserialize)]
pub struct FhFetchParams {
    pub flake_ref: String,
    pub target_link: String,
}

#[derive(Debug, Deserialize, Default)]
pub struct FhLoginParams {
    pub token_file: Option<String>,
}

// Background task params
#[derive(Debug, Deserialize, Default)]
pub struct TaskStatusParams {
    pub task_id: Option<String>,
}

// nil LSP params
#[derive(Debug, Deserialize)]
pub struct NilDiagnosticsParams {
    pub file_path: String,
    pub offset: Option<usize>,
    pub limit: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct NilCompletionsParams {
    pub file_path: String,
    pub line: u32,
    pub character: u32,
    pub offset: Option<usize>,
    pub limit: Option<usize>,
}

#[derive(Debug, Deserialize)]
pub struct NilHoverParams {
    pub file_path: String,
    pub line: u32,
    pub character: u32,
}

#[derive(Debug, Deserialize)]
pub struct NilDefinitionParams {
    pub file_path: String,
    pub line: u32,
    pub character: u32,
}
