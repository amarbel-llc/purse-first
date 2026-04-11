package command

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// RunCLI parses CLI arguments, dispatches to the matched command handler,
// and prints the result. Global params (App.Params) are parsed before
// the subcommand name; command params and global params are both accepted
// after. Prefix subcommands joined by hyphens are resolved from
// space-separated args (e.g. "perms check" → "perms-check").
//
// The flags -h, --help, and the bare word "help" print usage and return
// without invoking any handler.
func (a *App) RunCLI(ctx context.Context, args []string, p Prompter) error {
	globalVals := make(map[string]any)
	remaining, err := parseFlags(args, a.Params, globalVals)
	if err != nil {
		return fmt.Errorf("parsing global flags: %w", err)
	}

	if len(remaining) == 0 {
		a.printUsage()
		return nil
	}

	if isHelpArg(remaining[0]) {
		a.printUsage()
		return nil
	}

	name := remaining[0]
	cmdArgs := remaining[1:]

	cmd, ok := a.GetCommand(name)
	if ok {
		// Resolve deeper prefix subcommands: "mcp stdio" → "mcp-stdio"
		for len(cmdArgs) > 0 && !strings.HasPrefix(cmdArgs[0], "-") {
			deeper := name + "-" + cmdArgs[0]
			if deeperCmd, found := a.GetCommand(deeper); found {
				name = deeper
				cmd = deeperCmd
				cmdArgs = cmdArgs[1:]
			} else {
				break
			}
		}
	} else {
		// Try joining with subsequent args for prefix subcommands:
		// "perms check" → "perms-check"
		for i := 1; i < len(remaining); i++ {
			name = name + "-" + remaining[i]
			if cmd, ok = a.GetCommand(name); ok {
				cmdArgs = remaining[i+1:]
				break
			}
		}
		if !ok {
			return fmt.Errorf("unknown command: %s", remaining[0])
		}
	}

	if hasHelpFlag(cmdArgs) {
		a.printCommandUsage(name, cmd)
		return nil
	}

	if cmd.PassthroughArgs {
		argsJSON, err := json.Marshal(map[string]any{"args": cmdArgs})
		if err != nil {
			return fmt.Errorf("marshaling passthrough args: %w", err)
		}
		if cmd.RunCLI != nil {
			return cmd.RunCLI(ctx, argsJSON)
		}
		if cmd.Run != nil {
			result, err := cmd.Run(ctx, argsJSON, p)
			if err != nil {
				return err
			}
			printResult(result)
			return nil
		}
		return fmt.Errorf("command %s has no handler", name)
	}

	cmdVals := make(map[string]any)
	for k, v := range globalVals {
		cmdVals[k] = v
	}

	// Merge command params and global params so flags after the subcommand
	// can include global params like --format.
	allParams := append(cmd.Params, a.Params...)
	positional, err := parseFlags(cmdArgs, allParams, cmdVals)
	if err != nil {
		return fmt.Errorf("parsing flags for %s: %w", name, err)
	}

	// Assign positional args to command params that weren't set by flags,
	// in declaration order.
	if len(positional) > 0 {
		pi := 0
		for _, param := range cmd.Params {
			if pi >= len(positional) {
				break
			}
			if _, set := cmdVals[param.Name]; set {
				continue
			}
			if param.Type == Bool {
				continue
			}
			cmdVals[param.Name] = positional[pi]
			pi++
		}
	}

	argsJSON, err := json.Marshal(cmdVals)
	if err != nil {
		return fmt.Errorf("marshaling args: %w", err)
	}

	if cmd.RunCLI != nil {
		return cmd.RunCLI(ctx, argsJSON)
	}

	if cmd.Run != nil {
		result, err := cmd.Run(ctx, argsJSON, p)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	}

	// Commands with subcommands but no handler show usage.
	prefix := name + "-"
	for n := range a.commands {
		if strings.HasPrefix(n, prefix) {
			a.printCommandUsage(name, cmd)
			return nil
		}
	}

	return fmt.Errorf("command %s has no handler", name)
}

func printResult(r *Result) {
	if r == nil {
		return
	}
	if r.JSON != nil {
		data, _ := json.MarshalIndent(r.JSON, "", "  ")
		fmt.Println(string(data))
	} else if r.Text != "" {
		fmt.Println(r.Text)
	}
}

func isHelpArg(s string) bool {
	return s == "-h" || s == "--help" || s == "help"
}

func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

func (a *App) printCommandUsage(name string, cmd *Command) {
	displayName := strings.ReplaceAll(name, "-", " ")
	fmt.Printf("%s %s — %s\n\n", a.Name, displayName, cmd.Description.Short)
	if cmd.Description.Long != "" {
		fmt.Printf("%s\n\n", cmd.Description.Long)
	}
	if len(cmd.Params) > 0 {
		fmt.Println("Options:")
		for _, p := range cmd.Params {
			flag := fmt.Sprintf("--%s", p.Name)
			if p.Short != 0 {
				flag = fmt.Sprintf("-%c, --%s", p.Short, p.Name)
			}
			fmt.Printf("  %-24s %s\n", flag, p.Description)
		}
	}

	// List subcommands (commands starting with name-)
	prefix := name + "-"
	var subs []sortedCommand
	for n, c := range a.VisibleCommands() {
		if strings.HasPrefix(n, prefix) {
			subs = append(subs, sortedCommand{strings.TrimPrefix(n, prefix), c})
		}
	}
	if len(subs) > 0 {
		sort.Slice(subs, func(i, j int) bool {
			return subs[i].name < subs[j].name
		})
		if len(cmd.Params) > 0 {
			fmt.Println()
		}
		fmt.Println("Subcommands:")
		for _, s := range subs {
			fmt.Printf("  %-16s %s\n", s.name, s.cmd.Description.Short)
		}
	}
}

func (a *App) printUsage() {
	fmt.Printf("%s — %s\n\n", a.Name, a.Description.Short)
	if a.Description.Long != "" {
		fmt.Printf("%s\n\n", a.Description.Long)
	}

	cmds := a.sortedVisibleCommands()

	// Identify group prefixes: commands whose name is a prefix of other commands.
	groups := make(map[string]bool)
	for _, e := range cmds {
		for _, other := range cmds {
			if strings.HasPrefix(other.name, e.name+"-") {
				groups[e.name] = true
				break
			}
		}
	}

	fmt.Println("Commands:")
	for _, e := range cmds {
		// Hide children of group commands from top-level listing.
		isChild := false
		for g := range groups {
			if strings.HasPrefix(e.name, g+"-") {
				isChild = true
				break
			}
		}
		if isChild {
			continue
		}
		fmt.Printf("  %-16s %s\n", e.name, e.cmd.Description.Short)
	}
}

// flagLabel returns a user-facing label for the flag the user typed.
// When the user typed a short flag like -c, it returns "-c (--count)".
// When they typed --count, it returns "--count".
func flagLabel(arg string, key string) string {
	if strings.HasPrefix(arg, "--") {
		return "--" + key
	}
	// Short flag: show what was typed plus the long name for context.
	shortPart := arg
	if idx := strings.IndexByte(shortPart, '='); idx >= 0 {
		shortPart = shortPart[:idx]
	}
	return fmt.Sprintf("%s (--%s)", shortPart, key)
}

// parseFlags extracts --flag and -x values from args into vals, returning
// unconsumed positional args. Non-flag args are collected but parsing
// continues, so flags can appear after positional args (e.g. "open target
// --format tap"). Short flags (-x) are resolved to their param name.
func parseFlags(args []string, params []Param, vals map[string]any) ([]string, error) {
	paramMap := make(map[string]Param)
	shortMap := make(map[rune]Param)
	for _, p := range params {
		paramMap[p.Name] = p
		if p.Short != 0 {
			shortMap[p.Short] = p
		}
	}

	var remaining []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		var p Param
		var key, value string
		var hasEquals, found bool

		switch {
		case strings.HasPrefix(arg, "--"):
			key = strings.TrimPrefix(arg, "--")
			if idx := strings.IndexByte(key, '='); idx >= 0 {
				value = key[idx+1:]
				key = key[:idx]
				hasEquals = true
			}
			p, found = paramMap[key]

		case strings.HasPrefix(arg, "-") && len(arg) >= 2:
			// Parse -x or -x=value or -long-flag; resolve short or long name.
			rest := arg[1:]
			if idx := strings.IndexByte(rest, '='); idx >= 0 {
				value = rest[idx+1:]
				rest = rest[:idx]
				hasEquals = true
			}
			short := []rune(rest)
			if len(short) == 1 {
				p, found = shortMap[short[0]]
				if found {
					key = p.Name
				}
			} else {
				// Multi-char single-dash: treat as long flag name (Go convention)
				key = rest
				p, found = paramMap[key]
			}

		default:
			remaining = append(remaining, arg)
			continue
		}

		if !found {
			remaining = append(remaining, arg)
			continue
		}

		label := flagLabel(arg, key)

		switch p.Type {
		case Bool:
			if hasEquals {
				vals[key] = value != "false"
			} else {
				vals[key] = true
			}
		case Int:
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag %s requires a value", label)
				}
				value = args[i]
			}
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("flag %s: invalid integer %q", label, value)
			}
			vals[key] = n
		case Float:
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag %s requires a value", label)
				}
				value = args[i]
			}
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("flag %s: invalid number %q", label, value)
			}
			vals[key] = f
		case Array:
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag %s requires a value", label)
				}
				value = args[i]
			}
			arr, _ := vals[key].([]string)
			vals[key] = append(arr, value)
		default: // String
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag %s requires a value", label)
				}
				value = args[i]
			}
			vals[key] = value
		}
	}

	return remaining, nil
}
