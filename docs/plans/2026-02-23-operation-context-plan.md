# Operation Context Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the `operation` package in go-mcp providing a structured
operational context with panic-safe lifecycle, pluggable writers, and a TAP-14
writer adapter.

**Architecture:** New `operation` package in `libs/go-mcp/operation/`. Core
`Context` interface with private `ctx` implementation. Panic-based control flow
for `ControlFail`/`ControlSkip`/`ControlAbort` caught by `Run`'s recover. Writer
interface receives streaming `BeginOperation`/`EndOperation` events. TAP writer
adapter lives in `packages/tap-dancer/go/` and bridges `operation.Writer` to
tap-dancer's `Writer`.

**Tech Stack:** Go 1.23, `runtime` (Caller), `runtime/debug` (Stack), `fmt`,
`sync`. No external dependencies.

**Design doc:** `docs/plans/2026-02-23-operation-context-design.md`

---

### Task 1: Core Types

**Files:**
- Create: `libs/go-mcp/operation/types.go`
- Test: `libs/go-mcp/operation/types_test.go`

**Step 1: Write the failing test**

```go
package operation

import "testing"

func TestOutcomeValues(t *testing.T) {
	if Success != 0 {
		t.Errorf("Success should be 0, got %d", Success)
	}
	if Failure != 1 {
		t.Errorf("Failure should be 1, got %d", Failure)
	}
	if Skipped != 2 {
		t.Errorf("Skipped should be 2, got %d", Skipped)
	}
	if Aborted != 3 {
		t.Errorf("Aborted should be 3, got %d", Aborted)
	}
}

func TestAnnotationBitfield(t *testing.T) {
	combined := Idempotent | Destructive
	if combined&Idempotent == 0 {
		t.Error("expected Idempotent set")
	}
	if combined&Destructive == 0 {
		t.Error("expected Destructive set")
	}
	if combined&ReadOnly != 0 {
		t.Error("expected ReadOnly not set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestOutcome`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
package operation

type Outcome int

const (
	Success Outcome = iota
	Failure
	Skipped
	Aborted
)

type Annotation int

const (
	Idempotent  Annotation = 1 << iota
	Destructive
	Recoverable
	ReadOnly
)

type Diagnostic struct {
	File     string
	Line     int
	Message  string
	Severity string
	Source   string
	Extras   map[string]any
}

type OperationEvent struct {
	Description string
	Annotations []Annotation
	Outcome     Outcome
	Diagnostic  *Diagnostic
	MustErrors  []error
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v`
Expected: PASS

**Step 5: Commit**

```
feat(operation): add core types — Outcome, Annotation, Diagnostic, OperationEvent
```

---

### Task 2: Writer and Context Interfaces

**Files:**
- Create: `libs/go-mcp/operation/writer.go`
- Create: `libs/go-mcp/operation/context.go`

**Step 1: Write the writer interface**

`writer.go`:

```go
package operation

type Writer interface {
	BeginOperation(depth int, op *OperationEvent)
	EndOperation(depth int, op *OperationEvent)
}
```

**Step 2: Write the context interface**

`context.go`:

```go
package operation

type Context interface {
	Run(description string, fn func(Context) error, annotations ...Annotation) error

	ControlFail(msg string) error
	ControlFailf(format string, args ...any) error
	ControlWrap(err error) error
	ControlWrapf(err error, format string, args ...any) error
	ControlSkip(reason string) error
	ControlSkipf(format string, args ...any) error
	ControlAbort(err error) error

	DiagSet(key string, value any)
	DiagHelper()

	After(fn func() error)
	Must(fn func() error)
}
```

**Step 3: Run test to verify it compiles**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v`
Expected: PASS (existing tests still pass, new files compile)

**Step 4: Commit**

```
feat(operation): add Writer and Context interfaces
```

---

### Task 3: NullWriter and RecordingWriter for Testing

**Files:**
- Create: `libs/go-mcp/operation/writer_test_helpers_test.go`

These test helpers are used by all subsequent tasks.

**Step 1: Write test helpers**

```go
package operation

type nullWriter struct{}

func (nullWriter) BeginOperation(int, *OperationEvent) {}
func (nullWriter) EndOperation(int, *OperationEvent)   {}

type recordingWriter struct {
	begins []recordedEvent
	ends   []recordedEvent
}

type recordedEvent struct {
	depth int
	event OperationEvent // copy, not pointer
}

func (w *recordingWriter) BeginOperation(depth int, op *OperationEvent) {
	w.begins = append(w.begins, recordedEvent{depth: depth, event: *op})
}

func (w *recordingWriter) EndOperation(depth int, op *OperationEvent) {
	w.ends = append(w.ends, recordedEvent{depth: depth, event: *op})
}
```

**Step 2: Verify compilation**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v`
Expected: PASS

**Step 3: Commit**

```
test(operation): add nullWriter and recordingWriter test helpers
```

---

### Task 4: callSafe — Panic-Safe Callback Execution

**Files:**
- Create: `libs/go-mcp/operation/safe.go`
- Create: `libs/go-mcp/operation/safe_test.go`

**Step 1: Write the failing tests**

```go
package operation

import (
	"errors"
	"strings"
	"testing"
)

func TestCallSafeReturnsNilOnSuccess(t *testing.T) {
	err := callSafe(func() error { return nil })
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCallSafeReturnsError(t *testing.T) {
	expected := errors.New("oops")
	err := callSafe(func() error { return expected })
	if err != expected {
		t.Errorf("expected %v, got %v", expected, err)
	}
}

func TestCallSafeRecoversPanic(t *testing.T) {
	err := callSafe(func() error { panic("boom") })
	if err == nil {
		t.Fatal("expected error from panic")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected panic message in error, got: %v", err)
	}
}

func TestCallSafeRecoversPanicWithError(t *testing.T) {
	err := callSafe(func() error { panic(errors.New("kaboom")) })
	if err == nil {
		t.Fatal("expected error from panic")
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Errorf("expected panic message in error, got: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestCallSafe`
Expected: FAIL — `callSafe` undefined

**Step 3: Write minimal implementation**

```go
package operation

import (
	"fmt"
	"runtime/debug"
)

func callSafe(fn func() error) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("panic in callback: %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestCallSafe`
Expected: PASS

**Step 5: Commit**

```
feat(operation): add callSafe panic-safe callback executor
```

---

### Task 5: Internal ctx Struct and Constructor

**Files:**
- Create: `libs/go-mcp/operation/ctx.go`
- Create: `libs/go-mcp/operation/ctx_test.go`

**Step 1: Write the failing test**

```go
package operation

import "testing"

func TestNewReturnsContext(t *testing.T) {
	var w nullWriter
	ctx := New(&w)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestNew`
Expected: FAIL — `New` undefined

**Step 3: Write minimal implementation**

```go
package operation

type ctx struct {
	writer     Writer
	depth      int
	outcome    Outcome
	diagnostic *Diagnostic
	extras     map[string]any
	helpers    map[string]struct{} // set of helper function names for frame-skipping
	musts      []func() error
	afters     []func() error
	event      OperationEvent
	cancelFn   func(error) // propagate abort to parent
}

func New(w Writer) Context {
	return &ctx{
		writer: w,
	}
}

func (c *ctx) child(description string, annotations []Annotation) *ctx {
	return &ctx{
		writer:  c.writer,
		depth:   c.depth + 1,
		helpers: c.helpers,
		event: OperationEvent{
			Description: description,
			Annotations: annotations,
		},
	}
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestNew`
Expected: PASS

**Step 5: Commit**

```
feat(operation): add ctx struct and New constructor
```

---

### Task 6: Sentinel Types and Control Methods

**Files:**
- Create: `libs/go-mcp/operation/control.go`
- Create: `libs/go-mcp/operation/control_test.go`

**Step 1: Write the failing tests**

Test that Control methods panic with the correct sentinel types. We test the
panic directly since Run (which catches them) isn't implemented yet.

```go
package operation

import (
	"errors"
	"testing"
)

func TestControlFailPanicsWithFailSentinel(t *testing.T) {
	c := &ctx{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		s, ok := r.(failSentinel)
		if !ok {
			t.Fatalf("expected failSentinel, got %T", r)
		}
		if s.diag.Message != "bad input" {
			t.Errorf("expected message 'bad input', got %q", s.diag.Message)
		}
		if s.diag.Severity != "error" {
			t.Errorf("expected severity 'error', got %q", s.diag.Severity)
		}
		if s.diag.File == "" {
			t.Error("expected file to be captured")
		}
		if s.diag.Line == 0 {
			t.Error("expected line to be captured")
		}
	}()
	c.ControlFail("bad input")
}

func TestControlWrapPanicsWithFailSentinel(t *testing.T) {
	c := &ctx{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		s, ok := r.(failSentinel)
		if !ok {
			t.Fatalf("expected failSentinel, got %T", r)
		}
		if s.diag.Source != "external" {
			t.Errorf("expected source 'external', got %q", s.diag.Source)
		}
	}()
	c.ControlWrap(errors.New("db error"))
}

func TestControlSkipPanicsWithSkipSentinel(t *testing.T) {
	c := &ctx{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if _, ok := r.(skipSentinel); !ok {
			t.Fatalf("expected skipSentinel, got %T", r)
		}
	}()
	c.ControlSkip("not ready")
}

func TestControlAbortPanicsWithAbortSentinel(t *testing.T) {
	c := &ctx{}
	origErr := errors.New("fatal")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		s, ok := r.(abortSentinel)
		if !ok {
			t.Fatalf("expected abortSentinel, got %T", r)
		}
		if s.err != origErr {
			t.Errorf("expected original error")
		}
	}()
	c.ControlAbort(origErr)
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestControl`
Expected: FAIL — sentinel types and methods undefined

**Step 3: Write minimal implementation**

```go
package operation

import (
	"fmt"
	"runtime"
)

type failSentinel struct{ diag Diagnostic }
type skipSentinel struct{ diag Diagnostic }
type abortSentinel struct{ err error }

func (c *ctx) callerInfo(skip int) (string, int) {
	skip++ // account for callerInfo frame
	for {
		_, file, line, ok := runtime.Caller(skip)
		if !ok {
			return "???", 0
		}
		if _, isHelper := c.helpers[file+fmt.Sprintf(":%d", line)]; !isHelper {
			return file, line
		}
		skip++
	}
}

func (c *ctx) ControlFail(msg string) error {
	file, line := c.callerInfo(1)
	panic(failSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  msg,
		Severity: "error",
	}})
}

func (c *ctx) ControlFailf(format string, args ...any) error {
	file, line := c.callerInfo(1)
	panic(failSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  fmt.Sprintf(format, args...),
		Severity: "error",
	}})
}

func (c *ctx) ControlWrap(err error) error {
	file, line := c.callerInfo(1)
	panic(failSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  err.Error(),
		Severity: "error",
		Source:   "external",
	}})
}

func (c *ctx) ControlWrapf(err error, format string, args ...any) error {
	file, line := c.callerInfo(1)
	msg := fmt.Sprintf(format, args...) + ": " + err.Error()
	panic(failSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  msg,
		Severity: "error",
		Source:   "external",
	}})
}

func (c *ctx) ControlSkip(reason string) error {
	file, line := c.callerInfo(1)
	panic(skipSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  reason,
		Severity: "skip",
	}})
}

func (c *ctx) ControlSkipf(format string, args ...any) error {
	file, line := c.callerInfo(1)
	panic(skipSentinel{diag: Diagnostic{
		File:     file,
		Line:     line,
		Message:  fmt.Sprintf(format, args...),
		Severity: "skip",
	}})
}

func (c *ctx) ControlAbort(err error) error {
	panic(abortSentinel{err: err})
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestControl`
Expected: PASS

**Step 5: Commit**

```
feat(operation): add sentinel types and Control* methods
```

---

### Task 7: DiagSet, DiagHelper, After, Must

**Files:**
- Create: `libs/go-mcp/operation/diag.go`
- Create: `libs/go-mcp/operation/diag_test.go`
- Create: `libs/go-mcp/operation/cleanup.go`
- Create: `libs/go-mcp/operation/cleanup_test.go`

**Step 1: Write the failing tests for DiagSet and DiagHelper**

`diag_test.go`:

```go
package operation

import "testing"

func TestDiagSetAddsToExtras(t *testing.T) {
	c := &ctx{}
	c.DiagSet("key", "value")
	if c.extras["key"] != "value" {
		t.Errorf("expected extras[key]=value, got %v", c.extras["key"])
	}
}

func TestDiagSetMultipleKeys(t *testing.T) {
	c := &ctx{}
	c.DiagSet("a", 1)
	c.DiagSet("b", 2)
	if c.extras["a"] != 1 || c.extras["b"] != 2 {
		t.Errorf("expected both keys set, got %v", c.extras)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestDiag`
Expected: FAIL — methods undefined

**Step 3: Write DiagSet and DiagHelper implementation**

`diag.go`:

```go
package operation

import (
	"fmt"
	"runtime"
)

func (c *ctx) DiagSet(key string, value any) {
	if c.extras == nil {
		c.extras = make(map[string]any)
	}
	c.extras[key] = value
}

func (c *ctx) DiagHelper() {
	if c.helpers == nil {
		c.helpers = make(map[string]struct{})
	}
	_, file, line, ok := runtime.Caller(1)
	if ok {
		c.helpers[file+fmt.Sprintf(":%d", line)] = struct{}{}
	}
}
```

**Step 4: Run diag tests**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestDiag`
Expected: PASS

**Step 5: Write the failing tests for After and Must**

`cleanup_test.go`:

```go
package operation

import (
	"errors"
	"testing"
)

func TestAfterRegistersCallback(t *testing.T) {
	c := &ctx{}
	called := false
	c.After(func() error { called = true; return nil })
	if len(c.afters) != 1 {
		t.Fatalf("expected 1 after, got %d", len(c.afters))
	}
	c.runAfter()
	if !called {
		t.Error("expected after to be called")
	}
}

func TestAfterRunsLIFO(t *testing.T) {
	c := &ctx{}
	var order []int
	c.After(func() error { order = append(order, 1); return nil })
	c.After(func() error { order = append(order, 2); return nil })
	c.runAfter()
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Errorf("expected LIFO [2,1], got %v", order)
	}
}

func TestAfterSwallowsErrors(t *testing.T) {
	c := &ctx{}
	c.After(func() error { return errors.New("ignored") })
	c.runAfter() // should not panic
}

func TestAfterRecoversPanics(t *testing.T) {
	c := &ctx{}
	c.After(func() error { panic("boom") })
	c.runAfter() // should not panic
}

func TestMustRegistersCallback(t *testing.T) {
	c := &ctx{outcome: Success}
	c.Must(func() error { return nil })
	if len(c.musts) != 1 {
		t.Fatalf("expected 1 must, got %d", len(c.musts))
	}
	c.runMust()
	if c.outcome != Success {
		t.Error("expected outcome to remain Success")
	}
}

func TestMustErrorFlipsSuccessToFailure(t *testing.T) {
	c := &ctx{outcome: Success}
	c.Must(func() error { return errors.New("flush failed") })
	c.runMust()
	if c.outcome != Failure {
		t.Errorf("expected Failure, got %d", c.outcome)
	}
	if len(c.event.MustErrors) != 1 {
		t.Fatalf("expected 1 must error, got %d", len(c.event.MustErrors))
	}
}

func TestMustMultipleErrors(t *testing.T) {
	c := &ctx{outcome: Success}
	c.Must(func() error { return errors.New("err1") })
	c.Must(func() error { return errors.New("err2") })
	c.runMust()
	if len(c.event.MustErrors) != 2 {
		t.Fatalf("expected 2 must errors, got %d", len(c.event.MustErrors))
	}
}

func TestMustRecoversPanics(t *testing.T) {
	c := &ctx{outcome: Success}
	c.Must(func() error { panic("kaboom") })
	c.runMust() // should not panic
	if c.outcome != Failure {
		t.Error("expected Failure from panicking Must")
	}
	if len(c.event.MustErrors) != 1 {
		t.Fatalf("expected 1 must error, got %d", len(c.event.MustErrors))
	}
}

func TestMustDoesNotFlipFailure(t *testing.T) {
	c := &ctx{outcome: Failure}
	c.Must(func() error { return errors.New("also failed") })
	c.runMust()
	if c.outcome != Failure {
		t.Error("expected outcome to remain Failure")
	}
}
```

**Step 6: Write After and Must implementation**

`cleanup.go`:

```go
package operation

func (c *ctx) After(fn func() error) {
	c.afters = append(c.afters, fn)
}

func (c *ctx) Must(fn func() error) {
	c.musts = append(c.musts, fn)
}

func (c *ctx) runAfter() {
	for i := len(c.afters) - 1; i >= 0; i-- {
		_ = callSafe(c.afters[i])
	}
}

func (c *ctx) runMust() {
	for i := len(c.musts) - 1; i >= 0; i-- {
		if err := callSafe(c.musts[i]); err != nil {
			c.event.MustErrors = append(c.event.MustErrors, err)
			if c.outcome == Success {
				c.outcome = Failure
			}
		}
	}
}
```

**Step 7: Run all cleanup tests**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run "TestAfter|TestMust"`
Expected: PASS

**Step 8: Commit**

```
feat(operation): add DiagSet, DiagHelper, After, and Must
```

---

### Task 8: Run Method

**Files:**
- Modify: `libs/go-mcp/operation/ctx.go`
- Create: `libs/go-mcp/operation/run_test.go`

**Step 1: Write failing tests for Run**

```go
package operation

import (
	"errors"
	"strings"
	"testing"
)

func TestRunSuccess(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	err := ctx.Run("step", func(ctx Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(w.ends) != 1 {
		t.Fatalf("expected 1 end event, got %d", len(w.ends))
	}
	if w.ends[0].event.Outcome != Success {
		t.Errorf("expected Success, got %d", w.ends[0].event.Outcome)
	}
}

func TestRunPlainError(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	err := ctx.Run("step", func(ctx Context) error {
		return errors.New("oops")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if w.ends[0].event.Outcome != Failure {
		t.Errorf("expected Failure, got %d", w.ends[0].event.Outcome)
	}
	if w.ends[0].event.Diagnostic == nil {
		t.Fatal("expected diagnostic")
	}
	if w.ends[0].event.Diagnostic.Message != "oops" {
		t.Errorf("expected message 'oops', got %q", w.ends[0].event.Diagnostic.Message)
	}
}

func TestRunControlFail(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	err := ctx.Run("step", func(ctx Context) error {
		return ctx.ControlFail("bad input")
	})
	if err != nil {
		t.Fatalf("ControlFail should not propagate as error, got %v", err)
	}
	if w.ends[0].event.Outcome != Failure {
		t.Errorf("expected Failure, got %d", w.ends[0].event.Outcome)
	}
	if w.ends[0].event.Diagnostic.File == "" {
		t.Error("expected file:line in diagnostic")
	}
}

func TestRunControlSkip(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	err := ctx.Run("step", func(ctx Context) error {
		return ctx.ControlSkip("not ready")
	})
	if err != nil {
		t.Fatalf("ControlSkip should not propagate as error, got %v", err)
	}
	if w.ends[0].event.Outcome != Skipped {
		t.Errorf("expected Skipped, got %d", w.ends[0].event.Outcome)
	}
}

func TestRunControlAbort(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	origErr := errors.New("fatal")
	err := ctx.Run("step", func(ctx Context) error {
		return ctx.ControlAbort(origErr)
	})
	if err != origErr {
		t.Errorf("expected abort error to propagate, got %v", err)
	}
	if w.ends[0].event.Outcome != Aborted {
		t.Errorf("expected Aborted, got %d", w.ends[0].event.Outcome)
	}
}

func TestRunUnknownPanicCaptured(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	err := ctx.Run("step", func(ctx Context) error {
		panic("nil pointer bug")
	})
	if err != nil {
		t.Fatalf("unknown panics should not propagate as error, got %v", err)
	}
	if w.ends[0].event.Outcome != Failure {
		t.Errorf("expected Failure, got %d", w.ends[0].event.Outcome)
	}
	if w.ends[0].event.Diagnostic.Severity != "panic" {
		t.Errorf("expected severity 'panic', got %q", w.ends[0].event.Diagnostic.Severity)
	}
	if !strings.Contains(w.ends[0].event.Diagnostic.Message, "nil pointer bug") {
		t.Error("expected panic message in diagnostic")
	}
}

func TestRunAnnotationsPassedToEvent(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	ctx.Run("step", func(ctx Context) error {
		return nil
	}, Destructive, Idempotent)
	if len(w.ends[0].event.Annotations) != 2 {
		t.Errorf("expected 2 annotations, got %d", len(w.ends[0].event.Annotations))
	}
}

func TestRunNestedDepth(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	ctx.Run("parent", func(ctx Context) error {
		ctx.Run("child", func(ctx Context) error {
			return nil
		})
		return nil
	})
	if w.begins[0].depth != 1 {
		t.Errorf("parent depth: expected 1, got %d", w.begins[0].depth)
	}
	if w.begins[1].depth != 2 {
		t.Errorf("child depth: expected 2, got %d", w.begins[1].depth)
	}
}

func TestRunDiagSetAppearsInEvent(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	ctx.Run("step", func(ctx Context) error {
		ctx.DiagSet("key", "value")
		return nil
	})
	if w.ends[0].event.Diagnostic == nil {
		t.Fatal("expected diagnostic with extras")
	}
	if w.ends[0].event.Diagnostic.Extras["key"] != "value" {
		t.Error("expected extras[key]=value")
	}
}

func TestRunMustAndAfterExecute(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	mustRan := false
	afterRan := false
	ctx.Run("step", func(ctx Context) error {
		ctx.Must(func() error { mustRan = true; return nil })
		ctx.After(func() error { afterRan = true; return nil })
		return nil
	})
	if !mustRan {
		t.Error("expected Must to run")
	}
	if !afterRan {
		t.Error("expected After to run")
	}
}

func TestRunMustAndAfterRunEvenOnPanic(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	mustRan := false
	afterRan := false
	ctx.Run("step", func(ctx Context) error {
		ctx.Must(func() error { mustRan = true; return nil })
		ctx.After(func() error { afterRan = true; return nil })
		panic("boom")
	})
	if !mustRan {
		t.Error("expected Must to run after panic")
	}
	if !afterRan {
		t.Error("expected After to run after panic")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run "TestRun"`
Expected: FAIL — Run method not implemented

**Step 3: Write Run implementation**

Add to `ctx.go`:

```go
func (c *ctx) event() *OperationEvent {
	return &c.event
}

func (c *ctx) Run(
	description string,
	fn func(Context) error,
	annotations ...Annotation,
) (retErr error) {
	child := c.child(description, annotations)
	c.writer.BeginOperation(child.depth, child.event())

	defer func() {
		// Merge extras into diagnostic if any were set
		if len(child.extras) > 0 {
			if child.diagnostic == nil {
				child.diagnostic = &Diagnostic{}
			}
			child.diagnostic.Extras = child.extras
		}
		child.event.Outcome = child.outcome
		child.event.Diagnostic = child.diagnostic
		// 1. Must callbacks
		child.runMust()
		// 2. After callbacks
		child.runAfter()
		// 3. Emit end event
		c.writer.EndOperation(child.depth, child.event())
	}()

	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case failSentinel:
				child.outcome = Failure
				child.diagnostic = &v.diag
			case skipSentinel:
				child.outcome = Skipped
				child.diagnostic = &v.diag
			case abortSentinel:
				child.outcome = Aborted
				retErr = v.err
			default:
				child.outcome = Failure
				child.diagnostic = &Diagnostic{
					Message:  fmt.Sprintf("panic: %v", r),
					Severity: "panic",
					Extras:   map[string]any{"stack": string(debug.Stack())},
				}
			}
		}
	}()

	retErr = fn(child)
	if retErr != nil {
		child.outcome = Failure
		child.diagnostic = &Diagnostic{
			Message:  retErr.Error(),
			Severity: "error",
		}
	} else if child.outcome == 0 {
		child.outcome = Success
	}

	return retErr
}
```

Add missing imports to `ctx.go`:

```go
import (
	"fmt"
	"runtime/debug"
)
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run "TestRun"`
Expected: PASS

**Step 5: Commit**

```
feat(operation): implement Run with panic recovery and writer events
```

---

### Task 9: DiagHelper Frame-Skipping Integration

**Files:**
- Modify: `libs/go-mcp/operation/control.go` (update callerInfo)
- Create: `libs/go-mcp/operation/helper_test.go`

**Step 1: Write the failing test**

The current `callerInfo` uses a simplistic helper check. Rework it to match
`testing.T.Helper()` semantics — tracking helper function names, not
file:line pairs.

```go
package operation

import "testing"

func helperThatFails(ctx Context) {
	ctx.DiagHelper()
	ctx.ControlFail("failed in helper")
}

func TestDiagHelperSkipsFrame(t *testing.T) {
	w := &recordingWriter{}
	root := New(w)
	root.Run("step", func(ctx Context) error {
		helperThatFails(ctx) // file:line should point HERE
		return nil
	})
	diag := w.ends[0].event.Diagnostic
	if diag == nil {
		t.Fatal("expected diagnostic")
	}
	// The file should be this test file, and line should be the
	// helperThatFails call site, NOT inside helperThatFails
	if diag.File == "" {
		t.Error("expected file in diagnostic")
	}
	// The line should not be the line inside helperThatFails where
	// ControlFail is called
	t.Logf("diagnostic file:line = %s:%d", diag.File, diag.Line)
}
```

**Step 2: Run test to verify current behavior**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestDiagHelper`
Expected: May pass or fail depending on current callerInfo — check the logged
file:line.

**Step 3: Revise callerInfo to use function-name-based helper tracking**

Update `diag.go`:

```go
func (c *ctx) DiagHelper() {
	if c.helpers == nil {
		c.helpers = make(map[string]struct{})
	}
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			c.helpers[fn.Name()] = struct{}{}
		}
	}
}
```

Update `control.go` callerInfo:

```go
func (c *ctx) callerInfo(skip int) (string, int) {
	skip++ // account for callerInfo frame
	for {
		pc, file, line, ok := runtime.Caller(skip)
		if !ok {
			return "???", 0
		}
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			return file, line
		}
		if _, isHelper := c.helpers[fn.Name()]; !isHelper {
			return file, line
		}
		skip++
	}
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestDiagHelper`
Expected: PASS — logged file:line points to the call site of helperThatFails

**Step 5: Commit**

```
feat(operation): implement DiagHelper frame-skipping via function names
```

---

### Task 10: TAP Writer Adapter

**Files:**
- Create: `packages/tap-dancer/go/operation_writer.go`
- Create: `packages/tap-dancer/go/operation_writer_test.go`

This bridges `operation.Writer` to tap-dancer's `Writer`. Lives in the
tap-dancer package since it depends on both operation and tap types.

**Step 1: Write the failing test**

```go
package tap

import (
	"bytes"
	"strings"
	"testing"

	"code.linenisgreat.com/purse-first/libs/go-mcp/operation"
)

func TestOperationWriterLeafSuccess(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Success,
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "ok 1 - step") {
		t.Errorf("expected ok line, got:\n%s", out)
	}
}

func TestOperationWriterLeafFailure(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Failure,
		Diagnostic: &operation.Diagnostic{
			File:     "main.go",
			Line:     42,
			Message:  "broken",
			Severity: "error",
		},
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "not ok 1 - step") {
		t.Errorf("expected not ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "file: main.go") {
		t.Errorf("expected file diagnostic, got:\n%s", out)
	}
	if !strings.Contains(out, "message: broken") {
		t.Errorf("expected message diagnostic, got:\n%s", out)
	}
}

func TestOperationWriterSkip(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Skipped,
		Diagnostic:  &operation.Diagnostic{Message: "not ready"},
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "# SKIP not ready") {
		t.Errorf("expected skip directive, got:\n%s", out)
	}
}

func TestOperationWriterNested(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	// Parent begins
	ow.BeginOperation(1, &operation.OperationEvent{Description: "parent"})
	// Child
	ow.BeginOperation(2, &operation.OperationEvent{Description: "child"})
	ow.EndOperation(2, &operation.OperationEvent{
		Description: "child",
		Outcome:     operation.Success,
	})
	// Parent ends
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "parent",
		Outcome:     operation.Success,
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "# Subtest: parent") {
		t.Errorf("expected subtest header, got:\n%s", out)
	}
	if !strings.Contains(out, "ok 1 - child") {
		t.Errorf("expected child ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "ok 1 - parent") {
		t.Errorf("expected parent ok line, got:\n%s", out)
	}
}

func TestOperationWriterExternalSource(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Failure,
		Diagnostic: &operation.Diagnostic{
			File:     "main.go",
			Line:     10,
			Message:  "db error",
			Severity: "error",
			Source:   "external",
		},
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "source: external") {
		t.Errorf("expected source: external, got:\n%s", out)
	}
}

func TestOperationWriterMustErrors(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)
	var errs []error
	errs = append(errs, errors.New("lock release failed"))

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Failure,
		MustErrors:  errs,
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "lock release failed") {
		t.Errorf("expected must error in output, got:\n%s", out)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./packages/tap-dancer/go/ -v -run TestOperationWriter`
Expected: FAIL — `NewOperationWriter` undefined

**Step 3: Write the TAP writer adapter**

`operation_writer.go`:

```go
package tap

import (
	"code.linenisgreat.com/purse-first/libs/go-mcp/operation"
)

// OperationWriter adapts operation.Writer to tap-dancer's Writer.
type OperationWriter struct {
	// stack of writers: stack[0] is root, stack[len-1] is current
	stack []*Writer
}

// NewOperationWriter creates an OperationWriter backed by a TAP Writer.
func NewOperationWriter(tw *Writer) *OperationWriter {
	return &OperationWriter{
		stack: []*Writer{tw},
	}
}

func (ow *OperationWriter) current() *Writer {
	return ow.stack[len(ow.stack)-1]
}

func (ow *OperationWriter) BeginOperation(depth int, op *operation.OperationEvent) {
	// If depth > current stack depth, this is a subtest — push a child writer
	for len(ow.stack) < depth {
		parent := ow.current()
		child := parent.Subtest(op.Description)
		ow.stack = append(ow.stack, child)
	}
}

func (ow *OperationWriter) EndOperation(depth int, op *operation.OperationEvent) {
	tw := ow.current()

	switch op.Outcome {
	case operation.Success:
		tw.Ok(op.Description)
		if op.Diagnostic != nil {
			ow.writeDiag(tw, op)
		}
	case operation.Failure:
		diags := ow.buildDiagMap(op)
		tw.NotOk(op.Description, diags)
	case operation.Skipped:
		reason := ""
		if op.Diagnostic != nil {
			reason = op.Diagnostic.Message
		}
		tw.Skip(op.Description, reason)
	case operation.Aborted:
		diags := ow.buildDiagMap(op)
		diags["aborted"] = "true"
		tw.NotOk(op.Description, diags)
	}

	// If we're deeper than depth, pop back (end of subtest)
	for len(ow.stack) > depth {
		child := ow.stack[len(ow.stack)-1]
		child.Plan()
		ow.stack = ow.stack[:len(ow.stack)-1]
	}
}

func (ow *OperationWriter) buildDiagMap(op *operation.OperationEvent) map[string]string {
	diags := make(map[string]string)
	if op.Diagnostic != nil {
		if op.Diagnostic.File != "" {
			diags["file"] = op.Diagnostic.File
		}
		if op.Diagnostic.Line != 0 {
			diags["line"] = fmt.Sprintf("%d", op.Diagnostic.Line)
		}
		if op.Diagnostic.Message != "" {
			diags["message"] = op.Diagnostic.Message
		}
		if op.Diagnostic.Severity != "" {
			diags["severity"] = op.Diagnostic.Severity
		}
		if op.Diagnostic.Source != "" {
			diags["source"] = op.Diagnostic.Source
		}
		for k, v := range op.Diagnostic.Extras {
			diags[k] = fmt.Sprintf("%v", v)
		}
	}
	for i, err := range op.MustErrors {
		key := "must_error"
		if len(op.MustErrors) > 1 {
			key = fmt.Sprintf("must_error_%d", i+1)
		}
		diags[key] = err.Error()
	}
	return diags
}

func (ow *OperationWriter) writeDiag(tw *Writer, op *operation.OperationEvent) {
	d := &Diagnostics{Extras: make(map[string]any)}
	if op.Diagnostic.File != "" {
		d.File = op.Diagnostic.File
	}
	if op.Diagnostic.Line != 0 {
		d.Line = op.Diagnostic.Line
	}
	if op.Diagnostic.Message != "" {
		d.Message = op.Diagnostic.Message
	}
	if op.Diagnostic.Severity != "" {
		d.Severity = op.Diagnostic.Severity
	}
	if op.Diagnostic.Source != "" {
		d.Extras["source"] = op.Diagnostic.Source
	}
	for k, v := range op.Diagnostic.Extras {
		d.Extras[k] = v
	}
	writeDiagnostics(tw.w, d)
}
```

Add `import "fmt"` to the file.

**Step 4: Update tap-dancer go.mod**

The tap-dancer go.mod needs to depend on the go-mcp operation package. Since
they're in a workspace, the `go.work` handles resolution. If needed, run:

Run: `nix develop --command go mod tidy` (from packages/tap-dancer/go/)

**Step 5: Run test to verify it passes**

Run: `nix develop --command go test ./packages/tap-dancer/go/ -v -run TestOperationWriter`
Expected: PASS

**Step 6: Commit**

```
feat(tap-dancer): add OperationWriter adapter for operation.Writer → TAP-14
```

---

### Task 11: End-to-End Integration Test

**Files:**
- Create: `libs/go-mcp/operation/integration_test.go`

This test wires the full stack: operation.Context → TAP OperationWriter →
tap-dancer Writer → buffer → validate with tap-dancer Reader.

**Step 1: Write the integration test**

```go
package operation_test

import (
	"bytes"
	"errors"
	"testing"

	"code.linenisgreat.com/purse-first/libs/go-mcp/operation"
	tap "github.com/amarbel-llc/tap-dancer/go"
)

func TestEndToEndTAPOutput(t *testing.T) {
	var buf bytes.Buffer
	tw := tap.NewWriter(&buf)
	ow := tap.NewOperationWriter(tw)
	ctx := operation.New(ow)

	ctx.Run("deploy", func(ctx operation.Context) error {
		ctx.Run("backup", func(ctx operation.Context) error {
			ctx.DiagSet("size_mb", 420)
			return nil
		}, operation.ReadOnly)

		ctx.Run("migrate", func(ctx operation.Context) error {
			return ctx.ControlWrap(errors.New("pq: relation exists"))
		}, operation.Destructive, operation.Idempotent)

		return nil
	})
	tw.Plan()

	out := buf.String()
	t.Logf("TAP output:\n%s", out)

	// Validate through Reader
	reader := tap.NewReader(bytes.NewReader(buf.Bytes()))
	for reader.Next() {
	}
	summary := reader.Summary()
	if summary.Valid {
		// Valid TAP is good
	}
	diags := reader.Diagnostics()
	t.Logf("Diagnostics: %v", diags)
}
```

**Step 2: Run test**

Run: `nix develop --command go test ./libs/go-mcp/operation/ -v -run TestEndToEnd`
Expected: PASS — validate TAP output is well-formed

**Step 3: Commit**

```
test(operation): add end-to-end integration test with TAP output validation
```

---

### Task 12: Run All Tests

**Step 1: Run full test suite**

Run: `nix develop --command go test ./...`
Expected: All tests PASS

**Step 2: Run nix flake check**

Run: `nix flake check` (from worktree root)
Expected: PASS

**Step 3: Commit any fixups if needed**

---

## Summary

| Task | What | Files |
|------|------|-------|
| 1 | Core types | `operation/types.go`, `types_test.go` |
| 2 | Interfaces | `operation/writer.go`, `context.go` |
| 3 | Test helpers | `operation/writer_test_helpers_test.go` |
| 4 | callSafe | `operation/safe.go`, `safe_test.go` |
| 5 | ctx struct + New | `operation/ctx.go`, `ctx_test.go` |
| 6 | Sentinel types + Control* | `operation/control.go`, `control_test.go` |
| 7 | DiagSet/DiagHelper/After/Must | `operation/diag.go`, `cleanup.go`, tests |
| 8 | Run method | `operation/ctx.go`, `run_test.go` |
| 9 | DiagHelper frame-skipping | `operation/control.go`, `helper_test.go` |
| 10 | TAP writer adapter | `tap-dancer/go/operation_writer.go`, test |
| 11 | End-to-end integration | `operation/integration_test.go` |
| 12 | Full test suite validation | N/A |
