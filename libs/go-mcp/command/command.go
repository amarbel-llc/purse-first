package command

import (
	"context"
	"encoding/json"
)

// ParamType identifies the data type of a command parameter.
type ParamType int

const (
	String ParamType = iota
	Int
	Bool
	Float
	Array
)

// JSONSchemaType returns the JSON Schema type name for this ParamType.
func (pt ParamType) JSONSchemaType() string {
	switch pt {
	case Int:
		return "integer"
	case Bool:
		return "boolean"
	case Float:
		return "number"
	case Array:
		return "array"
	default:
		return "string"
	}
}

// Description holds short and long descriptions for a command.
type Description struct {
	Short string // one-line: manpage NAME, completion tab text, MCP tool description
	Long  string // paragraph: manpage DESCRIPTION, --help output
}

// ToolMapping declares that this command's MCP tool should intercept
// a specific Claude Code tool under certain conditions.
type ToolMapping struct {
	Replaces        string   // Claude Code tool to intercept: "Read", "Grep", "Glob", "Bash"
	Extensions      []string // file extensions to match, e.g. [".go", ".py"]
	CommandPrefixes []string // bash command prefixes, e.g. ["git status"]
	UseWhen         string   // shown to Claude in denial reason
}

// Param declares a single command parameter, used for CLI flags,
// MCP JSON schema properties, manpage OPTIONS, and completions.
type Param struct {
	Name        string
	Short       rune // single-character CLI alias (e.g. 'v' for -v); zero means none
	Type        ParamType
	Description string
	Required    bool
	Default     any
	Completer   func() map[string]string
}

// Example represents a single usage example for a command or app.
type Example struct {
	Description string // what this example demonstrates
	Command     string // shell invocation (may be multi-line)
	Output      string // optional expected output snippet
}

// Command declares a single subcommand with all metadata needed
// to generate CLI parsing, MCP tool registration, manpages,
// completions, and plugin manifests.
type Command struct {
	Name        string
	Aliases     []string
	Description Description
	Hidden      bool

	Params    []Param
	MapsTools []ToolMapping
	Examples  []Example

	// Run handles both MCP tool invocations and CLI execution.
	// In MCP mode, Prompter is a StubPrompter that returns errors.
	// In CLI mode, Prompter is a real interactive implementation.
	Run func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error)

	// RunCLI handles CLI-only invocations. Commands with only RunCLI
	// are not registered as MCP tools or included in plugin.json.
	RunCLI func(ctx context.Context, args json.RawMessage) error
}

// RequiredParams returns only the params marked as required.
func (c *Command) RequiredParams() []Param {
	var out []Param
	for _, p := range c.Params {
		if p.Required {
			out = append(out, p)
		}
	}
	return out
}

// OptionalParams returns only the params not marked as required.
func (c *Command) OptionalParams() []Param {
	var out []Param
	for _, p := range c.Params {
		if !p.Required {
			out = append(out, p)
		}
	}
	return out
}
