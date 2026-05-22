//go:build test

package ui_test

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/ui"
)

// Compile test: verify test-tagged symbols are re-exported from pkgs/ui.
func TestTaggedSymbolsExported(t *testing.T) {
	tt := ui.MakeT(t)
	var _ ui.T = tt

	ui.RunTestContext(t, func(tc *ui.TestContext) {
		_ = tc
	})
}
