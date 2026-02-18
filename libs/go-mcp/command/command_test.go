package command

import (
	"context"
	"encoding/json"
	"testing"
)

func TestParamTypeString(t *testing.T) {
	tests := []struct {
		pt   ParamType
		want string
	}{
		{String, "string"},
		{Int, "integer"},
		{Bool, "boolean"},
		{Float, "number"},
		{Array, "array"},
	}
	for _, tt := range tests {
		if got := tt.pt.JSONSchemaType(); got != tt.want {
			t.Errorf("ParamType(%d).JSONSchemaType() = %q, want %q", tt.pt, got, tt.want)
		}
	}
}

func TestCommandHasRunAndRunCLI(t *testing.T) {
	cmd := Command{
		Name: "status",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return TextResult("ok"), nil
		},
	}
	if cmd.Run == nil {
		t.Error("Run should be set")
	}
	if cmd.RunCLI != nil {
		t.Error("RunCLI should be nil")
	}
}

func TestCommandCLIOnly(t *testing.T) {
	cmd := Command{
		Name: "open",
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			return nil
		},
	}
	if cmd.Run != nil {
		t.Error("Run should be nil for CLI-only commands")
	}
	if cmd.RunCLI == nil {
		t.Error("RunCLI should be set")
	}
}

func TestCommandParamsRequired(t *testing.T) {
	cmd := Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
	}

	required := cmd.RequiredParams()
	if len(required) != 1 {
		t.Fatalf("RequiredParams() len = %d, want 1", len(required))
	}
	if required[0].Name != "repo_path" {
		t.Errorf("RequiredParams()[0].Name = %q, want %q", required[0].Name, "repo_path")
	}
}
