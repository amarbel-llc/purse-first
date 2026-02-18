package command

import (
	"context"
	"encoding/json"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
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
	Type        ParamType
	Description string
	Required    bool
	Default     any
	Completer   func() map[string]string
}

// Command declares a single subcommand with all metadata needed
// to generate CLI parsing, MCP tool registration, manpages,
// completions, and plugin manifests.
type Command struct {
	Name        string
	Aliases     []string
	Description Description
	Hidden      bool

	Params   []Param
	MapsTools []ToolMapping

	// RunMCP handles MCP tool invocations.
	RunMCP func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error)
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
