package command

import (
	"encoding/json"
	"testing"
)

func TestCommandInputSchema(t *testing.T) {
	cmd := Command{
		Name: "status",
		OldParams: []OldParam{
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
		OldParams: []OldParam{
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
		OldParams: []OldParam{
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

func TestCommandInputSchemaObject(t *testing.T) {
	cmd := Command{
		Name: "exec",
		OldParams: []OldParam{
			{Name: "server", Type: String, Description: "Server name", Required: true},
			{Name: "args", Type: Object, Description: "Arguments as JSON object"},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	props := parsed["properties"].(map[string]any)
	argsProp := props["args"].(map[string]any)
	if argsProp["type"] != "object" {
		t.Errorf("args.type = %v, want object", argsProp["type"])
	}

	if _, ok := argsProp["items"]; ok {
		t.Error("object params should not have items")
	}
}

func TestCommandInputSchemaArrayWithObjectItems(t *testing.T) {
	cmd := Command{
		Name: "rebase",
		OldParams: []OldParam{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
			{
				Name: "todo", Type: Array,
				Description: "Ordered list of rebase entries",
				Required:    true,
				Items: []OldParam{
					{Name: "action", Type: String, Description: "Rebase action", Required: true},
					{Name: "hash", Type: String, Description: "Commit hash", Required: true},
					{Name: "message", Type: String, Description: "New commit message"},
				},
			},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	props := parsed["properties"].(map[string]any)
	todoProp := props["todo"].(map[string]any)
	if todoProp["type"] != "array" {
		t.Errorf("todo.type = %v, want array", todoProp["type"])
	}

	items := todoProp["items"].(map[string]any)
	if items["type"] != "object" {
		t.Errorf("todo.items.type = %v, want object", items["type"])
	}

	itemProps := items["properties"].(map[string]any)
	actionProp := itemProps["action"].(map[string]any)
	if actionProp["type"] != "string" {
		t.Errorf("action.type = %v, want string", actionProp["type"])
	}
	if actionProp["description"] != "Rebase action" {
		t.Errorf("action.description = %v", actionProp["description"])
	}

	hashProp := itemProps["hash"].(map[string]any)
	if hashProp["type"] != "string" {
		t.Errorf("hash.type = %v, want string", hashProp["type"])
	}

	messageProp := itemProps["message"].(map[string]any)
	if messageProp["type"] != "string" {
		t.Errorf("message.type = %v, want string", messageProp["type"])
	}

	itemRequired := items["required"].([]any)
	requiredNames := make(map[string]bool)
	for _, r := range itemRequired {
		requiredNames[r.(string)] = true
	}
	if !requiredNames["action"] {
		t.Error("items.required should contain action")
	}
	if !requiredNames["hash"] {
		t.Error("items.required should contain hash")
	}
	if requiredNames["message"] {
		t.Error("items.required should not contain message")
	}
	if len(itemRequired) != 2 {
		t.Errorf("items.required length = %d, want 2", len(itemRequired))
	}
}

func TestCommandInputSchemaWithDefault(t *testing.T) {
	cmd := Command{
		Name: "serve",
		OldParams: []OldParam{
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

// --- New Param interface schema tests ---

func TestNewParamInputSchema(t *testing.T) {
	cmd := Command{
		Name: "status",
		Params: []Param{
			StringFlag{Name: "repo_path", Description: "Path to repo", Required: true},
			BoolFlag{Name: "verbose", Description: "Verbose output"},
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

	verboseProp := props["verbose"].(map[string]any)
	if verboseProp["type"] != "boolean" {
		t.Errorf("verbose.type = %v, want boolean", verboseProp["type"])
	}

	required := parsed["required"].([]any)
	if len(required) != 1 || required[0] != "repo_path" {
		t.Errorf("required = %v, want [repo_path]", required)
	}
}

func TestNewParamInputSchemaEnum(t *testing.T) {
	cmd := Command{
		Name: "list",
		Params: []Param{
			StringFlag{
				Name:        "format",
				Description: "Output format",
				EnumValues:  []string{"json", "text", "tap"},
			},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	json.Unmarshal(schema, &parsed)

	props := parsed["properties"].(map[string]any)
	formatProp := props["format"].(map[string]any)

	enumVals, ok := formatProp["enum"].([]any)
	if !ok {
		t.Fatal("format.enum missing")
	}
	if len(enumVals) != 3 {
		t.Fatalf("format.enum len = %d, want 3", len(enumVals))
	}
	if enumVals[0] != "json" || enumVals[1] != "text" || enumVals[2] != "tap" {
		t.Errorf("format.enum = %v, want [json text tap]", enumVals)
	}
}

func TestNewParamInputSchemaPositionalArg(t *testing.T) {
	cmd := Command{
		Name: "open",
		Params: []Param{
			StringArg{Name: "target", Description: "Target to open", Required: true},
			StringFlag{Name: "format", Description: "Output format"},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	json.Unmarshal(schema, &parsed)

	props := parsed["properties"].(map[string]any)

	// Positional args appear in schema the same as flags
	targetProp := props["target"].(map[string]any)
	if targetProp["type"] != "string" {
		t.Errorf("target.type = %v, want string", targetProp["type"])
	}

	required := parsed["required"].([]any)
	if len(required) != 1 || required[0] != "target" {
		t.Errorf("required = %v, want [target]", required)
	}
}

func TestNewParamInputSchemaArrayFlag(t *testing.T) {
	cmd := Command{
		Name: "diff",
		Params: []Param{
			StringFlag{Name: "repo_path", Description: "Path to repo", Required: true},
			ArrayFlag{Name: "paths", Description: "Limit diff to specific paths"},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	json.Unmarshal(schema, &parsed)

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

func TestNewParamInputSchemaArrayFlagWithObjectItems(t *testing.T) {
	cmd := Command{
		Name: "rebase",
		Params: []Param{
			StringFlag{Name: "repo_path", Description: "Path to repo", Required: true},
			ArrayFlag{
				Name:        "todo",
				Description: "Ordered list of rebase entries",
				Required:    true,
				Items: []Param{
					StringFlag{Name: "action", Description: "Rebase action", Required: true},
					StringFlag{Name: "hash", Description: "Commit hash", Required: true},
					StringFlag{Name: "message", Description: "New commit message"},
				},
			},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	props := parsed["properties"].(map[string]any)
	todoProp := props["todo"].(map[string]any)
	items := todoProp["items"].(map[string]any)
	if items["type"] != "object" {
		t.Errorf("todo.items.type = %v, want object", items["type"])
	}

	itemProps := items["properties"].(map[string]any)
	if _, ok := itemProps["action"]; !ok {
		t.Error("items.properties missing action")
	}

	itemRequired := items["required"].([]any)
	if len(itemRequired) != 2 {
		t.Errorf("items.required length = %d, want 2", len(itemRequired))
	}
}

func TestNewParamInputSchemaObjectFlag(t *testing.T) {
	cmd := Command{
		Name: "exec",
		Params: []Param{
			StringFlag{Name: "server", Description: "Server name", Required: true},
			ObjectFlag{Name: "args", Description: "Arguments as JSON object"},
		},
	}

	schema := cmd.InputSchema()

	var parsed map[string]any
	json.Unmarshal(schema, &parsed)

	props := parsed["properties"].(map[string]any)
	argsProp := props["args"].(map[string]any)
	if argsProp["type"] != "object" {
		t.Errorf("args.type = %v, want object", argsProp["type"])
	}
	if _, ok := argsProp["items"]; ok {
		t.Error("object params should not have items")
	}
}

func TestNewParamInputSchemaWithDefault(t *testing.T) {
	cmd := Command{
		Name: "serve",
		Params: []Param{
			IntFlag{Name: "port", Description: "Port number", Default: 8080},
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
