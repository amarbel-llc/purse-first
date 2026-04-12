package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// addCompleteCommand registers the hidden __complete subcommand that provides
// dynamic value completions at tab-completion time. Shell completion scripts
// call this to get completions for params that have a Completer function.
//
// Usage: appname __complete --command <subcmd> --param <paramname>
// Output: tab-separated "value\tdescription" lines, one per completion candidate.
func (u *Utility) addCompleteCommand() {
	u.AddCommand(&Command{
		Name:   "__complete",
		Hidden: true,
		OldParams: []OldParam{
			{Name: "command", Type: String, Required: true, Description: "Subcommand name"},
			{Name: "param", Type: String, Required: true, Description: "Parameter name"},
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			var params struct {
				Command string `json:"command"`
				Param   string `json:"param"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return fmt.Errorf("parsing __complete args: %w", err)
			}

			cmd, ok := u.GetCommand(params.Command)
			if !ok {
				return nil // unknown command, no completions
			}

			// Check old params
			for _, p := range cmd.OldParams {
				if p.Name == params.Param && p.Completer != nil {
					completions := p.Completer()
					printOldCompletions(completions)
					return nil
				}
			}

			// Check new params
			for _, p := range cmd.Params {
				if f, ok := p.(interface{ flagCompleter() ParamCompleter }); ok {
					if p.paramName() == params.Param {
						if c := f.flagCompleter(); c != nil {
							printCompletions(c)
							return nil
						}
					}
				}
			}

			return nil // param not found or no completer, no output
		},
	})
}

// printCompletions writes completion candidates from an iterator to stdout.
func printCompletions(completions ParamCompleter) {
	for c := range completions {
		if c.Description != "" {
			fmt.Fprintf(os.Stdout, "%s\t%s\n", c.Value, c.Description)
		} else {
			fmt.Fprintln(os.Stdout, c.Value)
		}
	}
}

// printOldCompletions writes completion candidates from a map to stdout.
// Deprecated: used by OldParam completers.
func printOldCompletions(completions map[string]string) {
	if len(completions) == 0 {
		return
	}

	for k, desc := range completions {
		if desc != "" {
			fmt.Fprintf(os.Stdout, "%s\t%s\n", k, desc)
		} else {
			fmt.Fprintln(os.Stdout, k)
		}
	}
}
