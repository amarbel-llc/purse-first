package command

import "github.com/amarbel-llc/purse-first/libs/dewey/0/interfaces"

type (
	// OldArg declares metadata for a single positional argument.
	// Mirrors the flag pattern: flagSet.Var(&cmd.RepoId, "repo", "usage")
	// becomes OldArg{Name: "repo-id", Value: &ids.RepoId{}, ...}
	//
	// Deprecated: use Arg[V] from param.go instead.
	OldArg struct {
		// Name is used in synopsis, error messages, and as MCP schema property
		// key. Should match the string passed to PopArg(name).
		Name        string
		Description string
		Required    bool

		// Variadic means this arg consumes all remaining positional arguments.
		// At most one Arg per command may be Variadic, and it must be last.
		Variadic bool

		// EnumValues constrains the arg to listed values. Used for MCP schema
		// enum and shell completion.
		EnumValues []string

		// Value carries type information for schema generation and future
		// auto-parsing. Same interface flags use (StringerSetter). Nil means
		// plain string. When non-nil, the concrete type determines the JSON
		// schema type and Value.Set() provides validation.
		Value interfaces.FlagValue
	}

)

type (
	// MCPAnnotations declares MCP tool hints without importing protocol types
	// directly, keeping the golf layer dependency-free.
	MCPAnnotations struct {
		ReadOnly    bool
		Destructive bool
	}

	// CommandWithMCPAnnotations lets commands declare their MCP hints.
	CommandWithMCPAnnotations interface {
		GetMCPAnnotations() MCPAnnotations
	}
)
