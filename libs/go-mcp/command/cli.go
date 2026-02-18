package command

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// RunCLI parses CLI arguments, dispatches to the matched command handler,
// and prints the result. Global params (App.Params) are parsed before
// the subcommand name; command params and global params are both accepted
// after. Prefix subcommands joined by hyphens are resolved from
// space-separated args (e.g. "perms check" → "perms-check").
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

	name := remaining[0]
	cmdArgs := remaining[1:]

	cmd, ok := a.GetCommand(name)
	if !ok {
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

func (a *App) printUsage() {
	fmt.Printf("%s — %s\n\n", a.Name, a.Description.Short)
	if a.Description.Long != "" {
		fmt.Printf("%s\n\n", a.Description.Long)
	}
	fmt.Println("Commands:")
	for name, cmd := range a.VisibleCommands() {
		fmt.Printf("  %-16s %s\n", name, cmd.Description.Short)
	}
}

// parseFlags extracts --flag values from args into vals, returning unconsumed
// positional args. Non-flag args are collected but parsing continues, so
// flags can appear after positional args (e.g. "open target --format tap").
func parseFlags(args []string, params []Param, vals map[string]any) ([]string, error) {
	paramMap := make(map[string]Param)
	for _, p := range params {
		paramMap[p.Name] = p
	}

	var remaining []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if !strings.HasPrefix(arg, "--") {
			remaining = append(remaining, arg)
			continue
		}

		key := strings.TrimPrefix(arg, "--")
		var value string
		hasEquals := false

		if idx := strings.IndexByte(key, '='); idx >= 0 {
			value = key[idx+1:]
			key = key[:idx]
			hasEquals = true
		}

		p, ok := paramMap[key]
		if !ok {
			remaining = append(remaining, arg)
			continue
		}

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
					return nil, fmt.Errorf("flag --%s requires a value", key)
				}
				value = args[i]
			}
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("flag --%s: invalid integer %q", key, value)
			}
			vals[key] = n
		case Float:
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag --%s requires a value", key)
				}
				value = args[i]
			}
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("flag --%s: invalid number %q", key, value)
			}
			vals[key] = f
		case Array:
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag --%s requires a value", key)
				}
				value = args[i]
			}
			arr, _ := vals[key].([]string)
			vals[key] = append(arr, value)
		default: // String
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag --%s requires a value", key)
				}
				value = args[i]
			}
			vals[key] = value
		}
	}

	return remaining, nil
}
