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

func TestCommandInputSchemaVariadicIsStringArray(t *testing.T) {
	// A variadic param is declared with its element Type (String) but
	// always crosses the wire as an array, so an MCP client must be told
	// "array of string" or it will send a bare string the handler can't
	// unmarshal into []string.
	cmd := &Command{
		Name: "close",
		Params: []Param{
			{Name: "target", Type: String, Description: "sessions", Variadic: true},
		},
	}

	var schema map[string]any
	if err := json.Unmarshal(cmd.InputSchema(), &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no properties: %v", schema)
	}
	target, ok := props["target"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no target property: %v", props)
	}
	if target["type"] != "array" {
		t.Errorf("variadic param type = %v, want array", target["type"])
	}
	items, ok := target["items"].(map[string]any)
	if !ok {
		t.Fatalf("variadic param has no items schema: %v", target)
	}
	if items["type"] != "string" {
		t.Errorf("variadic items type = %v, want string", items["type"])
	}
}

func TestCommandInputSchemaObject(t *testing.T) {
	cmd := Command{
		Name: "exec",
		Params: []Param{
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
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
			{
				Name: "todo", Type: Array,
				Description: "Ordered list of rebase entries",
				Required:    true,
				Items: []Param{
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
