#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  purse_first="$(purse_first_bin)"

  # Create a temp project dir with local lux mappings
  project_dir="$(mktemp -d "$BATS_TEST_TMPDIR/project-XXXXXX")"
  mkdir -p "$project_dir/.purse-first"
  cat >"$project_dir/.purse-first/lux.json" <<'MAPPING'
{
  "server": "lux",
  "mappings": [
    {
      "replaces": "Read",
      "extensions": [".go", ".py"],
      "tools": [
        {"name": "hover", "use_when": "getting type info"},
        {"name": "document_symbols", "use_when": "understanding file structure"}
      ],
      "reason": "Use lux MCP tools instead"
    },
    {
      "replaces": "Grep",
      "extensions": [".go"],
      "tools": [
        {"name": "workspace_symbols", "use_when": "finding symbols by name"}
      ],
      "reason": "Use lux MCP tools for semantic search"
    }
  ]
}
MAPPING
}

function deny_read_go_suggests_correct_plugin_tool_names { # @test
  # Issue #2: format should be mcp__plugin_{plugin}_{server}__{tool}
  # Issue #3: tool names should not have lsp_ prefix
  local payload
  payload=$(hook_payload Read file_path=/path/to/foo.go)

  run sh -c 'cd "$3" && echo "$1" | "$2" hook' -- "$payload" "$purse_first" "$project_dir"
  assert_success

  # Must contain the plugin-prefixed tool name format
  assert_output --partial "mcp__plugin_lux_lux__"

  # Must NOT contain old-style server-only format
  refute_output --partial "mcp__lux__lsp_"
}

function deny_git_suggests_correct_plugin_tool_names { # @test
  # Issue #2: grit tools should also use plugin prefix
  local payload
  payload=$(hook_payload Bash command="git status")

  run sh -c 'echo "$1" | "$2" hook' -- "$payload" "$purse_first"
  assert_success

  assert_output --partial "mcp__plugin_grit_grit__"
  refute_output --partial "mcp__grit__status"
}

function passthrough_non_matching_extension { # @test
  local payload
  payload=$(hook_payload Read file_path=/path/to/readme.md)

  run sh -c 'cd "$3" && echo "$1" | "$2" hook' -- "$payload" "$purse_first" "$project_dir"
  assert_success
  assert_output ""
}

function passthrough_no_file_path { # @test
  run sh -c 'cd "$2" && echo "{\"session_id\":\"test\",\"tool_name\":\"Read\",\"tool_input\":{},\"hook_event_name\":\"PreToolUse\"}" | "$1" hook' -- "$purse_first" "$project_dir"
  assert_success
  assert_output ""
}

function deny_output_valid_hook_json { # @test
  local payload
  payload=$(hook_payload Read file_path=/path/to/foo.go)

  run sh -c 'cd "$3" && echo "$1" | "$2" hook' -- "$payload" "$purse_first" "$project_dir"
  assert_success

  # Must be valid JSON with expected structure
  echo "$output" | jq -e '.hookSpecificOutput.permissionDecision == "deny"'
  echo "$output" | jq -e '.hookSpecificOutput.hookEventName == "PreToolUse"'
  echo "$output" | jq -e '.hookSpecificOutput.permissionDecisionReason | length > 0'
}

function grep_directory_path_with_type_filter { # @test
  skip "TODO: Grep with directory path + type filter should deny (issue #8)"

  local payload='{"session_id":"test","tool_name":"Grep","tool_input":{"pattern":"HandlePreToolUse","path":"/some/directory","type":"go"},"hook_event_name":"PreToolUse"}'

  run sh -c 'echo "$1" | "$2" hook' -- "$payload" "$purse_first"
  assert_success
  assert_output --partial "deny"
}

function glob_directory_path_pattern { # @test
  skip "TODO: Glob with directory path + *.go pattern should deny (issue #8)"

  local payload='{"session_id":"test","tool_name":"Glob","tool_input":{"pattern":"**/*.go","path":"/some/directory"},"hook_event_name":"PreToolUse"}'

  run sh -c 'echo "$1" | "$2" hook' -- "$payload" "$purse_first"
  assert_success
  assert_output --partial "deny"
}

function deny_read_go_with_tool_prefix_uses_prefix { # @test
  # Create a mapping with tool_prefix set (dev mode)
  cat >"$project_dir/.purse-first/lux.json" <<'MAPPING'
{
  "server": "lux",
  "tool_prefix": "mcp__lux-dev",
  "mappings": [
    {
      "replaces": "Read",
      "extensions": [".go"],
      "tools": [
        {"name": "hover", "use_when": "getting type info"}
      ],
      "reason": "Use lux-dev MCP tools"
    }
  ]
}
MAPPING

  local payload
  payload=$(hook_payload Read file_path=/path/to/foo.go)

  run sh -c 'cd "$3" && echo "$1" | "$2" hook' -- "$payload" "$purse_first" "$project_dir"
  assert_success
  assert_output --partial "mcp__lux-dev__hover"
  refute_output --partial "mcp__plugin_lux_lux__"
}

function deny_git_status_with_dev_mcp_prefix { # @test
  # Simulate dev-mcp: grit mappings with tool_prefix set
  cat >"$project_dir/.purse-first/grit.json" <<'MAPPING'
{
  "server": "grit",
  "tool_prefix": "mcp__grit-dev",
  "mappings": [
    {
      "replaces": "Bash",
      "command_prefixes": ["git status"],
      "tools": [
        {"name": "status", "use_when": "checking repository status"}
      ],
      "reason": "Use the grit MCP tool instead"
    }
  ]
}
MAPPING

  local payload
  payload=$(hook_payload Bash command="git status")

  run sh -c 'cd "$3" && echo "$1" | "$2" hook' -- "$payload" "$purse_first" "$project_dir"
  assert_success

  assert_output --partial "mcp__grit-dev__status"
  refute_output --partial "mcp__plugin_grit_grit__"

  echo "$output" | jq -e '.hookSpecificOutput.permissionDecision == "deny"'
}

function deny_git_log_with_dev_mcp_prefix { # @test
  cat >"$project_dir/.purse-first/grit.json" <<'MAPPING'
{
  "server": "grit",
  "tool_prefix": "mcp__grit-dev",
  "mappings": [
    {
      "replaces": "Bash",
      "command_prefixes": ["git log"],
      "tools": [
        {"name": "log", "use_when": "viewing commit history"}
      ],
      "reason": "Use the grit MCP tool instead"
    }
  ]
}
MAPPING

  local payload
  payload=$(hook_payload Bash command="git log --oneline -10")

  run sh -c 'cd "$3" && echo "$1" | "$2" hook' -- "$payload" "$purse_first" "$project_dir"
  assert_success

  assert_output --partial "mcp__grit-dev__log"
  refute_output --partial "mcp__plugin_grit_grit__"

  echo "$output" | jq -e '.hookSpecificOutput.permissionDecision == "deny"'
}

function deny_git_show_with_dev_mcp_prefix { # @test
  cat >"$project_dir/.purse-first/grit.json" <<'MAPPING'
{
  "server": "grit",
  "tool_prefix": "mcp__grit-dev",
  "mappings": [
    {
      "replaces": "Bash",
      "command_prefixes": ["git show"],
      "tools": [
        {"name": "show", "use_when": "inspecting commits or objects"}
      ],
      "reason": "Use the grit MCP tool instead"
    }
  ]
}
MAPPING

  local payload
  payload=$(hook_payload Bash command="git show HEAD")

  run sh -c 'cd "$3" && echo "$1" | "$2" hook' -- "$payload" "$purse_first" "$project_dir"
  assert_success

  assert_output --partial "mcp__grit-dev__show"
  refute_output --partial "mcp__plugin_grit_grit__"

  echo "$output" | jq -e '.hookSpecificOutput.permissionDecision == "deny"'
}

function post_hook_reads_stdin_cleanly { # @test
  local payload
  payload=$(hook_payload Read file_path=/path/to/foo.go)

  run sh -c 'cd "$3" && echo "$1" | "$2" post-hook' -- "$payload" "$purse_first" "$project_dir"
  assert_success
}

function session_end_exits_cleanly { # @test
  run sh -c 'echo "{}" | "$1" session-end' -- "$purse_first"
  assert_success
}
