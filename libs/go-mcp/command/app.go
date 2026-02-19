package command

import "fmt"

// App holds the command registry and top-level metadata for a CLI/MCP application.
type App struct {
	Name        string
	Description Description
	Version     string
	MCPArgs     []string // extra args passed to the binary in plugin manifests
	MCPBinary   string   // binary name for plugin.json command; defaults to Name
	Params      []Param  // global flags
	Examples    []Example // app-level workflow examples
	commands       map[string]*Command
	canonicalNames map[*Command]string
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

	return a
}

// AddCommand registers a command and its aliases. Panics on duplicate names.
func (a *App) AddCommand(cmd *Command) {
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
