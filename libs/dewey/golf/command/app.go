package command

import "fmt"

// App holds the command registry and top-level metadata for a CLI/MCP application.
type App struct {
	Name              string
	Aliases           []string // Aliases are additional binary names that should get shell completions
	Description       Description
	Version           string
	MCPArgs           []string  // extra args passed to the binary in plugin manifests
	MCPBinary         string    // binary name for plugin.json command; defaults to Name
	PluginDescription string    // "description" in plugin.json; omitted if empty
	PluginAuthor      string    // "author.name" in plugin.json; omitted if empty
	Params            []Param   // global flags
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

// NewApp creates a new App with the given name and short description.
func NewApp(name, short string) *App {
	a := &App{
		Name:           name,
		Description:    Description{Short: short},
		commands:       make(map[string]*Command),
		canonicalNames: make(map[*Command]string),
	}

	a.addDevMCPCommand()
	a.addCompleteCommand()

	return a
}

// AddCommand registers a command and its aliases. Panics on duplicate names
// or if any command param's Short rune conflicts with a global param's Short rune.
func (a *App) AddCommand(cmd *Command) {
	// Check for short flag collisions between command params and global params.
	for _, gp := range a.Params {
		if gp.Short == 0 {
			continue
		}
		for _, cp := range cmd.Params {
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
	for _, cp := range cmd.Params {
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

	a.addName(cmd.Name, cmd)
	for _, alias := range cmd.Aliases {
		a.addName(alias, cmd)
	}
}

func (a *App) addName(name string, cmd *Command) {
	if _, ok := a.commands[name]; ok {
		panic(fmt.Sprintf("command added more than once: %s", name))
	}
	a.commands[name] = cmd
	if _, ok := a.canonicalNames[cmd]; !ok {
		a.canonicalNames[cmd] = name
	}
}

// GetCommand looks up a command by name or alias.
func (a *App) GetCommand(name string) (*Command, bool) {
	cmd, ok := a.commands[name]
	return cmd, ok
}

// AllCommands iterates over all registered commands (including hidden).
// Each unique command is yielded once even if it has aliases.
func (a *App) AllCommands() func(yield func(string, *Command) bool) {
	return func(yield func(string, *Command) bool) {
		seen := make(map[*Command]bool)
		for _, cmd := range a.commands {
			if seen[cmd] {
				continue
			}
			seen[cmd] = true
			if !yield(a.canonicalNames[cmd], cmd) {
				return
			}
		}
	}
}

// VisibleCommands iterates over non-hidden commands.
func (a *App) VisibleCommands() func(yield func(string, *Command) bool) {
	return func(yield func(string, *Command) bool) {
		for name, cmd := range a.AllCommands() {
			if cmd.Hidden {
				continue
			}
			if !yield(name, cmd) {
				return
			}
		}
	}
}

// MergeWithPrefix adds all commands from another App, prefixed with the given string.
func (a *App) MergeWithPrefix(other *App, prefix string) {
	for name, cmd := range other.AllCommands() {
		key := name
		if prefix != "" {
			key = prefix + "-" + name
		}
		a.addName(key, cmd)
		a.canonicalNames[cmd] = key
	}
}
