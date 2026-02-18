package huh

import (
	"testing"

	"github.com/amarbel-llc/go-lib-mcp/command"
)

func TestHuhPrompterImplementsPrompter(t *testing.T) {
	var _ command.Prompter = &Prompter{}
}
