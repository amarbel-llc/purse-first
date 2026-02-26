# Stop Hook Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `stop_hook` support to the sweatfile so `spinclass hooks` can block an agent from stopping when a command fails, with a once-per-session sentinel to avoid infinite loops.

**Architecture:** The `spinclass hooks` command becomes a router: it reads `hook_event_name` from stdin JSON and dispatches to either the existing PreToolUse boundary check or the new Stop hook logic. The Stop path loads the sweatfile to get the command, checks a sentinel file keyed by `session_id`, runs the command, and returns a block/approve decision.

**Tech Stack:** Go, TOML (sweatfile), JSON (Claude settings + hook I/O), `sh -c` for command execution.

---

### Task 1: Add `StopHook` field to sweatfile struct

**Files:**
- Modify: `packages/spinclass/internal/sweatfile/sweatfile.go:13-16`
- Test: `packages/spinclass/internal/sweatfile/sweatfile_test.go`

**Step 1: Write the failing tests**

Add to `sweatfile_test.go`:

```go
func TestParseStopHook(t *testing.T) {
	input := `stop_hook = "just test"`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.StopHook == nil || *sf.StopHook != "just test" {
		t.Errorf("stop_hook: got %v", sf.StopHook)
	}
}

func TestParseStopHookAbsent(t *testing.T) {
	sf, err := Parse([]byte(`git_excludes = [".claude/"]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.StopHook != nil {
		t.Errorf("expected nil stop_hook, got %v", sf.StopHook)
	}
}

func TestMergeStopHookInherit(t *testing.T) {
	cmd := "just test"
	base := Sweatfile{StopHook: &cmd}
	repo := Sweatfile{}
	merged := Merge(base, repo)
	if merged.StopHook == nil || *merged.StopHook != "just test" {
		t.Errorf("expected inherited stop_hook, got %v", merged.StopHook)
	}
}

func TestMergeStopHookOverride(t *testing.T) {
	base_cmd := "just test"
	repo_cmd := "just lint"
	base := Sweatfile{StopHook: &base_cmd}
	repo := Sweatfile{StopHook: &repo_cmd}
	merged := Merge(base, repo)
	if merged.StopHook == nil || *merged.StopHook != "just lint" {
		t.Errorf("expected overridden stop_hook, got %v", merged.StopHook)
	}
}

func TestMergeStopHookClear(t *testing.T) {
	base_cmd := "just test"
	empty := ""
	base := Sweatfile{StopHook: &base_cmd}
	repo := Sweatfile{StopHook: &empty}
	merged := Merge(base, repo)
	if merged.StopHook == nil || *merged.StopHook != "" {
		t.Errorf("expected cleared stop_hook, got %v", merged.StopHook)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop ../.. --command go test ./packages/spinclass/internal/sweatfile/ -run 'TestParseStopHook|TestMergeStopHook' -v`
Expected: FAIL — `StopHook` field does not exist.

**Step 3: Implement StopHook field and merge logic**

In `sweatfile.go`, change the struct to:

```go
type Sweatfile struct {
	GitExcludes []string `toml:"git_excludes"`
	ClaudeAllow []string `toml:"claude_allow"`
	StopHook    *string  `toml:"stop_hook"`
}
```

In `Merge()`, add after the `ClaudeAllow` block:

```go
if repo.StopHook != nil {
	merged.StopHook = repo.StopHook
}
```

**Step 4: Run tests to verify they pass**

Run: `nix develop ../.. --command go test ./packages/spinclass/internal/sweatfile/ -v`
Expected: All PASS.

**Step 5: Commit**

```
feat(sweatfile): add StopHook field with pointer merge semantics
```

---

### Task 2: Route `spinclass hooks` on `hook_event_name`

**Files:**
- Modify: `packages/spinclass/internal/hooks/hooks.go:11-41`
- Test: `packages/spinclass/internal/hooks/hooks_test.go`

**Step 1: Write the failing test**

Add to `hooks_test.go`:

```go
func TestStopHookEventRouteApproves(t *testing.T) {
	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "test-session-123",
		"cwd":             t.TempDir(),
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No stop_hook configured -> approve (no output)
	if out.Len() != 0 {
		t.Errorf("expected no output for Stop with no stop_hook, got %q", out.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop ../.. --command go test ./packages/spinclass/internal/hooks/ -run TestStopHookEventRouteApproves -v`
Expected: FAIL or PASS depending on how the existing `Run()` handles unknown events. The key is that `hook_event_name` is not parsed yet.

**Step 3: Refactor hookInput and Run to route on event name**

In `hooks.go`, update the struct and `Run` function:

```go
type hookInput struct {
	HookEventName string         `json:"hook_event_name"`
	SessionID     string         `json:"session_id"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	CWD           string         `json:"cwd"`
}

func Run(r io.Reader, w io.Writer, boundary string) error {
	var input hookInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return fmt.Errorf("decoding hook input: %w", err)
	}

	switch input.HookEventName {
	case "Stop":
		return runStopHook(input, w)
	default:
		return runPreToolUse(input, w, boundary)
	}
}
```

Extract the existing boundary logic into `runPreToolUse(input hookInput, w io.Writer, boundary string) error` — same body as current `Run` after the decode, just using the already-decoded `input`.

Add a stub `runStopHook`:

```go
func runStopHook(input hookInput, w io.Writer) error {
	return nil // no stop_hook configured -> approve
}
```

**Step 4: Run all hooks tests to verify nothing broke**

Run: `nix develop ../.. --command go test ./packages/spinclass/internal/hooks/ -v`
Expected: All PASS (existing PreToolUse tests still work, new Stop test passes).

**Step 5: Commit**

```
refactor(hooks): route on hook_event_name, extract runPreToolUse
```

---

### Task 3: Implement stop hook execution with sentinel

**Files:**
- Modify: `packages/spinclass/internal/hooks/hooks.go`
- Test: `packages/spinclass/internal/hooks/hooks_test.go`

**Step 1: Write failing tests for the stop hook flow**

Add to `hooks_test.go`:

```go
func TestStopHookBlocksOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	// Create a sweatfile with a failing stop_hook
	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "sweatfile"), []byte(`stop_hook = "false"`), 0o644)

	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "block-test-session",
		"cwd":             cwd,
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected block output for failing stop_hook")
	}

	var result map[string]any
	json.Unmarshal(out.Bytes(), &result)
	if result["decision"] != "block" {
		t.Errorf("expected block decision, got %v", result["decision"])
	}

	// Sentinel file should exist
	sentinel := filepath.Join(tmpDir, "stop-hook-block-test-session")
	if _, err := os.Stat(sentinel); os.IsNotExist(err) {
		t.Error("expected sentinel file to be created")
	}
}

func TestStopHookApprovesOnSecondInvocation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "sweatfile"), []byte(`stop_hook = "false"`), 0o644)

	// Create sentinel file (simulating first invocation already happened)
	sentinel := filepath.Join(tmpDir, "stop-hook-approve-test-session")
	os.WriteFile(sentinel, []byte("previous failure output"), 0o644)

	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "approve-test-session",
		"cwd":             cwd,
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sentinel exists -> approve (no output)
	if out.Len() != 0 {
		t.Errorf("expected no output on second invocation, got %q", out.String())
	}
}

func TestStopHookApprovesOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "sweatfile"), []byte(`stop_hook = "true"`), 0o644)

	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "success-test-session",
		"cwd":             cwd,
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for passing stop_hook, got %q", out.String())
	}

	// No sentinel should exist on success
	sentinel := filepath.Join(tmpDir, "stop-hook-success-test-session")
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Error("expected no sentinel file for successful stop_hook")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop ../.. --command go test ./packages/spinclass/internal/hooks/ -run 'TestStopHook' -v`
Expected: FAIL — `runStopHook` is a no-op stub.

**Step 3: Implement runStopHook**

In `hooks.go`, add the imports and implement `runStopHook`:

```go
import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/spinclass/internal/sweatfile"
)

func runStopHook(input hookInput, w io.Writer) error {
	tmpDir := os.TempDir()
	sentinelPath := filepath.Join(tmpDir, "stop-hook-"+input.SessionID)

	if _, err := os.Stat(sentinelPath); err == nil {
		return nil // second invocation -> approve
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil // can't load sweatfile -> approve
	}

	result, err := sweatfile.LoadHierarchy(home, input.CWD)
	if err != nil || result.Merged.StopHook == nil || *result.Merged.StopHook == "" {
		return nil // no stop_hook -> approve
	}

	cmd := exec.Command("sh", "-c", *result.Merged.StopHook)
	cmd.Dir = input.CWD
	output, cmdErr := cmd.CombinedOutput()

	if cmdErr == nil {
		return nil // command passed -> approve
	}

	// Command failed -> write output to sentinel and block
	os.WriteFile(sentinelPath, output, 0o644)

	reason := fmt.Sprintf("stop_hook failed: %s", *result.Merged.StopHook)
	systemMsg := fmt.Sprintf(
		"Stop hook failed. Output written to %s. Review the failures and address them before completing.",
		sentinelPath,
	)

	decision := map[string]any{
		"decision":      "block",
		"reason":        reason,
		"systemMessage": systemMsg,
	}

	return json.NewEncoder(w).Encode(decision)
}
```

**Step 4: Run all hooks tests**

Run: `nix develop ../.. --command go test ./packages/spinclass/internal/hooks/ -v`
Expected: All PASS.

**Step 5: Commit**

```
feat(hooks): implement stop hook with sentinel-based once-per-session blocking
```

---

### Task 4: Register Stop hook in ApplyClaudeSettings

**Files:**
- Modify: `packages/spinclass/internal/sweatfile/apply.go:29,64,96-111`
- Test: `packages/spinclass/internal/sweatfile/apply_test.go`

**Step 1: Write the failing tests**

Add to `apply_test.go`:

```go
func TestApplyClaudeSettingsWritesStopHookWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /tmp/fake"), 0o644)

	cmd := "just test"
	sf := sweatfile.Sweatfile{StopHook: &cmd}
	err := ApplyClaudeSettings(dir, sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	var doc map[string]any
	json.Unmarshal(data, &doc)

	hooks := doc["hooks"].(map[string]any)

	stopRaw, ok := hooks["Stop"]
	if !ok {
		t.Fatal("expected Stop key in hooks")
	}

	entries := stopRaw.([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 Stop entry, got %d", len(entries))
	}

	entry := entries[0].(map[string]any)
	if entry["matcher"] != "*" {
		t.Errorf("matcher: got %q", entry["matcher"])
	}
}

func TestApplyClaudeSettingsNoStopHookWhenNotConfigured(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /tmp/fake"), 0o644)

	sf := sweatfile.Sweatfile{}
	err := ApplyClaudeSettings(dir, sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	var doc map[string]any
	json.Unmarshal(data, &doc)

	hooks := doc["hooks"].(map[string]any)
	if _, ok := hooks["Stop"]; ok {
		t.Error("expected no Stop key when stop_hook is not configured")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop ../.. --command go test ./packages/spinclass/internal/sweatfile/ -run 'TestApplyClaudeSettings.*StopHook' -v`
Expected: FAIL — `ApplyClaudeSettings` signature doesn't accept `Sweatfile`.

**Step 3: Update ApplyClaudeSettings signature and hook generation**

Change `ApplyClaudeSettings` to accept the full `Sweatfile` instead of just `[]string`:

```go
func ApplyClaudeSettings(worktreePath string, sf Sweatfile) error {
```

Update the body to use `sf.ClaudeAllow` where `rules` was used. Then update the hooks section:

```go
if git.IsWorktree(worktreePath) {
	hooksMap := map[string]any{
		"PreToolUse": []any{
			map[string]any{
				"matcher": "Read|Write|Edit|Glob|Grep|Bash|Task",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": "spinclass hooks",
					},
				},
			},
		},
	}

	if sf.StopHook != nil && *sf.StopHook != "" {
		hooksMap["Stop"] = []any{
			map[string]any{
				"matcher": "*",
				"hooks": []any{
					map[string]any{
						"type":    "command",
						"command": "spinclass hooks",
					},
				},
			},
		}
	}

	doc["hooks"] = hooksMap
}
```

Update `Apply()` to pass the full sweatfile:

```go
if err := ApplyClaudeSettings(worktreePath, sf); err != nil {
```

**Step 4: Fix existing tests for new signature**

Update all existing `ApplyClaudeSettings` calls in `apply_test.go` to pass a `Sweatfile` struct instead of `[]string`. For example:

```go
// Before:
ApplyClaudeSettings(dir, []string{"Read", "Glob", "Bash(git *)"})

// After:
ApplyClaudeSettings(dir, Sweatfile{ClaudeAllow: []string{"Read", "Glob", "Bash(git *)"}})
```

The `TestApplyClaudeSettingsEmpty` test passes `nil` for rules — change to `Sweatfile{}`.

**Step 5: Run all sweatfile tests**

Run: `nix develop ../.. --command go test ./packages/spinclass/internal/sweatfile/ -v`
Expected: All PASS.

**Step 6: Commit**

```
feat(apply): register Stop hook when stop_hook is configured in sweatfile
```

---

### Task 5: Run full test suite and verify

**Step 1: Run all spinclass tests**

Run: `nix develop ../.. --command go test ./packages/spinclass/... -v`
Expected: All PASS. Watch for any callers of `ApplyClaudeSettings` outside `apply.go` that need the new signature.

**Step 2: Check for other callers of ApplyClaudeSettings**

Grep for `ApplyClaudeSettings` across the codebase. The known callers are:
- `apply.go:Apply()` — already updated in Task 4
- `apply_test.go` — already updated in Task 4

If `worktree.go` or other files call it, update them too.

**Step 3: Run nix build**

Run: `nix build ../..#spinclass`
Expected: Build succeeds.

**Step 4: Commit if any fixes were needed**

```
fix: update remaining ApplyClaudeSettings callers for new signature
```

---

### Task 6: Add LoadHierarchy integration test for stop_hook

**Files:**
- Test: `packages/spinclass/internal/sweatfile/sweatfile_test.go`

**Step 1: Write the test**

```go
func TestLoadHierarchyStopHookInherited(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "repos", "myrepo")
	os.MkdirAll(repoDir, 0o755)

	globalPath := filepath.Join(home, ".config", "spinclass", "sweatfile")
	writeSweatfile(t, globalPath, `stop_hook = "just test"`)

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("LoadHierarchy returned error: %v", err)
	}

	if result.Merged.StopHook == nil || *result.Merged.StopHook != "just test" {
		t.Errorf("expected inherited stop_hook, got %v", result.Merged.StopHook)
	}
}

func TestLoadHierarchyStopHookOverriddenByRepo(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "repos", "myrepo")
	os.MkdirAll(repoDir, 0o755)

	globalPath := filepath.Join(home, ".config", "spinclass", "sweatfile")
	writeSweatfile(t, globalPath, `stop_hook = "just test"`)

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `stop_hook = "just lint"`)

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("LoadHierarchy returned error: %v", err)
	}

	if result.Merged.StopHook == nil || *result.Merged.StopHook != "just lint" {
		t.Errorf("expected overridden stop_hook, got %v", result.Merged.StopHook)
	}
}
```

**Step 2: Run tests**

Run: `nix develop ../.. --command go test ./packages/spinclass/internal/sweatfile/ -v`
Expected: All PASS.

**Step 3: Commit**

```
test: add LoadHierarchy integration tests for stop_hook inheritance
```
