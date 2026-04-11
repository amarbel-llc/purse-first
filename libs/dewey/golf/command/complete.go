package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// addCompleteCommand registers the hidden __complete subcommand that provides
// dynamic value completions at tab-completion time. Shell completion scripts
// call this to get completions for params that have a Completer function.
//
// Usage: appname __complete --command <subcmd> --param <paramname>
// Output: tab-separated "value\tdescription" lines, one per completion candidate.
func (a *App) addCompleteCommand() {
	a.AddCommand(&Command{
		Name:   "__complete",
		Hidden: true,
		Params: []Param{
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

			cmd, ok := a.GetCommand(params.Command)
			if !ok {
				return nil // unknown command, no completions
			}

			for _, p := range cmd.Params {
				if p.Name == params.Param && p.Completer != nil {
					completions := p.Completer()
					printCompletions(completions)
					return nil
				}
			}

			return nil // param not found or no completer, no output
		},
	})
}

// printCompletions writes completion candidates as tab-separated lines to stdout.
// Keys are sorted for deterministic output.
func printCompletions(completions map[string]string) {
	if len(completions) == 0 {
		return
	}

	keys := make([]string, 0, len(completions))
	for k := range completions {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		desc := completions[k]
		if desc != "" {
			fmt.Fprintf(os.Stdout, "%s\t%s\n", k, desc)
		} else {
			fmt.Fprintln(os.Stdout, k)
		}
	}
}
