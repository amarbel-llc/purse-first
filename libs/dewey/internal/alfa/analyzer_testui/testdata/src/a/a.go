package a

import "testing"

// --- No diagnostics: canonical test entry points ---

// The runtime mandates this signature; the first param is exempt.
func TestSomething(t *testing.T) {}

// "Test" exactly is a valid entry-point name.
func Test(t *testing.T) {}

// TestMain takes *testing.M, not *testing.T — nothing to flag.
func TestMain(m *testing.M) { m.Run() }

// --- Diagnostics expected ---

// A plain helper that receives *testing.T.
func helper(t *testing.T) {} // want "prefer dewey's test_ui"

// "Testify" has a lowercase rune after "Test"; go test would not run
// it, so it is a helper and must be flagged.
func Testify(t *testing.T) {} // want "prefer dewey's test_ui"

// A function returning *testing.T.
func makeT() *testing.T { return nil } // want "prefer dewey's test_ui"

// A non-first parameter of an entry point is still flagged.
func TestExtra(t *testing.T, other *testing.T) {} // want "prefer dewey's test_ui"

// Methods are never entry points, even with a Test-shaped name.
type fixture struct {
	t    *testing.T // want "prefer dewey's test_ui"
	name string
}

func (f *fixture) TestRun(t *testing.T) {} // want "prefer dewey's test_ui"

// Interface methods are flagged.
type harness interface {
	Check(t *testing.T) error // want "prefer dewey's test_ui"
}

// Embedded stdlib testing.T (value form) is flagged.
type embedder struct {
	testing.T // want "prefer dewey's test_ui"
}

// --- No diagnostics: suppressed or unrelated ---

// Interop boundary opt-out via the directive on the same line.
func interop(t *testing.T) {} //testui:allow

type allowedField struct {
	t *testing.T //testui:allow
}

// Directive on the line above also suppresses.
//
//testui:allow
func interopAbove(t *testing.T) {}

// Non-testing parameters are untouched.
func noT(name string, n int) {}
