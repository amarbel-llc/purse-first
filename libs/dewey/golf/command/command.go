package command

import (
	"context"
	"encoding/json"
	"io/fs"

	"github.com/amarbel-llc/purse-first/libs/dewey/golf/protocol"
)

// EnvVar declares an environment variable that an app or command reads,
// for inclusion in the manpage ENVIRONMENT section.
type EnvVar struct {
	Name        string // variable name, e.g. "LUX_SOCKET"
	Description string // one-paragraph description (plain text)
	Default     string // optional; rendered as "Default: ..." when non-empty
}

// FilePath declares a file or directory path that an app or command reads
// or writes, for inclusion in the manpage FILES section.
type FilePath struct {
	Path        string // filesystem path, e.g. "$XDG_CONFIG_HOME/lux"
	Description string // one-paragraph description (plain text)
}

// ManpageFile declares a hand-written manpage source file (any roff dialect)
// to be installed alongside the auto-generated pages produced by
// GenerateManpages. The framework reads bytes from Source and writes them
// verbatim to {dir}/share/man/man{Section}/{Name}.
//
// Source may be any fs.FS — typically an embed.FS for binary-bundled docs,
// or os.DirFS(".") for paths relative to the package source root (which is
// the convention for nix postInstall steps).
type ManpageFile struct {
	Source  fs.FS  // filesystem to read from; required
	Path    string // path within Source; required
	Section int    // man section number, e.g. 1, 5, 7; required
	Name    string // installed filename, e.g. "lux-config.5"; required
}

// ParamType identifies the data type of a command parameter.
type ParamType int

const (
	String ParamType = iota
	Int
	Bool
	Float
	Array
	Object
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
	case Object:
		return "object"
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

// OldParam declares a single command parameter, used for CLI flags,
// MCP JSON schema properties, manpage OPTIONS, and completions.
//
// Deprecated: use Flag[V], Arg[V], ArrayFlag, or ObjectFlag instead.
type OldParam struct {
	Name        string
	Short       rune // single-character CLI alias (e.g. 'v' for -v); zero means none
	Type        ParamType
	Description string
	Required    bool
	Default     any
	Completer   func() map[string]string
	Items       []OldParam // item schema for Array params (generates object items with properties)
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

	// Title is a human-readable display name for the MCP tool (V1).
	Title string

	// Annotations provides V1 behavior hints (readOnly, destructive, etc.).
	Annotations *protocol.ToolAnnotations

	// Execution describes task execution support for this tool.
	Execution *protocol.ToolExecution

	// Params declares parameters using the new sealed Param interface
	// (Flag[V], Arg[V], ArrayFlag, ObjectFlag). Used by InputSchema()
	// and CLI dispatch when non-empty.
	Params []Param

	// OldParams is the deprecated flat-struct parameter list.
	// Used by generators until they migrate to the Param interface.
	OldParams []OldParam

	MapsTools []ToolMapping
	Examples  []Example

	// EnvVars are environment variables this command reads, rendered into
	// the per-command manpage's ENVIRONMENT section.
	EnvVars []EnvVar

	// Files are filesystem paths this command reads or writes, rendered into
	// the per-command manpage's FILES section.
	Files []FilePath

	// SeeAlso lists related command page names (e.g. "lux-definition",
	// "lux-references") rendered into the per-command manpage's SEE ALSO
	// section alongside the automatic back-reference to the parent app page.
	SeeAlso []string

	// componentFlags holds OldParam entries collected from SetFlagDefinitions
	// at AddCmd registration time. RunCLI merges them into the parseFlags
	// param list so flags registered via the Go flag pattern are visible.
	// Used only by legacy (non-AddCmd) commands; AddCmd commands use
	// runFromCLI which parses via FlagSet directly.
	componentFlags []OldParam

	// runFromCLI handles CLI dispatch for AddCmd commands without JSON
	// serialization. Receives raw args after subcommand resolution.
	// Set by AddCmd. RunCLI prefers this over Run when set.
	runFromCLI func(ctx context.Context, args []string, p Prompter) (*Result, error)

	// PassthroughArgs disables flag parsing for this command. All arguments
	// after the command name are passed raw as {"args": [...]} to the handler.
	// Passthrough commands appear in help, manpages, and completions but have
	// no individual flag completions.
	PassthroughArgs bool

	// Run handles both MCP tool invocations and CLI execution.
	// In MCP mode, Prompter is a StubPrompter that returns errors.
	// In CLI mode, Prompter is a real interactive implementation.
	Run func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error)

	// RunCLI handles CLI-only invocations. Commands with only RunCLI
	// are not registered as MCP tools or included in plugin.json.
	RunCLI func(ctx context.Context, args json.RawMessage) error
}

// RequiredParams returns only the new Param entries marked as required.
func (c *Command) RequiredParams() []Param {
	var out []Param
	for _, p := range c.Params {
		if p.paramRequired() {
			out = append(out, p)
		}
	}
	return out
}

// OptionalParams returns only the new Param entries not marked as required.
func (c *Command) OptionalParams() []Param {
	var out []Param
	for _, p := range c.Params {
		if !p.paramRequired() {
			out = append(out, p)
		}
	}
	return out
}

// RequiredOldParams returns only the old params marked as required.
//
// Deprecated: will be replaced when Command.OldParams migrates to Param interface.
func (c *Command) RequiredOldParams() []OldParam {
	var out []OldParam
	for _, p := range c.OldParams {
		if p.Required {
			out = append(out, p)
		}
	}
	return out
}

// OptionalOldParams returns only the old params not marked as required.
//
// Deprecated: will be replaced when Command.OldParams migrates to Param interface.
func (c *Command) OptionalOldParams() []OldParam {
	var out []OldParam
	for _, p := range c.OldParams {
		if !p.Required {
			out = append(out, p)
		}
	}
	return out
}
