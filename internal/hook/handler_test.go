package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amarbel-llc/purse-first/internal/decision"
	"github.com/amarbel-llc/purse-first/internal/mapping"
)

func setupMappings(t *testing.T, dir string) {
	t.Helper()

	mf := mapping.MappingFile{
		Server: "lux",
		Mappings: []mapping.Mapping{
			{
				Replaces:   "Read",
				Extensions: []string{".go", ".py"},
				Tools: []mapping.ToolSuggestion{
					{Name: "document_symbols", UseWhen: "understanding file structure"},
					{Name: "hover", UseWhen: "getting type info"},
				},
				Reason: "Use lux MCP tools instead",
			},
			{
				Replaces:   "Grep",
				Extensions: []string{".go"},
				Tools: []mapping.ToolSuggestion{
					{Name: "workspace_symbols", UseWhen: "finding symbols by name"},
				},
				Reason: "Use lux MCP tools for semantic search",
			},
		},
	}

	gritMF := mapping.MappingFile{
		Server: "grit",
		Mappings: []mapping.Mapping{
			{
				Replaces:        "Bash",
				CommandPrefixes: []string{"git "},
				Tools: []mapping.ToolSuggestion{
					{Name: "status", UseWhen: "checking repository status"},
					{Name: "diff", UseWhen: "viewing changes"},
				},
				Reason: "Use grit MCP tools for git operations",
			},
		},
	}

	gritData, err := json.Marshal(gritMF)
	if err != nil {
		t.Fatal(err)
	}

	mappingDir := filepath.Join(dir, ".purse-first")
	if err := os.MkdirAll(mappingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(mf)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(mappingDir, "lux.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(mappingDir, "grit.json"), gritData, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerDeniesMatchingTool(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID:     "test",
		ToolName:      "Read",
		ToolInput:     map[string]any{"file_path": "/path/to/foo.go"},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	var output decision.HookOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}

	if output.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("expected deny, got %s", output.HookSpecificOutput.PermissionDecision)
	}

	reason := output.HookSpecificOutput.PermissionDecisionReason
	if !strings.Contains(reason, "mcp__plugin_lux_lux__document_symbols") {
		t.Errorf("expected mcp__plugin_lux_lux__ prefix in reason, got: %s", reason)
	}

	if !strings.Contains(reason, "mcp__plugin_lux_lux__hover") {
		t.Errorf("expected lsp_hover suggestion in reason, got: %s", reason)
	}
}

func TestHandlerPassthroughNoMappings(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	input := decision.HookInput{
		SessionID:     "test",
		ToolName:      "Read",
		ToolInput:     map[string]any{"file_path": "/path/to/foo.go"},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected empty output for passthrough, got: %s", stdout.String())
	}
}

func TestHandlerPassthroughNonMatchingExtension(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID:     "test",
		ToolName:      "Read",
		ToolInput:     map[string]any{"file_path": "/path/to/readme.md"},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected passthrough for .md file, got: %s", stdout.String())
	}
}

func TestHandlerPassthroughNoFilePath(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID:     "test",
		ToolName:      "Read",
		ToolInput:     map[string]any{},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected passthrough with no file_path, got: %s", stdout.String())
	}
}

func TestHandlerFailOpenOnBadJSON(t *testing.T) {
	var stdout bytes.Buffer
	err := HandlePreToolUse(strings.NewReader("not json"), &stdout, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected passthrough on bad JSON, got: %s", stdout.String())
	}
}

func TestHandlerGrepWithPath(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID: "test",
		ToolName:  "Grep",
		ToolInput: map[string]any{
			"pattern": "func main",
			"path":    "/path/to/foo.go",
		},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	var output decision.HookOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("expected deny for Grep on .go path, got %s", output.HookSpecificOutput.PermissionDecision)
	}
}

func TestHandlerDeniesGitCommand(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID:     "test",
		ToolName:      "Bash",
		ToolInput:     map[string]any{"command": "git status"},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	var output decision.HookOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}

	if output.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("expected deny for git command, got %s", output.HookSpecificOutput.PermissionDecision)
	}

	reason := output.HookSpecificOutput.PermissionDecisionReason
	if !strings.Contains(reason, "mcp__plugin_grit_grit__status") {
		t.Errorf("expected grit status suggestion in reason, got: %s", reason)
	}
}

func TestHandlerPassthroughNonGitCommand(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID:     "test",
		ToolName:      "Bash",
		ToolInput:     map[string]any{"command": "npm install"},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected passthrough for npm command, got: %s", stdout.String())
	}
}

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
