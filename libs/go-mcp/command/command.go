package command

import (
	"context"
	"encoding/json"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
)

// ParamType identifies the data type of a command parameter.
type ParamType int

const (
	String ParamType = iota
	Int
	Bool
	Float
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
	default:
		return "string"
	}
}

// Description holds short and long descriptions for a command.
type Description struct {
	Short string // one-line: manpage NAME, completion tab text, MCP tool description
	Long  string // paragraph: manpage DESCRIPTION, --help output
}

// BashMapping declares a bash command prefix that should be intercepted
// and redirected to this command's MCP tool.
type BashMapping struct {
	Prefixes []string // e.g., "git status"
	UseWhen  string   // shown to Claude in mapping denial
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
	MapsBash []BashMapping

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
