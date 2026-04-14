package command

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/dewey/0/interfaces"
	"github.com/amarbel-llc/purse-first/libs/dewey/bravo/collections_slice"
	"github.com/amarbel-llc/purse-first/libs/dewey/bravo/errors"
	"github.com/amarbel-llc/purse-first/libs/dewey/charlie/flags"
	"github.com/amarbel-llc/purse-first/libs/dewey/golf/protocol"
)

// Utility holds the command registry and top-level metadata for a CLI/MCP application.
type Utility struct {
	Name              string
	Aliases           []string // Aliases are additional binary names that should get shell completions
	Description       Description
	Version           string
	MCPArgs           []string   // extra args passed to the binary in plugin manifests
	MCPBinary         string     // binary name for plugin.json command; defaults to Name
	PluginDescription string     // "description" in plugin.json; omitted if empty
	PluginAuthor      string     // "author.name" in plugin.json; omitted if empty
	OldParams         []OldParam // global flags
	Examples          []Example  // app-level workflow examples

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

// GetName returns the utility's name.
func (u *Utility) GetName() string {
	return u.Name
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

	ccw, hasComponentFlags := cmd.(interfaces.CommandComponentWriter)

	// Collect flag definitions for legacy parseFlags path (componentFlags)
	// and for the runFromCLI FlagSet registration.
	if hasComponentFlags {
		collector := &flagDefCollector{}
		ccw.SetFlagDefinitions(collector)
		wrapped.componentFlags = collector.params
	}

	// runFromCLI: CLI dispatch without JSON serialization.
	// FlagSet is the sole parsing backend.
	wrapped.runFromCLI = func(ctx context.Context, args []string, p Prompter) (*Result, error) {
		errCtx := errors.MakeContextDefault()

		// 1. Create FlagSet and register ALL flags.
		fs := flags.NewFlagSet(name, flags.ContinueOnError)

		// Register GetParams() flags as temporary variables.
		type paramFlag struct {
			name string
			str  *string
			b    *bool
			i    *int
		}
		var paramFlags []paramFlag

		for _, param := range wrapped.Params {
			if param.isPositional() {
				continue
			}
			pn := param.paramName()
			short := param.paramShort()
			switch param.jsonSchemaType() {
			case "boolean":
				b := new(bool)
				defVal := false
				if d, ok := param.paramDefault().(bool); ok {
					defVal = d
				}
				fs.BoolVar(b, pn, defVal, param.paramDescription())
				if short != 0 {
					fs.BoolVar(b, string(short), defVal, param.paramDescription())
				}
				paramFlags = append(paramFlags, paramFlag{name: pn, b: b})
			case "integer":
				i := new(int)
				defVal := 0
				if d, ok := param.paramDefault().(int); ok {
					defVal = d
				}
				fs.IntVar(i, pn, defVal, param.paramDescription())
				if short != 0 {
					fs.IntVar(i, string(short), defVal, param.paramDescription())
				}
				paramFlags = append(paramFlags, paramFlag{name: pn, i: i})
			default: // string
				s := new(string)
				defVal := ""
				if d, ok := param.paramDefault().(string); ok {
					defVal = d
				}
				fs.StringVar(s, pn, defVal, param.paramDescription())
				if short != 0 {
					fs.StringVar(s, string(short), defVal, param.paramDescription())
				}
				paramFlags = append(paramFlags, paramFlag{name: pn, str: s})
			}
		}

		// Register SetFlagDefinitions flags (pointers into cmd struct).
		if hasComponentFlags {
			ccw.SetFlagDefinitions(fs)
		}

		// Register global flags so they're recognized after the subcommand.
		for _, gp := range u.OldParams {
			switch gp.Type {
			case Bool:
				b := new(bool)
				fs.BoolVar(b, gp.Name, false, gp.Description)
				if gp.Short != 0 {
					fs.BoolVar(b, string(gp.Short), false, gp.Description)
				}
			case Int:
				i := new(int)
				fs.IntVar(i, gp.Name, 0, gp.Description)
				if gp.Short != 0 {
					fs.IntVar(i, string(gp.Short), 0, gp.Description)
				}
			default:
				s := new(string)
				fs.StringVar(s, gp.Name, "", gp.Description)
				if gp.Short != 0 {
					fs.StringVar(s, string(gp.Short), "", gp.Description)
				}
			}
		}

		// 2. Parse.
		if err := fs.Parse(args); err != nil {
			return nil, fmt.Errorf("parsing flags for %s: %w", name, err)
		}
		positional := fs.Args()

		// 3. Build CommandLineInput.Args from params in declaration order.
		var cliArgs collections_slice.String
		pi := 0
		for _, param := range wrapped.Params {
			pn := param.paramName()

			if !param.isPositional() {
				// Flag: read value from FlagSet.
				if f := fs.Lookup(pn); f != nil {
					cliArgs.Append(f.Value.String())
				}
				continue
			}

			// Positional arg.
			if pi >= len(positional) {
				continue
			}
			if param.isVariadic() {
				cliArgs.Append(positional[pi:]...)
				pi = len(positional)
				break
			}
			cliArgs.Append(positional[pi])
			pi++
		}

		input := CommandLineInput{Args: cliArgs}
		req := Request{
			Context:  errCtx,
			Utility:  u,
			Prompter: p,
			FlagSet:  fs,
			input:    &input,
		}

		// 4. Dispatch through errCtx.Run.
		if cwr, ok := cmd.(CommandWithResult); ok {
			var result *Result
			var resultErr error
			err := errCtx.Run(func(_ errors.Context) {
				result, resultErr = cwr.RunResult(req)
			})
			if err != nil {
				return nil, err
			}
			return result, resultErr
		}

		err := errCtx.Run(func(_ errors.Context) {
			cmd.Run(req)
		})
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	// Run: MCP dispatch via JSON (kept for MCP tool invocations).
	wrapped.Run = func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
		errCtx := errors.MakeContextDefault()
		input := makeInputFromJSON(args, wrapped.Params)
		req := Request{
			Context:  errCtx,
			Utility:  u,
			Prompter: p,
			input:    &input,
		}

		// Apply SetFlagDefinitions values from JSON to struct pointers.
		if hasComponentFlags {
			applyComponentFlags(ccw, name, args)
		}

		if cwr, ok := cmd.(CommandWithResult); ok {
			var result *Result
			var resultErr error
			err := errCtx.Run(func(_ errors.Context) {
				result, resultErr = cwr.RunResult(req)
			})
			if err != nil {
				return nil, err
			}
			return result, resultErr
		}

		err := errCtx.Run(func(_ errors.Context) {
			cmd.Run(req)
		})
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	u.AddCommand(wrapped)
}

// applyComponentFlags creates a real FlagSet, registers the command's flags
// via SetFlagDefinitions, then applies values from the parsed JSON args to
// the FlagSet. This sets the command struct's field pointers.
func applyComponentFlags(ccw interfaces.CommandComponentWriter, name string, args json.RawMessage) {
	if len(args) == 0 {
		return
	}

	var vals map[string]json.RawMessage
	if err := json.Unmarshal(args, &vals); err != nil {
		return
	}

	fs := flags.NewFlagSet(name, flags.ContinueOnError)
	ccw.SetFlagDefinitions(fs)

	for flagName, rawVal := range vals {
		if fs.Lookup(flagName) == nil {
			continue
		}
		// Try string unmarshal first (handles JSON "foo").
		// For non-strings (bool, int), use the raw JSON representation
		// which is already the right format for FlagSet.Set().
		var s string
		if err := json.Unmarshal(rawVal, &s); err != nil {
			s = string(rawVal)
		}
		fs.Set(flagName, s)
	}
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
