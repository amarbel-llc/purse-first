package huh

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/golf/command"
)

func TestHuhPrompterImplementsPrompter(t *testing.T) {
	var _ command.Prompter = &Prompter{}
}
