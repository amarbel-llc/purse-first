package validate

import (
	"testing"
)

func TestDetectTypeFromFilename(t *testing.T) {
	tests := []struct {
		name string
		want DocType
	}{
		{"plugin.json", PluginDoc},
		{".claude-plugin/plugin.json", PluginDoc},
		{"mappings.json", MappingDoc},
		{"other.json", Unknown},
		{"README.md", Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectTypeFromFilename(tt.name)
			if got != tt.want {
				t.Errorf("DetectTypeFromFilename(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestDetectTypeFromContent(t *testing.T) {
	tests := []struct {
		name string
		data string
		want DocType
	}{
		{
			"mapping",
			`{"server":"x","mappings":[]}`,
			MappingDoc,
		},
		{
			"plugin",
			`{"name":"x","mcpServers":{}}`,
			PluginDoc,
		},
		{
			"unknown",
			`{"foo":"bar"}`,
			Unknown,
		},
		{
			"invalid json",
			`not json`,
			Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectTypeFromContent([]byte(tt.data))
			if got != tt.want {
				t.Errorf("DetectTypeFromContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePluginMinimal(t *testing.T) {
	data := []byte(`{"name":"my-plugin","mcpServers":{"my-plugin":{"type":"stdio","command":"my-plugin"}}}`)
	r, dt, err := ValidateBytes(data, PluginDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	if dt != PluginDoc {
		t.Errorf("got doctype %v, want %v", dt, PluginDoc)
	}
	if r.HasErrors() {
		t.Errorf("unexpected errors: %v", r.Errors())
	}
}

func TestValidatePluginMissingName(t *testing.T) {
	data := []byte(`{}`)
	r, _, err := ValidateBytes(data, PluginDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasErrors() {
		t.Error("expected errors for empty plugin")
	}
	assertHasError(t, r, "name", "name is required")
}

func TestValidatePluginBadName(t *testing.T) {
	data := []byte(`{"name":"My Plugin"}`)
	r, _, err := ValidateBytes(data, PluginDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasError(t, r, "name", "must match")
}

func TestValidatePluginNoMcpServers(t *testing.T) {
	data := []byte(`{"name":"my-plugin"}`)
	r, _, err := ValidateBytes(data, PluginDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	if !r.HasWarnings() {
		t.Error("expected warning about no MCP servers")
	}
}

func TestValidatePluginBadServerType(t *testing.T) {
	data := []byte(`{"name":"x","mcpServers":{"x":{"type":"http","command":"x"}}}`)
	r, _, err := ValidateBytes(data, PluginDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasError(t, r, "mcpServers.x.type", "must be")
}

func TestValidatePluginMissingCommand(t *testing.T) {
	data := []byte(`{"name":"x","mcpServers":{"x":{"type":"stdio"}}}`)
	r, _, err := ValidateBytes(data, PluginDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasError(t, r, "mcpServers.x.command", "command is required")
}

func TestValidatePluginCommandWithSpaces(t *testing.T) {
	data := []byte(`{"name":"x","mcpServers":{"x":{"type":"stdio","command":"my cmd --flag"}}}`)
	r, _, err := ValidateBytes(data, PluginDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasWarning(t, r, "mcpServers.x.command", "spaces")
}

func TestValidatePluginAuthorWithoutName(t *testing.T) {
	data := []byte(`{"name":"x","author":{}}`)
	r, _, err := ValidateBytes(data, PluginDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasError(t, r, "author.name", "author name is required")
}

func TestValidatePluginUnknownFieldsStrict(t *testing.T) {
	data := []byte(`{"name":"x","foo":"bar"}`)
	r, _, err := ValidateBytes(data, PluginDoc, true)
	if err != nil {
		t.Fatal(err)
	}
	assertHasError(t, r, "foo", "unknown field")
}

func TestValidatePluginUnknownFieldsNonStrict(t *testing.T) {
	data := []byte(`{"name":"x","foo":"bar"}`)
	r, _, err := ValidateBytes(data, PluginDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasWarning(t, r, "foo", "unknown field")
}

func TestValidatePluginInvalidJSON(t *testing.T) {
	data := []byte(`not json`)
	r, _, err := ValidateBytes(data, PluginDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasError(t, r, "", "invalid JSON")
}

func TestValidateMappingMinimal(t *testing.T) {
	data := []byte(`{
		"server": "my-server",
		"mappings": [{
			"replaces": "Read",
			"extensions": [".go"],
			"tools": [{"name": "my_read", "use_when": "reading go files"}],
			"reason": "better go support"
		}]
	}`)
	r, dt, err := ValidateBytes(data, MappingDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	if dt != MappingDoc {
		t.Errorf("got doctype %v, want %v", dt, MappingDoc)
	}
	if r.HasErrors() {
		t.Errorf("unexpected errors: %v", r.Errors())
	}
}

func TestValidateMappingMissingServer(t *testing.T) {
	data := []byte(`{"mappings":[]}`)
	r, _, err := ValidateBytes(data, MappingDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasError(t, r, "server", "server is required")
}

func TestValidateMappingEmptyMappings(t *testing.T) {
	data := []byte(`{"server":"x","mappings":[]}`)
	r, _, err := ValidateBytes(data, MappingDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasError(t, r, "mappings", "at least one mapping is required")
}

func TestValidateMappingUnknownTool(t *testing.T) {
	data := []byte(`{
		"server": "x",
		"mappings": [{
			"replaces": "FooTool",
			"extensions": [".go"],
			"tools": [{"name": "t", "use_when": "w"}],
			"reason": "r"
		}]
	}`)
	r, _, err := ValidateBytes(data, MappingDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasWarning(t, r, "mappings[0].replaces", "not a known built-in tool")
}

func TestValidateMappingMissingToolFields(t *testing.T) {
	data := []byte(`{
		"server": "x",
		"mappings": [{
			"replaces": "Read",
			"extensions": [".go"],
			"tools": [{"name": "", "use_when": ""}],
			"reason": "r"
		}]
	}`)
	r, _, err := ValidateBytes(data, MappingDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasError(t, r, "mappings[0].tools[0].name", "name is required")
	assertHasError(t, r, "mappings[0].tools[0].use_when", "use_when is required")
}

func TestValidateMappingExtensionFormat(t *testing.T) {
	data := []byte(`{
		"server": "x",
		"mappings": [{
			"replaces": "Read",
			"extensions": ["go"],
			"tools": [{"name": "t", "use_when": "w"}],
			"reason": "r"
		}]
	}`)
	r, _, err := ValidateBytes(data, MappingDoc, false)
	if err != nil {
		t.Fatal(err)
	}
	assertHasWarning(t, r, "mappings[0].extensions", "should start with")
}

func TestValidateMappingNoScopingStrict(t *testing.T) {
	data := []byte(`{
		"server": "x",
		"mappings": [{
			"replaces": "Read",
			"tools": [{"name": "t", "use_when": "w"}],
			"reason": "r"
		}]
	}`)
	r, _, err := ValidateBytes(data, MappingDoc, true)
	if err != nil {
		t.Fatal(err)
	}
	assertHasError(t, r, "mappings[0]", "must have extensions or command_prefixes")
}

func TestValidateBytesAutoDetect(t *testing.T) {
	data := []byte(`{"name":"x","mcpServers":{"x":{"type":"stdio","command":"x"}}}`)
	_, dt, err := ValidateBytes(data, Unknown, false)
	if err != nil {
		t.Fatal(err)
	}
	if dt != PluginDoc {
		t.Errorf("auto-detected %v, want %v", dt, PluginDoc)
	}
}

func TestValidateBytesUnknownType(t *testing.T) {
	data := []byte(`{"foo":"bar"}`)
	_, _, err := ValidateBytes(data, Unknown, false)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestResultMethods(t *testing.T) {
	r := &Result{}
	r.addError("a", "error msg")
	r.addWarning("b", "warning msg")

	if !r.HasErrors() {
		t.Error("HasErrors should be true")
	}
	if !r.HasWarnings() {
		t.Error("HasWarnings should be true")
	}
	if len(r.Errors()) != 1 {
		t.Errorf("got %d errors, want 1", len(r.Errors()))
	}
	if len(r.Warnings()) != 1 {
		t.Errorf("got %d warnings, want 1", len(r.Warnings()))
	}
	if len(r.Issues()) != 2 {
		t.Errorf("got %d issues, want 2", len(r.Issues()))
	}
}

func TestIssueString(t *testing.T) {
	i := Issue{Severity: Error, Path: "name", Message: "is required"}
	if got := i.String(); got != "error: name: is required" {
		t.Errorf("got %q", got)
	}

	i2 := Issue{Severity: Warning, Message: "something"}
	if got := i2.String(); got != "warning: something" {
		t.Errorf("got %q", got)
	}
}

func TestSeverityString(t *testing.T) {
	if Error.String() != "error" {
		t.Errorf("Error.String() = %q", Error.String())
	}
	if Warning.String() != "warning" {
		t.Errorf("Warning.String() = %q", Warning.String())
	}
}

func TestDocTypeString(t *testing.T) {
	tests := []struct {
		dt   DocType
		want string
	}{
		{PluginDoc, "plugin"},
		{MappingDoc, "mapping"},
		{Unknown, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.dt.String(); got != tt.want {
			t.Errorf("%v.String() = %q, want %q", tt.dt, got, tt.want)
		}
	}
}

func assertHasError(t *testing.T, r *Result, path, substr string) {
	t.Helper()
	for _, e := range r.Errors() {
		if (path == "" || e.Path == path) && contains(e.Message, substr) {
			return
		}
	}
	t.Errorf("expected error at path %q containing %q; got errors: %v", path, substr, r.Errors())
}

func assertHasWarning(t *testing.T, r *Result, path, substr string) {
	t.Helper()
	for _, w := range r.Warnings() {
		if (path == "" || w.Path == path) && contains(w.Message, substr) {
			return
		}
	}
	t.Errorf("expected warning at path %q containing %q; got warnings: %v", path, substr, r.Warnings())
}

func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
