package a

import "context"

// activeContext stands in for dewey's ActiveContext: a non-stdlib
// context type that must never be flagged.
type activeContext interface {
	Done() <-chan struct{}
}

// --- Diagnostics expected ---

func paramContext(ctx context.Context) {} // want "stdlib context.Context used here"

func resultContext() context.Context { // want "stdlib context.Context used here"
	return context.Background()
}

func multiParam(name string, ctx context.Context, n int) {} // want "stdlib context.Context used here"

type server struct {
	ctx  context.Context // want "stdlib context.Context used here"
	name string
}

func (s *server) handle(ctx context.Context) {} // want "stdlib context.Context used here"

type runner interface {
	Run(ctx context.Context) error // want "stdlib context.Context used here"
}

// Embedded stdlib context is also flagged.
type embedder struct {
	context.Context // want "stdlib context.Context used here"
}

// --- No diagnostics expected ---

// Interop boundary opt-out via the directive comment.
func interopBoundary(ctx context.Context) {} //actx:allow

type allowedField struct {
	ctx context.Context //actx:allow
}

// Non-context parameters are untouched.
func noContext(name string, n int) {}

// ActiveContext-style custom types are not stdlib context.Context.
func customContext(ctx activeContext) {}

// Nested context types (slices, maps, pointers) are not the direct
// case this analyzer targets.
func nestedContexts(ctxs []context.Context, m map[string]context.Context) {}
