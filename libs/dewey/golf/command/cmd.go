package command

type (
	// Cmd is the interface for dodder-style commands that use the Request pattern.
	// Commands implement Run(Request) and optionally implement CommandWithDescription,
	// CommandWithParams, CommandWithResult, CommandWithMCPAnnotations, etc.
	Cmd interface {
		Run(Request)
	}

	// CommandWithDescription is implemented by Cmd types that provide metadata.
	CommandWithDescription interface {
		GetDescription() Description
	}

	// CommandWithParams is the opt-in interface for declaring parameters.
	// Commands returning both flags and positional args (via Positional: true
	// on the Param) get automatic MCP schema generation and CLI dispatch.
	CommandWithParams interface {
		GetParams() []Param
	}

	// CommandWithResult is implemented by Cmd types that can return a
	// structured Result for MCP tool responses. Commands implementing
	// this interface get registered as MCP tools via Utility.AddCmd.
	// Commands implementing only Cmd (not CommandWithResult) are CLI-only.
	CommandWithResult interface {
		RunResult(Request) (*Result, error)
	}
)
