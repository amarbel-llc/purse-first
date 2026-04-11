package command

type (
	// Cmd is the interface for dodder-style commands that use the Request pattern.
	// Commands implement Run(Request) and optionally implement CommandWithDescription,
	// CommandWithParams, CommandWithMCPAnnotations, etc.
	Cmd interface {
		Run(Request)
	}

	// CommandWithDescription is implemented by Cmd types that provide metadata.
	CommandWithDescription interface {
		GetDescription() Description
	}
)
