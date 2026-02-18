package command

import "fmt"

// Prompter provides interactive prompts. CLI implementations use terminal UI;
// MCP implementations return errors since mid-call prompting is not supported.
type Prompter interface {
	Confirm(msg string) (bool, error)
	Select(msg string, options []string) (int, error)
	Input(msg string) (string, error)
}

// StubPrompter returns errors for all prompts. Used in MCP mode.
type StubPrompter struct{}

func (StubPrompter) Confirm(msg string) (bool, error) {
	return false, fmt.Errorf("interactive prompt not available in MCP mode: %s", msg)
}

func (StubPrompter) Select(msg string, options []string) (int, error) {
	return 0, fmt.Errorf("interactive prompt not available in MCP mode: %s", msg)
}

func (StubPrompter) Input(msg string) (string, error) {
	return "", fmt.Errorf("interactive prompt not available in MCP mode: %s", msg)
}
