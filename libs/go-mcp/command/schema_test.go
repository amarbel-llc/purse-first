package command

import (
	"encoding/json"
	"testing"
)

func TestCommandInputSchema(t *testing.T) {
	cmd := Command{
		Name: "status",
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	if parsed["type"] != "object" {
		t.Errorf("type = %v, want object", parsed["type"])
	}

	props := parsed["properties"].(map[string]any)
	repoProp := props["repo_path"].(map[string]any)
	if repoProp["type"] != "string" {
		t.Errorf("repo_path.type = %v, want string", repoProp["type"])
	}
	if repoProp["description"] != "Path to repo" {
		t.Errorf("repo_path.description = %v", repoProp["description"])
	}

	verboseProp := props["verbose"].(map[string]any)
	if verboseProp["type"] != "boolean" {
		t.Errorf("verbose.type = %v, want boolean", verboseProp["type"])
	}

	required := parsed["required"].([]any)
	if len(required) != 1 || required[0] != "repo_path" {
		t.Errorf("required = %v, want [repo_path]", required)
	}
}

func TestCommandInputSchemaNoRequired(t *testing.T) {
	cmd := Command{
		Name: "ping",
		Params: []Param{
			{Name: "message", Type: String, Description: "Optional message"},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	json.Unmarshal(schema, &parsed)

	if _, ok := parsed["required"]; ok {
		t.Error("required should be omitted when no params are required")
	}
}

func TestCommandInputSchemaArray(t *testing.T) {
	cmd := Command{
		Name: "diff",
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
			{Name: "paths", Type: Array, Description: "Limit diff to specific paths"},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	props := parsed["properties"].(map[string]any)
	pathsProp := props["paths"].(map[string]any)
	if pathsProp["type"] != "array" {
		t.Errorf("paths.type = %v, want array", pathsProp["type"])
	}

	items := pathsProp["items"].(map[string]any)
	if items["type"] != "string" {
		t.Errorf("paths.items.type = %v, want string", items["type"])
	}
}

func TestCommandInputSchemaWithDefault(t *testing.T) {
	cmd := Command{
		Name: "serve",
		Params: []Param{
			{Name: "port", Type: Int, Description: "Port number", Default: 8080},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	json.Unmarshal(schema, &parsed)

	props := parsed["properties"].(map[string]any)
	portProp := props["port"].(map[string]any)
	if portProp["default"] != float64(8080) {
		t.Errorf("port.default = %v, want 8080", portProp["default"])
	}
}
