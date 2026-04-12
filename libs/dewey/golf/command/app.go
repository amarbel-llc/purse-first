package command

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/dewey/bravo/errors"
	"github.com/amarbel-llc/purse-first/libs/dewey/golf/protocol"
)

// Utility holds the command registry and top-level metadata for a CLI/MCP application.
type Utility struct {
	Name              string
	Aliases           []string // Aliases are additional binary names that should get shell completions
	Description       Description
	Version           string
	MCPArgs           []string  // extra args passed to the binary in plugin manifests
	MCPBinary         string    // binary name for plugin.json command; defaults to Name
	PluginDescription string    // "description" in plugin.json; omitted if empty
	PluginAuthor      string    // "author.name" in plugin.json; omitted if empty
	OldParams         []OldParam // global flags
	Examples          []Example // app-level workflow examples

	// EnvVars are environment variables the app as a whole reads, rendered
	// into the app manpage's ENVIRONMENT section.
	EnvVars []EnvVar

	// Files are filesystem paths the app as a whole reads or writes, rendered
	// into the app manpage's FILES section.
	Files []FilePath

	// ExtraManpages are hand-written manpage source files (any roff dialect)
	// to install alongside the auto-generated pages. Each entry is read from
	// its Source fs.FS and written verbatim to share/man/man{Section}/{Name}.
	// The framework does not parse, validate, or modify these files —
	// authors choose any dialect (man(7), mdoc(7), or pre-rendered output
	// from scdoc/ronn/asciidoctor).
	ExtraManpages []ManpageFile

	commands       map[string]*Command
	canonicalNames map[*Command]string
	pluginSkills   []string // discovered skill paths for plugin.json
}

// NewUtility creates a new Utility with the given name and short description.
func NewUtility(name, short string) *Utility {
	u := &Utility{
		Name:           name,
		Description:    Description{Short: short},
		commands:       make(map[string]*Command),
		canonicalNames: make(map[*Command]string),
	}

	u.addDevMCPCommand()
	u.addCompleteCommand()

	return u
}

// AddCommand registers a command and its aliases. Panics on duplicate names
// or if any command param's Short rune conflicts with a global param's Short rune.
func (u *Utility) AddCommand(cmd *Command) {
	// Check for short flag collisions between command params and global params.
	for _, gp := range u.OldParams {
		if gp.Short == 0 {
			continue
		}
		for _, cp := range cmd.OldParams {
			if cp.Short == gp.Short {
				panic(fmt.Sprintf(
					"short flag -%c on command %q param %q conflicts with global param %q",
					cp.Short, cmd.Name, cp.Name, gp.Name,
				))
			}
		}
	}

	// Check for duplicate short flags within the command's own params.
	shortSeen := make(map[rune]string)
	for _, cp := range cmd.OldParams {
		if cp.Short == 0 {
			continue
		}
		if existing, ok := shortSeen[cp.Short]; ok {
			panic(fmt.Sprintf(
				"duplicate short flag -%c: used by both %q and %q",
				cp.Short, existing, cp.Name,
			))
		}
		shortSeen[cp.Short] = cp.Name
	}

	u.addName(cmd.Name, cmd)
	for _, alias := range cmd.Aliases {
		u.addName(alias, cmd)
	}
}

func (u *Utility) addName(name string, cmd *Command) {
	if _, ok := u.commands[name]; ok {
		panic(fmt.Sprintf("command added more than once: %s", name))
	}
	u.commands[name] = cmd
	if _, ok := u.canonicalNames[cmd]; !ok {
		u.canonicalNames[cmd] = name
	}
}

// GetCommand looks up a command by name or alias.
func (u *Utility) GetCommand(name string) (*Command, bool) {
	cmd, ok := u.commands[name]
	return cmd, ok
}

// AllCommands iterates over all registered commands (including hidden).
// Each unique command is yielded once even if it has aliases.
func (u *Utility) AllCommands() func(yield func(string, *Command) bool) {
	return func(yield func(string, *Command) bool) {
		seen := make(map[*Command]bool)
		for _, cmd := range u.commands {
			if seen[cmd] {
				continue
			}
			seen[cmd] = true
			if !yield(u.canonicalNames[cmd], cmd) {
				return
			}
		}
	}
}

// VisibleCommands iterates over non-hidden commands.
func (u *Utility) VisibleCommands() func(yield func(string, *Command) bool) {
	return func(yield func(string, *Command) bool) {
		for name, cmd := range u.AllCommands() {
			if cmd.Hidden {
				continue
			}
			if !yield(name, cmd) {
				return
			}
		}
	}
}

// AddCmd wraps a dodder-style Cmd into a *Command and registers it.
// Metadata is extracted from opt-in interfaces:
//   - CommandWithDescription → Command.Description
//   - CommandWithParams → Command.Params
//   - CommandWithMCPAnnotations → Command.Annotations
//   - CommandWithResult → Command.Run (enables MCP tool registration)
//
// Commands implementing only Cmd (not CommandWithResult) are CLI-only.
func (u *Utility) AddCmd(name string, cmd Cmd) {
	wrapped := &Command{
		Name: name,
	}

	if cwp, ok := cmd.(CommandWithDescription); ok {
		wrapped.Description = cwp.GetDescription()
	}

	if cwp, ok := cmd.(CommandWithParams); ok {
		wrapped.Params = cwp.GetParams()
	}

	if cwa, ok := cmd.(CommandWithMCPAnnotations); ok {
		ann := cwa.GetMCPAnnotations()
		wrapped.Annotations = &protocol.ToolAnnotations{
			ReadOnlyHint:    &ann.ReadOnly,
			DestructiveHint: &ann.Destructive,
		}
	}

	if cwr, ok := cmd.(CommandWithResult); ok {
		wrapped.Run = func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			errCtx := errors.MakeContextDefault()
			req := Request{
				Context:  errCtx,
				Prompter: p,
			}
			return cwr.RunResult(req)
		}
	}

	u.AddCommand(wrapped)
}

// MergeWithPrefix adds all commands from another Utility, prefixed with the given string.
func (u *Utility) MergeWithPrefix(other *Utility, prefix string) {
	for name, cmd := range other.AllCommands() {
		key := name
		if prefix != "" {
			key = prefix + "-" + name
		}
		u.addName(key, cmd)
		u.canonicalNames[cmd] = key
	}
}
