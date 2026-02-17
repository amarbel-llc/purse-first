package command

import "testing"

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
