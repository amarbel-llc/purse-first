//go:build test

package ui

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

type T = test_ui.T

// MakeT wraps the runtime *testing.T into dewey's test_ui.T.
//
//testui:allow -- constructor seam: test_ui.T is built from the stdlib T here.
func MakeT(t *testing.T) T {
	return T{
		T:            t,
		Printer:      Err(),
		ErrorEncoder: CLIErrorTreeEncoder,
	}
}
