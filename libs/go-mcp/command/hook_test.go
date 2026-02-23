package command

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleHookDeniesMatch(t *testing.T) {
	app := NewApp("grit", "Git MCP server")
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show working tree status"},
		MapsTools: []ToolMapping{
			{
				Replaces:        "Bash",
				CommandPrefixes: []string{"git status"},
				UseWhen:         "checking repository status",
			},
		},
	})

	input := hookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "git status --short"},
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}

	var out bytes.Buffer
	if err := app.HandleHook(bytes.NewReader(data), &out); err != nil {
		t.Fatalf("HandleHook error: %v", err)
	}

	var got hookOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output: %v\nraw: %s", err, out.String())
	}

	if got.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("permissionDecision = %q, want %q",
			got.HookSpecificOutput.PermissionDecision, "deny")
	}

	reason := got.HookSpecificOutput.PermissionDecisionReason
	if !strings.Contains(reason, "mcp__plugin_grit_grit__status") {
		t.Errorf("reason missing tool name:\n  got:  %s\n  want substring: mcp__plugin_grit_grit__status", reason)
	}
}

func TestHandleHookAllowsNoMatch(t *testing.T) {
	app := NewApp("grit", "Git MCP server")
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show working tree status"},
		MapsTools: []ToolMapping{
			{
				Replaces:        "Bash",
				CommandPrefixes: []string{"git status"},
				UseWhen:         "checking repository status",
			},
		},
	})

	input := hookInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "docker ps"},
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}

	var out bytes.Buffer
	if err := app.HandleHook(bytes.NewReader(data), &out); err != nil {
		t.Fatalf("HandleHook error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected empty output for no match, got %q", out.String())
	}
}

func TestHandleHookExtractsFilePath(t *testing.T) {
	app := NewApp("lux", "LSP multiplexer MCP server")
	app.AddCommand(&Command{
		Name:        "hover",
		Description: Description{Short: "Get hover information"},
		MapsTools: []ToolMapping{
			{
				Replaces:   "Read",
				Extensions: []string{".go"},
				UseWhen:    "getting type info or docs for a symbol",
			},
		},
	})

	input := hookInput{
		ToolName:  "Read",
		ToolInput: map[string]any{"file_path": "/foo/bar.go"},
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}

	var out bytes.Buffer
	if err := app.HandleHook(bytes.NewReader(data), &out); err != nil {
		t.Fatalf("HandleHook error: %v", err)
	}

	var got hookOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output: %v\nraw: %s", err, out.String())
	}

	if got.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("permissionDecision = %q, want %q",
			got.HookSpecificOutput.PermissionDecision, "deny")
	}

	reason := got.HookSpecificOutput.PermissionDecisionReason
	if !strings.Contains(reason, "mcp__plugin_lux_lux__hover") {
		t.Errorf("reason missing tool name:\n  got:  %s\n  want substring: mcp__plugin_lux_lux__hover", reason)
	}
}
