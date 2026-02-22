# tap-dancer Go Iterator API Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `WriteAll(iter.Seq[TestPoint])` to tap-dancer's Go Writer, enabling iterator-based TAP-14 output with structured diagnostics and mixed imperative/iterator subtests.

**Architecture:** New `Diagnostics` and `TestPoint` types in `tap.go`. `WriteAll` method on `Writer` walks the iterator, delegates to existing `Ok`/`NotOk`/`Skip`/`Todo`/`Subtest` methods, and auto-emits a trailing plan unless one was already emitted. Internal YAML helper extended to handle `any` values for `Diagnostics.Extras`.

**Tech Stack:** Go 1.23+ (`iter` package), existing tap-dancer Writer

**Design doc:** `docs/plans/2026-02-22-tap-dancer-iterator-api-design.md`

---

### Task 1: Add `planEmitted` field to Writer

**Files:**
- Modify: `packages/tap-dancer/go/tap.go:10-14` (Writer struct)
- Modify: `packages/tap-dancer/go/tap.go:69-75` (PlanAhead and Plan methods)
- Test: `packages/tap-dancer/go/tap_test.go`

**Step 1: Write failing test — PlanAhead prevents double plan**

Add to `packages/tap-dancer/go/tap_test.go`:

```go
func TestPlanAheadPreventsDoublePlan(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.PlanAhead(2)
	tw.Ok("a")
	tw.Ok("b")
	// Plan() after PlanAhead should be a no-op
	tw.Plan()
	out := buf.String()
	count := strings.Count(out, "1..")
	if count != 1 {
		t.Errorf("expected exactly one plan line, got %d in:\n%s", count, out)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `just test-tap-dancer-go`
Expected: FAIL — `Plan()` emits a second plan line

**Step 3: Add `planEmitted` field and set it in PlanAhead/Plan**

In `packages/tap-dancer/go/tap.go`, add `planEmitted bool` to Writer struct:

```go
type Writer struct {
	w           io.Writer
	n           int
	depth       int
	planEmitted bool
}
```

Update `PlanAhead`:

```go
func (tw *Writer) PlanAhead(n int) {
	fmt.Fprintf(tw.w, "1..%d\n", n)
	tw.planEmitted = true
}
```

Update `Plan`:

```go
func (tw *Writer) Plan() {
	if tw.planEmitted {
		return
	}
	fmt.Fprintf(tw.w, "1..%d\n", tw.n)
	tw.planEmitted = true
}
```

**Step 4: Run tests to verify all pass**

Run: `just test-tap-dancer-go`
Expected: All PASS (existing tests unaffected since they never call both PlanAhead and Plan)

**Step 5: Commit**

```
feat(tap-dancer): add planEmitted tracking to Writer

Prevents double plan emission when PlanAhead is called before Plan.
Prerequisite for WriteAll which auto-emits trailing plans.
```

---

### Task 2: Add Diagnostics type and YAML serialization helper

**Files:**
- Modify: `packages/tap-dancer/go/tap.go`
- Test: `packages/tap-dancer/go/tap_test.go`

**Step 1: Write failing test — Diagnostics struct serialization**

Add to `packages/tap-dancer/go/tap_test.go`:

```go
func TestWriteDiagnosticsNamedFields(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, &Diagnostics{
		Message:  "something broke",
		Severity: "fail",
		File:     "main.go",
		Line:     42,
	})
	out := buf.String()
	expected := "  ---\n  file: main.go\n  line: 42\n  message: something broke\n  severity: fail\n  ...\n"
	if out != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, out)
	}
}

func TestWriteDiagnosticsOmitsZeroValues(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, &Diagnostics{
		Message: "only message",
	})
	out := buf.String()
	if strings.Contains(out, "severity:") || strings.Contains(out, "file:") || strings.Contains(out, "line:") {
		t.Errorf("expected zero-value fields omitted, got:\n%s", out)
	}
	if !strings.Contains(out, "message: only message") {
		t.Errorf("expected message field, got:\n%s", out)
	}
}

func TestWriteDiagnosticsExtras(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, &Diagnostics{
		Message: "error",
		Extras: map[string]any{
			"exitcode": 1,
			"context":  "test run",
		},
	})
	out := buf.String()
	if !strings.Contains(out, "  context: test run\n") {
		t.Errorf("expected context extra, got:\n%s", out)
	}
	if !strings.Contains(out, "  exitcode: 1\n") {
		t.Errorf("expected exitcode extra, got:\n%s", out)
	}
}

func TestWriteDiagnosticsMultilineExtra(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, &Diagnostics{
		Extras: map[string]any{
			"output": "line one\nline two",
		},
	})
	out := buf.String()
	if !strings.Contains(out, "  output: |\n    line one\n    line two\n") {
		t.Errorf("expected block scalar for multiline extra, got:\n%s", out)
	}
}

func TestWriteDiagnosticsNil(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil diagnostics, got: %q", buf.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `just test-tap-dancer-go`
Expected: FAIL — `writeDiagnostics` and `Diagnostics` don't exist

**Step 3: Implement Diagnostics type and writeDiagnostics helper**

Add to `packages/tap-dancer/go/tap.go`:

```go
// Diagnostics represents structured YAML diagnostic data for a test point.
type Diagnostics struct {
	Message  string
	Severity string
	File     string
	Line     int
	Extras   map[string]any
}

func writeDiagnostics(w io.Writer, d *Diagnostics) {
	if d == nil {
		return
	}

	entries := make([]struct{ k, v string }, 0, 8)

	if d.File != "" {
		entries = append(entries, struct{ k, v string }{"file", d.File})
	}
	if d.Line != 0 {
		entries = append(entries, struct{ k, v string }{"line", fmt.Sprintf("%d", d.Line)})
	}
	if d.Message != "" {
		entries = append(entries, struct{ k, v string }{"message", d.Message})
	}
	if d.Severity != "" {
		entries = append(entries, struct{ k, v string }{"severity", d.Severity})
	}

	if len(d.Extras) > 0 {
		keys := make([]string, 0, len(d.Extras))
		for k := range d.Extras {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			entries = append(entries, struct{ k, v string }{k, fmt.Sprintf("%v", d.Extras[k])})
		}
	}

	if len(entries) == 0 {
		return
	}

	fmt.Fprintln(w, "  ---")
	for _, e := range entries {
		if strings.Contains(e.v, "\n") {
			fmt.Fprintf(w, "  %s: |\n", e.k)
			lines := strings.Split(e.v, "\n")
			for len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
			for _, line := range lines {
				fmt.Fprintf(w, "    %s\n", line)
			}
		} else {
			fmt.Fprintf(w, "  %s: %s\n", e.k, e.v)
		}
	}
	fmt.Fprintln(w, "  ...")
}
```

**Step 4: Run tests to verify all pass**

Run: `just test-tap-dancer-go`
Expected: All PASS

**Step 5: Commit**

```
feat(tap-dancer): add Diagnostics type and YAML serialization

Structured type for TAP-14 YAML diagnostic blocks with named fields
(Message, Severity, File, Line) and Extras map[string]any for
arbitrary additional keys.
```

---

### Task 3: Add TestPoint type and WriteAll method

**Files:**
- Modify: `packages/tap-dancer/go/tap.go`
- Test: `packages/tap-dancer/go/tap_test.go`

**Step 1: Write failing tests for WriteAll**

Add to `packages/tap-dancer/go/tap_test.go`:

```go
import "slices"

func TestWriteAllBasicOk(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "first", Ok: true},
		{Description: "second", Ok: true},
	}))
	expected := "TAP version 14\n" +
		"ok 1 - first\n" +
		"ok 2 - second\n" +
		"1..2\n"
	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestWriteAllNotOkWithDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "failing", Ok: false, Diagnostics: &Diagnostics{
			Message:  "broke",
			Severity: "fail",
		}},
	}))
	out := buf.String()
	if !strings.Contains(out, "not ok 1 - failing\n") {
		t.Errorf("expected not ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "  message: broke\n") {
		t.Errorf("expected message diagnostic, got:\n%s", out)
	}
	if !strings.HasSuffix(out, "1..1\n") {
		t.Errorf("expected trailing plan, got:\n%s", out)
	}
}

func TestWriteAllSkip(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "skipped", Skip: "not ready"},
	}))
	if !strings.Contains(buf.String(), "ok 1 - skipped # SKIP not ready\n") {
		t.Errorf("expected skip line, got:\n%s", buf.String())
	}
}

func TestWriteAllTodo(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "unfinished", Todo: "later"},
	}))
	if !strings.Contains(buf.String(), "not ok 1 - unfinished # TODO later\n") {
		t.Errorf("expected todo line, got:\n%s", buf.String())
	}
}

func TestWriteAllPlanAheadSkipsTrailingPlan(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.PlanAhead(2)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "a", Ok: true},
		{Description: "b", Ok: true},
	}))
	count := strings.Count(buf.String(), "1..")
	if count != 1 {
		t.Errorf("expected exactly one plan line, got %d in:\n%s", count, buf.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `just test-tap-dancer-go`
Expected: FAIL — `TestPoint` and `WriteAll` don't exist

**Step 3: Implement TestPoint and WriteAll**

Add to `packages/tap-dancer/go/tap.go`:

```go
import "iter"

// TestPoint represents a single test result for use with WriteAll.
type TestPoint struct {
	Description string
	Ok          bool
	Skip        string
	Todo        string
	Diagnostics *Diagnostics
	Subtests    func(*Writer)
}

func (tw *Writer) WriteAll(tests iter.Seq[TestPoint]) {
	for tp := range tests {
		if tp.Subtests != nil {
			child := tw.Subtest(tp.Description)
			tp.Subtests(child)
			if !child.planEmitted {
				child.Plan()
			}
			tw.Ok(tp.Description)
		} else if tp.Skip != "" {
			tw.Skip(tp.Description, tp.Skip)
		} else if tp.Todo != "" {
			tw.Todo(tp.Description, tp.Todo)
		} else if tp.Ok {
			tw.n++
			fmt.Fprintf(tw.w, "ok %d - %s\n", tw.n, tp.Description)
			writeDiagnostics(tw.w, tp.Diagnostics)
		} else {
			tw.n++
			fmt.Fprintf(tw.w, "not ok %d - %s\n", tw.n, tp.Description)
			writeDiagnostics(tw.w, tp.Diagnostics)
		}
	}
	if !tw.planEmitted {
		tw.Plan()
	}
}
```

**Step 4: Run tests to verify all pass**

Run: `just test-tap-dancer-go`
Expected: All PASS

**Step 5: Commit**

```
feat(tap-dancer): add TestPoint and WriteAll iterator API

WriteAll walks an iter.Seq[TestPoint] and produces TAP-14 output.
Supports ok/not-ok/skip/todo test points with structured Diagnostics,
and auto-emits a trailing plan unless PlanAhead was called.
```

---

### Task 4: Add subtest support to WriteAll

**Files:**
- Modify: `packages/tap-dancer/go/tap_test.go`
- (Implementation already in Task 3, just needs tests)

**Step 1: Write tests for subtests via WriteAll**

Add to `packages/tap-dancer/go/tap_test.go`:

```go
func TestWriteAllSubtest(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "nested", Subtests: func(sub *Writer) {
			sub.Ok("inner pass")
		}},
	}))
	expected := "TAP version 14\n" +
		"    # Subtest: nested\n" +
		"    ok 1 - inner pass\n" +
		"    1..1\n" +
		"ok 1 - nested\n" +
		"1..1\n"
	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestWriteAllNestedWriteAll(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "outer", Subtests: func(sub *Writer) {
			sub.WriteAll(slices.Values([]TestPoint{
				{Description: "inner-a", Ok: true},
				{Description: "inner-b", Ok: false, Diagnostics: &Diagnostics{
					Message: "broke",
				}},
			}))
		}},
	}))
	out := buf.String()
	if !strings.Contains(out, "    ok 1 - inner-a\n") {
		t.Errorf("expected inner-a, got:\n%s", out)
	}
	if !strings.Contains(out, "    not ok 2 - inner-b\n") {
		t.Errorf("expected inner-b, got:\n%s", out)
	}
	if !strings.Contains(out, "    1..2\n") {
		t.Errorf("expected subtest plan, got:\n%s", out)
	}
	if !strings.Contains(out, "ok 1 - outer\n") {
		t.Errorf("expected parent ok, got:\n%s", out)
	}
}

func TestWriteAllMixedImperativeAndIterator(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "mixed", Subtests: func(sub *Writer) {
			sub.Ok("imperative")
			sub.WriteAll(slices.Values([]TestPoint{
				{Description: "from-iter", Ok: true},
			}))
		}},
	}))
	out := buf.String()
	if !strings.Contains(out, "    ok 1 - imperative\n") {
		t.Errorf("expected imperative test, got:\n%s", out)
	}
	if !strings.Contains(out, "    ok 2 - from-iter\n") {
		t.Errorf("expected iterator test, got:\n%s", out)
	}
	// Plan should reflect total count (2) from the imperative + iterator
	if !strings.Contains(out, "    1..2\n") {
		t.Errorf("expected combined plan 1..2, got:\n%s", out)
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `just test-tap-dancer-go`
Expected: All PASS (subtest logic already implemented in Task 3)

**Step 3: Write validation test — WriteAll output validates with Reader**

Add to `packages/tap-dancer/go/tap_test.go`:

```go
func TestWriteAllOutputValidatesWithReader(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "pass", Ok: true},
		{Description: "fail", Ok: false, Diagnostics: &Diagnostics{
			Message: "broke",
		}},
		{Description: "skipped", Skip: "not ready"},
		{Description: "todo", Todo: "later"},
		{Description: "nested", Subtests: func(sub *Writer) {
			sub.WriteAll(slices.Values([]TestPoint{
				{Description: "inner", Ok: true},
			}))
		}},
	}))

	reader := NewReader(strings.NewReader(buf.String()))
	summary := reader.Summary()
	if !summary.Valid {
		diags := reader.Diagnostics()
		for _, d := range diags {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("WriteAll output did not validate as TAP-14:\n%s", buf.String())
	}
}
```

**Step 4: Run tests to verify all pass**

Run: `just test-tap-dancer-go`
Expected: All PASS

**Step 5: Commit**

```
test(tap-dancer): add WriteAll subtest and validation tests

Tests cover pure iterator subtests, nested WriteAll, mixed
imperative/iterator subtests, and Reader validation of WriteAll output.
```

---

### Task 5: Add ok diagnostics support

The current `Ok` method doesn't support diagnostics. `WriteAll` needs to emit
diagnostics after ok lines too (the design handles this inline in WriteAll
rather than changing the Ok method signature).

**Files:**
- Modify: `packages/tap-dancer/go/tap_test.go`

**Step 1: Write test for ok with diagnostics via WriteAll**

Add to `packages/tap-dancer/go/tap_test.go`:

```go
func TestWriteAllOkWithDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "pass with info", Ok: true, Diagnostics: &Diagnostics{
			Message: "inserted id=42",
		}},
	}))
	out := buf.String()
	if !strings.Contains(out, "ok 1 - pass with info\n") {
		t.Errorf("expected ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "  ---\n") {
		t.Errorf("expected YAML block after ok, got:\n%s", out)
	}
	if !strings.Contains(out, "  message: inserted id=42\n") {
		t.Errorf("expected message diagnostic, got:\n%s", out)
	}
}
```

**Step 2: Run tests to verify it passes**

Run: `just test-tap-dancer-go`
Expected: PASS (already handled in Task 3 implementation — WriteAll emits diagnostics for both ok and not-ok)

**Step 3: Commit**

```
test(tap-dancer): add test for ok-with-diagnostics via WriteAll
```

---

### Task 6: Final integration — format and full test run

**Step 1: Format code**

Run: `just fmt`

**Step 2: Run full test suite**

Run: `just test-tap-dancer-go`
Expected: All PASS

**Step 3: Run nix build**

Run: `just build`
Expected: Build succeeds

**Step 4: Commit any formatting changes**

```
chore(tap-dancer): format code
```
