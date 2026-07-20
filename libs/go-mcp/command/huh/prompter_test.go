package huh

import (
	"testing"

	"code.linenisgreat.com/purse-first/libs/go-mcp/command"
)

func TestHuhPrompterImplementsPrompter(t *testing.T) {
	var _ command.Prompter = &Prompter{}
}
