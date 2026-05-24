package primordial

import (
	"os"

	"golang.org/x/term"
)

func IsTTY(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// Deprecated: use IsTTY.
var IsTty = IsTTY
