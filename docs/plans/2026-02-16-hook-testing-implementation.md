# Hook Testing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Write failing tests for all 10 hook issues, with TODO/skip markers for design-gap issues.

**Architecture:** Extend existing Go test files for unit-level validation. Add two new BATS files (hook_io.bats, hook_lifecycle.bats) for integration testing. A sandcastle wrapper isolates lifecycle tests from the user's real `~/.claude/`.

**Tech Stack:** Go testing stdlib, BATS + bats-assert + bats-support, sandcastle, jq, justfile

---

### Task 1: Go unit test — formatDenyReason uses plugin prefix (issues #2, #3)

**Files:**
- Modify: `internal/hook/handler_test.go` (append after line 296)

**Step 1: Write the failing test**

Append to `internal/hook/handler_test.go`:

```go
func TestFormatDenyReasonUsesPluginPrefix(t *testing.T) {
	m := mapping.Mapping{
		Tools: []mapping.ToolSuggestion{
			{Name: "hover", UseWhen: "getting type info"},
			{Name: "document_symbols", UseWhen: "understanding file structure"},
		},
		Reason: "Use lux MCP tools instead",
	}

	reason := formatDenyReason("lux", m)

	if !strings.Contains(reason, "mcp__plugin_lux_lux__hover") {
		t.Errorf("expected plugin-prefixed tool name mcp__plugin_lux_lux__hover, got:\n%s", reason)
	}

	if !strings.Contains(reason, "mcp__plugin_lux_lux__document_symbols") {
		t.Errorf("expected plugin-prefixed tool name mcp__plugin_lux_lux__document_symbols, got:\n%s", reason)
	}

	if strings.Contains(reason, "mcp__lux__") {
		t.Errorf("should not contain old-style mcp__lux__ prefix, got:\n%s", reason)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestFormatDenyReasonUsesPluginPrefix ./internal/hook/...`

Expected: FAIL — the format string `mcp__%s__%s` produces `mcp__lux__hover`, not `mcp__plugin_lux_lux__hover`.

**Step 3: Commit the failing test**

```
git add internal/hook/handler_test.go
git commit -m "test: add failing test for plugin-prefixed MCP tool names in deny reason

Validates issue #2 (wrong format) and #3 (lsp_ prefix data bug).
formatDenyReason currently produces mcp__server__tool but should
produce mcp__plugin_plugin_server__tool.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: Go unit test — Install sets blocking:true for PreToolUse (issue #5)

**Files:**
- Modify: `internal/hook/install_test.go` (new file in hook package, not install package)

Note: `install_test.go` already exists in `internal/install/`. The hook `Install` function lives in `internal/hook/install.go`, so its tests go in `internal/hook/`.

**Step 1: Write the failing test**

Create `internal/hook/install_hooks_test.go`:

```go
package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSetsBlockingTrueForPreToolUse(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	settingsDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Install("/usr/bin/purse-first", false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	preToolUse, _ := hooks["PreToolUse"].([]any)
	if len(preToolUse) == 0 {
		t.Fatal("expected PreToolUse hook entries, got none")
	}

	entry, _ := preToolUse[0].(map[string]any)
	hooksList, _ := entry["hooks"].([]any)
	if len(hooksList) == 0 {
		t.Fatal("expected hook entries, got none")
	}

	hookEntry, _ := hooksList[0].(map[string]any)

	blocking, ok := hookEntry["blocking"]
	if !ok {
		t.Fatal("expected blocking field to be present in PreToolUse hook")
	}

	if blocking != true {
		t.Errorf("expected PreToolUse blocking=true, got %v", blocking)
	}
}

func TestInstallSetsBlockingFalseForPostToolUse(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	settingsDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Install("/usr/bin/purse-first", false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	postToolUse, _ := hooks["PostToolUse"].([]any)
	if len(postToolUse) == 0 {
		t.Fatal("expected PostToolUse hook entries, got none")
	}

	entry, _ := postToolUse[0].(map[string]any)
	hooksList, _ := entry["hooks"].([]any)
	if len(hooksList) == 0 {
		t.Fatal("expected hook entries, got none")
	}

	hookEntry, _ := hooksList[0].(map[string]any)

	blocking, ok := hookEntry["blocking"]
	if !ok {
		t.Fatal("expected blocking field to be present in PostToolUse hook")
	}

	if blocking != false {
		t.Errorf("expected PostToolUse blocking=false, got %v", blocking)
	}
}
```

**Step 2: Run test to verify the PreToolUse test fails**

Run: `nix develop --command go test -v -run TestInstallSetsBlocking ./internal/hook/...`

Expected: `TestInstallSetsBlockingTrueForPreToolUse` FAILS — Install sets `blocking: false`.
Expected: `TestInstallSetsBlockingFalseForPostToolUse` PASSES.

**Step 3: Commit**

```
git add internal/hook/install_hooks_test.go
git commit -m "test: add failing test for PreToolUse hook blocking field

Install currently sets blocking:false on PreToolUse, which prevents
deny decisions from taking effect. Test asserts blocking:true.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: Go unit tests — TODO/skip tests for Grep/Glob directory path (issue #8) and Read granularity (issue #9)

**Files:**
- Modify: `internal/hook/handler_test.go` (append after Task 1's addition)

**Step 1: Write the skip-marked tests**

Append to `internal/hook/handler_test.go`:

```go
func TestHandlerGrepDirectoryPathWithTypeFilter(t *testing.T) {
	t.Skip("TODO: Grep with directory path and type filter should deny (issue #8)")

	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID: "test",
		ToolName:  "Grep",
		ToolInput: map[string]any{
			"pattern": "HandlePreToolUse",
			"path":    "/some/directory",
			"type":    "go",
		},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() == 0 {
		t.Error("expected deny for Grep on directory with type=go, got passthrough")
	}
}

func TestHandlerGlobDirectoryPathDenies(t *testing.T) {
	t.Skip("TODO: Glob with directory path and *.go pattern should deny (issue #8)")

	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Add Glob mapping to lux
	mf := mapping.MappingFile{
		Server: "lux",
		Mappings: []mapping.Mapping{
			{
				Replaces:   "Glob",
				Extensions: []string{".go"},
				Tools: []mapping.ToolSuggestion{
					{Name: "workspace_symbols", UseWhen: "finding symbols by name"},
				},
				Reason: "Use lux MCP tools for semantic search",
			},
		},
	}

	mappingDir := filepath.Join(projectDir, ".purse-first")
	if err := os.MkdirAll(mappingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(mf)
	if err := os.WriteFile(filepath.Join(mappingDir, "lux.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	input := decision.HookInput{
		SessionID: "test",
		ToolName:  "Glob",
		ToolInput: map[string]any{
			"pattern": "**/*.go",
			"path":    "/some/directory",
		},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() == 0 {
		t.Error("expected deny for Glob **/*.go with directory path, got passthrough")
	}
}

func TestHandlerReadDenialGranularity(t *testing.T) {
	t.Skip("TODO: design decision needed — Read on .go denies even for simple file reading (issue #9)")
}
```

**Step 2: Run tests to verify they're skipped**

Run: `nix develop --command go test -v -run 'TestHandler(GrepDirectory|GlobDirectory|ReadDenialGranularity)' ./internal/hook/...`

Expected: All 3 show `--- SKIP`.

**Step 3: Commit**

```
git add internal/hook/handler_test.go
git commit -m "test: add TODO-skipped tests for Grep/Glob dir path bypass and Read granularity

Documents issues #8 (directory path bypasses extension matching) and
#9 (Read denial too aggressive). Skipped pending design decisions.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Go unit test — TODO/skip for FindMatch glob param (issue #8)

**Files:**
- Modify: `internal/mapping/loader_test.go` (append after line 399)

**Step 1: Write the skip-marked test**

Append to `internal/mapping/loader_test.go`:

```go
func TestFindMatchChecksGlobParam(t *testing.T) {
	t.Skip("TODO: FindMatch should check glob/type params when path is a directory (issue #8)")

	files := []MappingFile{
		{
			Server: "lux",
			Mappings: []Mapping{
				{
					Replaces:   "Grep",
					Extensions: []string{".go"},
					Tools: []ToolSuggestion{
						{Name: "references", UseWhen: "finding usages"},
					},
					Reason: "Use lux",
				},
			},
		},
	}

	// path is a directory, but type/glob indicates Go files
	match := FindMatch(files, "Grep", "/some/directory", "")
	if match == nil {
		t.Error("expected match when path is directory but file type is known from context")
	}
}
```

**Step 2: Run to verify skip**

Run: `nix develop --command go test -v -run TestFindMatchChecksGlobParam ./internal/mapping/...`

Expected: `--- SKIP`.

**Step 3: Commit**

```
git add internal/mapping/loader_test.go
git commit -m "test: add TODO-skipped test for FindMatch with glob/type params

Documents that FindMatch only checks path extension, not the type
or glob parameters that indicate which files are being searched.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: BATS common helper — add hook_payload helper function

**Files:**
- Modify: `zz-tests_bats/common.bash` (append helper)

**Step 1: Add the helper**

Append to `zz-tests_bats/common.bash`:

```bash

hook_payload() {
  local tool_name="$1"
  shift
  # Remaining args are key=value pairs for tool_input
  local tool_input="{"
  local first=true
  for kv in "$@"; do
    local key="${kv%%=*}"
    local val="${kv#*=}"
    if [ "$first" = true ]; then
      first=false
    else
      tool_input+=","
    fi
    tool_input+="\"$key\":\"$val\""
  done
  tool_input+="}"

  cat <<EOF
{"session_id":"bats-test","tool_name":"$tool_name","tool_input":$tool_input,"hook_event_name":"PreToolUse"}
EOF
}
```

**Step 2: Verify helper works by checking it's valid JSON**

Run: `nix develop --command bash -c 'source zz-tests_bats/common.bash 2>/dev/null; hook_payload Read file_path=/path/to/foo.go | jq .'`

This may fail due to BATS not being loaded. Instead, just verify it by eye and test it through the BATS tests in the next task.

**Step 3: Commit**

```
git add zz-tests_bats/common.bash
git commit -m "test: add hook_payload helper to BATS common.bash

Builds PreToolUse JSON payloads for hook I/O tests from simple
key=value arguments.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 6: BATS hook_io.bats — hook I/O simulation tests

**Files:**
- Create: `zz-tests_bats/hook_io.bats`

**Step 1: Write the BATS test file**

Create `zz-tests_bats/hook_io.bats`:

```bash
#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  purse_first="$(purse_first_bin)"
}

function deny_read_go_suggests_correct_plugin_tool_names { # @test
  # Issue #2: format should be mcp__plugin_{plugin}_{server}__{tool}
  # Issue #3: tool names should not have lsp_ prefix
  local payload
  payload=$(hook_payload Read file_path=/path/to/foo.go)

  run sh -c 'echo "$1" | "$2" hook' -- "$payload" "$purse_first"
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

  run sh -c 'echo "$1" | "$2" hook' -- "$payload" "$purse_first"
  assert_success
  assert_output ""
}

function passthrough_no_file_path { # @test
  run sh -c 'echo "{\"session_id\":\"test\",\"tool_name\":\"Read\",\"tool_input\":{},\"hook_event_name\":\"PreToolUse\"}" | "$1" hook' -- "$purse_first"
  assert_success
  assert_output ""
}

function deny_output_valid_hook_json { # @test
  local payload
  payload=$(hook_payload Read file_path=/path/to/foo.go)

  run sh -c 'echo "$1" | "$2" hook' -- "$payload" "$purse_first"
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

function post_hook_reads_stdin_cleanly { # @test
  local payload
  payload=$(hook_payload Read file_path=/path/to/foo.go)

  run sh -c 'echo "$1" | "$2" post-hook' -- "$payload" "$purse_first"
  assert_success
}

function session_end_exits_cleanly { # @test
  run sh -c 'echo "{}" | "$1" session-end' -- "$purse_first"
  assert_success
}
```

**Step 2: Build and run to verify**

Run:
```
nix build
nix develop --command zz-tests_bats/bin/run-sandcastle-bats.bash \
  bats --tap zz-tests_bats/hook_io.bats
```

Expected:
- `deny_read_go_suggests_correct_plugin_tool_names` — FAIL (wrong format)
- `deny_git_suggests_correct_plugin_tool_names` — FAIL (wrong format)
- `passthrough_non_matching_extension` — PASS
- `passthrough_no_file_path` — PASS
- `deny_output_valid_hook_json` — PASS
- `grep_directory_path_with_type_filter` — SKIP
- `glob_directory_path_pattern` — SKIP
- `post_hook_reads_stdin_cleanly` — PASS
- `session_end_exits_cleanly` — PASS

**Step 3: Commit**

```
git add zz-tests_bats/hook_io.bats
git commit -m "test: add BATS hook I/O simulation tests

Exercises PreToolUse, PostToolUse, and Stop hooks by piping JSON
payloads to the binary. Includes failing tests for wrong MCP tool
name format (issues #2, #3) and TODO-skipped tests for directory
path bypass (issue #8).

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 7: Sandcastle lifecycle wrapper

**Files:**
- Create: `zz-tests_bats/bin/run-sandcastle-lifecycle.bash`

**Step 1: Write the wrapper**

Create `zz-tests_bats/bin/run-sandcastle-lifecycle.bash`:

```bash
#!/usr/bin/env bash
set -e

real_home="$HOME"
tmp_home="$(mktemp -d /tmp/purse-first-lifecycle-XXXXXX)"

srt_config="$(mktemp)"
trap 'rm -f "$srt_config"' EXIT

cat >"$srt_config" <<SETTINGS
{
  "filesystem": {
    "denyRead": [
      "$real_home/.claude",
      "$real_home/.ssh",
      "$real_home/.aws",
      "$real_home/.gnupg",
      "$real_home/.config",
      "$real_home/.password-store",
      "$real_home/.kube"
    ],
    "denyWrite": [],
    "allowWrite": [
      "/tmp"
    ]
  },
  "network": {
    "allowedDomains": [],
    "deniedDomains": []
  }
}
SETTINGS

HOME="$tmp_home" \
  PURSE_FIRST_REAL_HOME="$real_home" \
  exec sandcastle \
    --shell bash \
    --config "$srt_config" \
    "$@"
```

**Step 2: Make it executable**

Run: `chmod +x zz-tests_bats/bin/run-sandcastle-lifecycle.bash`

**Step 3: Commit**

```
git add zz-tests_bats/bin/run-sandcastle-lifecycle.bash
git commit -m "test: add sandcastle wrapper for lifecycle tests with isolated HOME

Creates a temp HOME and blocks read access to real ~/.claude/ to
prevent clobbering user settings during install tests. Sandcastle
does not support --env natively, so HOME is set in the parent env.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 8: BATS hook_lifecycle.bats — install + verify tests

**Files:**
- Create: `zz-tests_bats/hook_lifecycle.bats`

**Step 1: Write the BATS test file**

Create `zz-tests_bats/hook_lifecycle.bats`:

```bash
#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  purse_first="$(purse_first_bin)"
  result_path="${PURSE_FIRST_RESULT:-$BATS_CWD/result}"
}

function install_registers_marketplace_and_plugins { # @test
  run "$purse_first" install
  assert_success
  assert_output --partial "TAP version 14"
  assert_output --partial "add marketplace"
}

function installed_hooks_reference_valid_binary { # @test
  skip "TODO: hooks are not installed by purse-first install (issues #1, #10)"

  "$purse_first" install

  local settings="$HOME/.claude/settings.json"
  [[ -f "$settings" ]]

  # Extract hook binary path and verify it exists
  local hook_bin
  hook_bin=$(jq -r '.hooks.PreToolUse[0].hooks[0].command' "$settings" | awk '{print $1}')
  [[ -x "$hook_bin" ]]
}

function pretooluse_hook_is_blocking { # @test
  skip "TODO: hook.Install is dead code; PreToolUse should be blocking (issues #4, #5)"

  "$purse_first" install

  local settings="$HOME/.claude/settings.json"
  [[ -f "$settings" ]]

  local blocking
  blocking=$(jq -r '.hooks.PreToolUse[0].hooks[0].blocking' "$settings")
  [[ "$blocking" == "true" ]]
}

function marketplace_json_validates { # @test
  local marketplace
  marketplace="$(marketplace_result)"

  run "$purse_first" validate --type marketplace "$marketplace"
  assert_success
}

function chix_has_mappings { # @test
  skip "TODO: chix does not ship mappings.json yet (issue #7)"

  local mappings="$result_path/share/purse-first/chix/mappings.json"
  [[ -f "$mappings" ]]

  run jq -e '.server == "chix"' "$mappings"
  assert_success
}

function get_hubbed_has_mappings { # @test
  skip "TODO: get-hubbed does not ship mappings.json yet (issue #7)"

  local mappings="$result_path/share/purse-first/get-hubbed/mappings.json"
  [[ -f "$mappings" ]]

  run jq -e '.server == "get-hubbed"' "$mappings"
  assert_success
}
```

**Step 2: Build and run**

Run:
```
nix build
nix develop --command zz-tests_bats/bin/run-sandcastle-lifecycle.bash \
  bats --tap zz-tests_bats/hook_lifecycle.bats
```

Expected:
- `install_registers_marketplace_and_plugins` — PASS
- `installed_hooks_reference_valid_binary` — SKIP
- `pretooluse_hook_is_blocking` — SKIP
- `marketplace_json_validates` — PASS
- `chix_has_mappings` — SKIP
- `get_hubbed_has_mappings` — SKIP

**Step 3: Commit**

```
git add zz-tests_bats/hook_lifecycle.bats
git commit -m "test: add BATS lifecycle tests for install+verify with sandcastle isolation

Tests the full install flow and marketplace validation in an isolated
HOME. Includes TODO-skipped tests for hooks not in install (issues
#1, #4, #5, #10) and missing mappings (issue #7).

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 9: Justfile — add test-hooks and test-lifecycle targets

**Files:**
- Modify: `justfile` (add after the `test-validate` target, around line 77)

**Step 1: Add the targets**

Add to `justfile` after the `test-validate` target:

```makefile
# Run hook unit tests + BATS hook I/O tests
test-hooks:
    nix develop --command go test -v ./internal/hook/...
    nix build
    nix develop --command zz-tests_bats/bin/run-sandcastle-bats.bash \
      bats --tap zz-tests_bats/hook_io.bats

# Run lifecycle tests with sandcastle-isolated HOME
test-lifecycle:
    nix build
    nix develop --command zz-tests_bats/bin/run-sandcastle-lifecycle.bash \
      bats --tap zz-tests_bats/hook_lifecycle.bats
```

**Step 2: Update test-all to include new targets**

Change the `test-all` line from:
```
test-all: test test-go-mcp test-rust-mcp test-integration
```
to:
```
test-all: test test-go-mcp test-rust-mcp test-integration test-hooks test-lifecycle
```

**Step 3: Verify targets are listed**

Run: `just --list`

Expected: `test-hooks` and `test-lifecycle` appear.

**Step 4: Commit**

```
git add justfile
git commit -m "chore: add test-hooks and test-lifecycle justfile targets

test-hooks: Go unit tests + BATS hook I/O simulation
test-lifecycle: sandcastle-isolated install+verify tests
Both included in test-all.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 10: Run full test suite and verify expected results

**Step 1: Run all Go tests**

Run: `nix develop --command go test -v ./internal/hook/... ./internal/mapping/...`

Expected:
- `TestFormatDenyReasonUsesPluginPrefix` — FAIL
- `TestInstallSetsBlockingTrueForPreToolUse` — FAIL
- `TestInstallSetsBlockingFalseForPostToolUse` — PASS
- `TestHandlerGrepDirectoryPathWithTypeFilter` — SKIP
- `TestHandlerGlobDirectoryPathDenies` — SKIP
- `TestHandlerReadDenialGranularity` — SKIP
- `TestFindMatchChecksGlobParam` — SKIP
- All existing tests — PASS

**Step 2: Run BATS hook I/O tests**

Run: `just test-hooks`

Expected (from BATS output):
- `deny_read_go_suggests_correct_plugin_tool_names` — FAIL
- `deny_git_suggests_correct_plugin_tool_names` — FAIL
- `passthrough_non_matching_extension` — PASS
- `passthrough_no_file_path` — PASS
- `deny_output_valid_hook_json` — PASS
- `grep_directory_path_with_type_filter` — SKIP
- `glob_directory_path_pattern` — SKIP
- `post_hook_reads_stdin_cleanly` — PASS
- `session_end_exits_cleanly` — PASS

**Step 3: Run BATS lifecycle tests**

Run: `just test-lifecycle`

Expected:
- `install_registers_marketplace_and_plugins` — PASS
- `installed_hooks_reference_valid_binary` — SKIP
- `pretooluse_hook_is_blocking` — SKIP
- `marketplace_json_validates` — PASS
- `chix_has_mappings` — SKIP
- `get_hubbed_has_mappings` — SKIP

This confirms the test suite correctly identifies the known bugs while skipping design-gap items.
