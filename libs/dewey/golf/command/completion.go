package command

import (
	"github.com/amarbel-llc/purse-first/libs/dewey/0/interfaces"
	"github.com/amarbel-llc/purse-first/libs/dewey/delta/cli"
)

type SupportsCompletion interface {
	SupportsCompletion()
}

type CLICompleter = cli.CLICompleter

type Completion struct {
	Value, Description string
}

// Completer is implemented by commands that provide shell completions.
// The env parameter is application-specific (e.g., dodder passes env_local.Env).
type Completer interface {
	Complete(Request, any, CommandLineInput)
}

type FuncCompleter func(Request, any, CommandLineInput)

type FlagValueCompleter struct {
	interfaces.FlagValue
	FuncCompleter
}

func (completer FlagValueCompleter) String() string {
	if completer.FlagValue == nil {
		return ""
	} else {
		return completer.FlagValue.String()
	}
}

func (completer FlagValueCompleter) Complete(
	req Request,
	env any,
	commandLine CommandLineInput,
) {
	completer.FuncCompleter(req, env, commandLine)
}
