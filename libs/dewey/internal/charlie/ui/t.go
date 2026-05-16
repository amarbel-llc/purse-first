//go:build test

package ui

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

type T = test_ui.T

func MakeT(t *testing.T) T {
	return T{
		T:            t,
		Printer:      Err(),
		ErrorEncoder: CLIErrorTreeEncoder,
	}
}
