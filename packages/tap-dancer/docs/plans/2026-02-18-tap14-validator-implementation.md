# TAP-14 Validator Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a TAP-14 parser/validator to tap-dancer as a reusable Go library and bob CLI+MCP tool.

**Architecture:** Line-oriented state machine with nesting stack. Single-pass streaming parser processes input line-by-line, classifying each line and validating against accumulated state. Subtest nesting is handled by a stack of context frames.

**Tech Stack:** Go 1.23, bob (`github.com/amarbel-llc/purse-first/libs/go-mcp/command`), Nix (`buildGoApplication` via `go.overlays.default`)

---

### Task 1: Diagnostic Types

**Files:**
- Create: `go/diagnostic.go`
- Test: `go/diagnostic_test.go`

**Step 1: Write the failing test**

Create `go/diagnostic_test.go`:

```go
package tap

import "testing"

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestDirectiveString(t *testing.T) {
	tests := []struct {
		d    Directive
		want string
	}{
		{DirectiveNone, ""},
		{DirectiveSkip, "SKIP"},
		{DirectiveTodo, "TODO"},
	}
	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("Directive(%d).String() = %q, want %q", tt.d, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `just test-go`
Expected: FAIL — `Severity`, `Directive` types undefined.

**Step 3: Write minimal implementation**

Create `go/diagnostic.go`:

```go
package tap

// Severity indicates the severity of a validation diagnostic.
type Severity int

const (
	SeverityError   Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "unknown"
	}
}

// Diagnostic represents a single validation problem found in TAP input.
type Diagnostic struct {
	Line     int      `json:"line"`
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
}

// Directive represents a TAP test point directive.
type Directive int

const (
	DirectiveNone Directive = iota
	DirectiveSkip
	DirectiveTodo
)

func (d Directive) String() string {
	switch d {
	case DirectiveSkip:
		return "SKIP"
	case DirectiveTodo:
		return "TODO"
	default:
		return ""
	}
}

// EventType classifies a parsed TAP line.
type EventType int

const (
	EventVersion EventType = iota
	EventPlan
	EventTestPoint
	EventYAMLDiagnostic
	EventComment
	EventBailOut
	EventPragma
	EventSubtestStart
	EventSubtestEnd
	EventUnknown
)

// TestPointResult holds parsed data from a test point line.
type TestPointResult struct {
	Number      int       `json:"number"`
	Description string    `json:"description"`
	OK          bool      `json:"ok"`
	Directive   Directive `json:"directive"`
	Reason      string    `json:"reason,omitempty"`
}

// PlanResult holds parsed data from a plan line.
type PlanResult struct {
	Count  int    `json:"count"`
	Reason string `json:"reason,omitempty"`
}

// BailOutResult holds parsed data from a bail out line.
type BailOutResult struct {
	Reason string `json:"reason,omitempty"`
}

// PragmaResult holds parsed data from a pragma line.
type PragmaResult struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// Event represents a single parsed TAP element.
type Event struct {
	Type      EventType        `json:"type"`
	Line      int              `json:"line"`
	Depth     int              `json:"depth"`
	Raw       string           `json:"raw"`
	TestPoint *TestPointResult `json:"test_point,omitempty"`
	Plan      *PlanResult      `json:"plan,omitempty"`
	BailOut   *BailOutResult   `json:"bail_out,omitempty"`
	YAML      map[string]string `json:"yaml,omitempty"`
	Comment   string           `json:"comment,omitempty"`
	Pragma    *PragmaResult    `json:"pragma,omitempty"`
}

// Summary provides aggregate results after parsing a TAP document.
type Summary struct {
	Version    int  `json:"version"`
	TotalTests int  `json:"total_tests"`
	Passed     int  `json:"passed"`
	Failed     int  `json:"failed"`
	Skipped    int  `json:"skipped"`
	Todo       int  `json:"todo"`
	BailedOut  bool `json:"bailed_out"`
	PlanCount  int  `json:"plan_count"`
	Valid      bool `json:"valid"`
}
```

**Step 4: Run test to verify it passes**

Run: `just test-go`
Expected: PASS

**Step 5: Commit**

```
git add go/diagnostic.go go/diagnostic_test.go
git commit -m "feat: add TAP-14 diagnostic and event types"
```

---

### Task 2: Line Classifier

The line classifier takes a raw line (with indentation already stripped) and determines what kind of TAP element it is. This is the foundation the state machine builds on.

**Files:**
- Create: `go/classify.go`
- Test: `go/classify_test.go`

**Step 1: Write the failing test**

Create `go/classify_test.go`:

```go
package tap

import "testing"

func TestClassifyVersion(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"TAP version 14", lineVersion},
		{"TAP version 13", lineUnknown},
		{"TAP version 14 ", lineUnknown},
		{"tap version 14", lineUnknown},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyPlan(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"1..5", linePlan},
		{"1..0", linePlan},
		{"1..0 # skip all", linePlan},
		{"1..100", linePlan},
		{"2..5", lineUnknown},
		{"1..", lineUnknown},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyTestPoint(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"ok", lineTestPoint},
		{"ok 1", lineTestPoint},
		{"ok 1 - description", lineTestPoint},
		{"not ok", lineTestPoint},
		{"not ok 2 - failing", lineTestPoint},
		{"ok 1 - desc # SKIP reason", lineTestPoint},
		{"not ok 3 - desc # TODO reason", lineTestPoint},
		{"okay", lineUnknown},
		{"not okay", lineUnknown},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyYAMLMarkers(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"---", lineYAMLStart},
		{"...", lineYAMLEnd},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyBailOut(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"Bail out!", lineBailOut},
		{"Bail out! reason", lineBailOut},
		{"bail out!", lineUnknown},
		{"Bail out", lineUnknown},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyPragma(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"pragma +strict", linePragma},
		{"pragma -strict", linePragma},
		{"pragma strict", lineUnknown},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestClassifyComment(t *testing.T) {
	tests := []struct {
		line string
		want lineKind
	}{
		{"# comment", lineComment},
		{"# Subtest: name", lineSubtestComment},
		{"#comment", lineComment},
	}
	for _, tt := range tests {
		if got := classifyLine(tt.line); got != tt.want {
			t.Errorf("classifyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `just test-go`
Expected: FAIL — `classifyLine`, `lineKind` undefined.

**Step 3: Write minimal implementation**

Create `go/classify.go`:

```go
package tap

import (
	"regexp"
	"strings"
)

type lineKind int

const (
	lineUnknown lineKind = iota
	lineVersion
	linePlan
	lineTestPoint
	lineYAMLStart
	lineYAMLEnd
	lineBailOut
	linePragma
	lineComment
	lineSubtestComment
	lineEmpty
)

var (
	planRegexp     = regexp.MustCompile(`^1\.\.(\d+)(\s+#\s+(.*))?$`)
	testPointRegexp = regexp.MustCompile(`^(not )?ok\b`)
	pragmaRegexp   = regexp.MustCompile(`^pragma\s+[+-]\w`)
)

func classifyLine(line string) lineKind {
	if line == "TAP version 14" {
		return lineVersion
	}

	if planRegexp.MatchString(line) {
		return linePlan
	}

	if testPointRegexp.MatchString(line) {
		return lineTestPoint
	}

	if line == "---" {
		return lineYAMLStart
	}

	if line == "..." {
		return lineYAMLEnd
	}

	if strings.HasPrefix(line, "Bail out!") {
		return lineBailOut
	}

	if pragmaRegexp.MatchString(line) {
		return linePragma
	}

	if strings.HasPrefix(line, "# Subtest") {
		return lineSubtestComment
	}

	if strings.HasPrefix(line, "#") {
		return lineComment
	}

	if strings.TrimSpace(line) == "" {
		return lineEmpty
	}

	return lineUnknown
}
```

**Step 4: Run test to verify it passes**

Run: `just test-go`
Expected: PASS

**Step 5: Commit**

```
git add go/classify.go go/classify_test.go
git commit -m "feat: add TAP-14 line classifier"
```

---

### Task 3: Line Parsers

Parse classified lines into their structured representations (TestPointResult, PlanResult, etc.).

**Files:**
- Create: `go/parse.go`
- Test: `go/parse_test.go`

**Step 1: Write the failing test**

Create `go/parse_test.go`:

```go
package tap

import "testing"

func TestParsePlan(t *testing.T) {
	tests := []struct {
		line    string
		count   int
		reason  string
		wantErr bool
	}{
		{"1..5", 5, "", false},
		{"1..0", 0, "", false},
		{"1..0 # skip all", 0, "skip all", false},
		{"1..100", 100, "", false},
	}
	for _, tt := range tests {
		p, err := parsePlan(tt.line)
		if (err != nil) != tt.wantErr {
			t.Errorf("parsePlan(%q) error = %v, wantErr %v", tt.line, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if p.Count != tt.count {
			t.Errorf("parsePlan(%q).Count = %d, want %d", tt.line, p.Count, tt.count)
		}
		if p.Reason != tt.reason {
			t.Errorf("parsePlan(%q).Reason = %q, want %q", tt.line, p.Reason, tt.reason)
		}
	}
}

func TestParseTestPoint(t *testing.T) {
	tests := []struct {
		line      string
		ok        bool
		number    int
		desc      string
		directive Directive
		reason    string
	}{
		{"ok", true, 0, "", DirectiveNone, ""},
		{"ok 1", true, 1, "", DirectiveNone, ""},
		{"ok 1 - first test", true, 1, "first test", DirectiveNone, ""},
		{"not ok 2 - failing", false, 2, "failing", DirectiveNone, ""},
		{"ok 3 - skipped # SKIP not applicable", true, 3, "skipped", DirectiveSkip, "not applicable"},
		{"not ok 4 - todo # TODO not done", false, 4, "todo", DirectiveTodo, "not done"},
		{"ok - no number", true, 0, "no number", DirectiveNone, ""},
		{"not ok - also no number", false, 0, "also no number", DirectiveNone, ""},
		{"ok 1 - has \\# escaped hash", true, 1, "has # escaped hash", DirectiveNone, ""},
	}
	for _, tt := range tests {
		tp, _ := parseTestPoint(tt.line)
		if tp.OK != tt.ok {
			t.Errorf("parseTestPoint(%q).OK = %v, want %v", tt.line, tp.OK, tt.ok)
		}
		if tp.Number != tt.number {
			t.Errorf("parseTestPoint(%q).Number = %d, want %d", tt.line, tp.Number, tt.number)
		}
		if tp.Description != tt.desc {
			t.Errorf("parseTestPoint(%q).Description = %q, want %q", tt.line, tp.Description, tt.desc)
		}
		if tp.Directive != tt.directive {
			t.Errorf("parseTestPoint(%q).Directive = %v, want %v", tt.line, tp.Directive, tt.directive)
		}
		if tp.Reason != tt.reason {
			t.Errorf("parseTestPoint(%q).Reason = %q, want %q", tt.line, tp.Reason, tt.reason)
		}
	}
}

func TestParseBailOut(t *testing.T) {
	tests := []struct {
		line   string
		reason string
	}{
		{"Bail out!", ""},
		{"Bail out! database down", "database down"},
	}
	for _, tt := range tests {
		b := parseBailOut(tt.line)
		if b.Reason != tt.reason {
			t.Errorf("parseBailOut(%q).Reason = %q, want %q", tt.line, b.Reason, tt.reason)
		}
	}
}

func TestParsePragma(t *testing.T) {
	tests := []struct {
		line    string
		key     string
		enabled bool
	}{
		{"pragma +strict", "strict", true},
		{"pragma -strict", "strict", false},
	}
	for _, tt := range tests {
		p := parsePragma(tt.line)
		if p.Key != tt.key {
			t.Errorf("parsePragma(%q).Key = %q, want %q", tt.line, p.Key, tt.key)
		}
		if p.Enabled != tt.enabled {
			t.Errorf("parsePragma(%q).Enabled = %v, want %v", tt.line, p.Enabled, tt.enabled)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `just test-go`
Expected: FAIL — parse functions undefined.

**Step 3: Write minimal implementation**

Create `go/parse.go`:

```go
package tap

import (
	"fmt"
	"strconv"
	"strings"
)

func parsePlan(line string) (PlanResult, error) {
	m := planRegexp.FindStringSubmatch(line)
	if m == nil {
		return PlanResult{}, fmt.Errorf("invalid plan line: %q", line)
	}

	count, err := strconv.Atoi(m[1])
	if err != nil {
		return PlanResult{}, fmt.Errorf("invalid plan count: %v", err)
	}

	return PlanResult{
		Count:  count,
		Reason: strings.TrimSpace(m[3]),
	}, nil
}

func parseTestPoint(line string) (TestPointResult, []Diagnostic) {
	var tp TestPointResult
	var diags []Diagnostic

	rest := line
	if strings.HasPrefix(rest, "not ok") {
		tp.OK = false
		rest = rest[6:]
	} else if strings.HasPrefix(rest, "ok") {
		tp.OK = true
		rest = rest[2:]
	}

	rest = strings.TrimLeft(rest, " ")

	// Parse optional test number
	if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
		numEnd := 0
		for numEnd < len(rest) && rest[numEnd] >= '0' && rest[numEnd] <= '9' {
			numEnd++
		}
		tp.Number, _ = strconv.Atoi(rest[:numEnd])
		rest = rest[numEnd:]
	}

	// Parse optional description separator " - "
	if strings.HasPrefix(rest, " - ") {
		rest = rest[3:]
	} else if strings.HasPrefix(rest, " ") {
		rest = rest[1:]
	}

	// Find unescaped # for directive
	desc, directive, reason := splitDirective(rest)
	tp.Description = unescapeDescription(strings.TrimSpace(desc))
	tp.Directive = directive
	tp.Reason = reason

	return tp, diags
}

func splitDirective(s string) (desc string, directive Directive, reason string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++ // skip escaped char
			continue
		}
		if s[i] == '#' {
			// Check for directive pattern: " # TODO" or " # SKIP"
			if i > 0 && s[i-1] == ' ' {
				after := strings.TrimSpace(s[i+1:])
				upper := strings.ToUpper(after)
				if strings.HasPrefix(upper, "TODO") {
					reason := strings.TrimSpace(after[4:])
					return s[:i-1], DirectiveTodo, reason
				}
				if strings.HasPrefix(upper, "SKIP") {
					reason := strings.TrimSpace(after[4:])
					return s[:i-1], DirectiveSkip, reason
				}
			}
		}
	}
	return s, DirectiveNone, ""
}

func unescapeDescription(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			if next == '#' || next == '\\' {
				b.WriteByte(next)
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func parseBailOut(line string) BailOutResult {
	reason := strings.TrimPrefix(line, "Bail out!")
	return BailOutResult{Reason: strings.TrimSpace(reason)}
}

func parsePragma(line string) PragmaResult {
	rest := strings.TrimPrefix(line, "pragma ")
	enabled := rest[0] == '+'
	key := rest[1:]
	return PragmaResult{Key: key, Enabled: enabled}
}
```

**Step 4: Run test to verify it passes**

Run: `just test-go`
Expected: PASS

**Step 5: Commit**

```
git add go/parse.go go/parse_test.go
git commit -m "feat: add TAP-14 line parsers"
```

---

### Task 4: Reader Core (State Machine)

The core state machine that processes lines, manages the nesting stack, and produces Events and Diagnostics.

**Files:**
- Create: `go/reader.go`
- Test: `go/reader_test.go`

**Step 1: Write the failing test**

Create `go/reader_test.go`:

```go
package tap

import (
	"io"
	"strings"
	"testing"
)

func collectEvents(input string) ([]Event, []Diagnostic, Summary) {
	r := NewReader(strings.NewReader(input))
	var events []Event
	for {
		ev, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		events = append(events, ev)
	}
	return events, r.Diagnostics(), r.Summary()
}

func TestReaderValidMinimal(t *testing.T) {
	input := "TAP version 14\n1..2\nok 1 - first\nok 2 - second\n"
	events, diags, summary := collectEvents(input)

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[0].Type != EventVersion {
		t.Errorf("event 0: expected Version, got %v", events[0].Type)
	}
	if events[1].Type != EventPlan {
		t.Errorf("event 1: expected Plan, got %v", events[1].Type)
	}
	if events[2].Type != EventTestPoint {
		t.Errorf("event 2: expected TestPoint, got %v", events[2].Type)
	}

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error diagnostic: %s: %s", d.Rule, d.Message)
		}
	}

	if !summary.Valid {
		t.Error("expected Valid=true")
	}
	if summary.TotalTests != 2 {
		t.Errorf("expected 2 total tests, got %d", summary.TotalTests)
	}
	if summary.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", summary.Passed)
	}
}

func TestReaderTrailingPlan(t *testing.T) {
	input := "TAP version 14\nok 1 - a\nok 2 - b\n1..2\n"
	_, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true for trailing plan")
	}
}

func TestReaderMissingVersion(t *testing.T) {
	input := "1..1\nok 1 - test\n"
	_, diags, summary := collectEvents(input)

	if summary.Valid {
		t.Error("expected Valid=false for missing version")
	}
	found := false
	for _, d := range diags {
		if d.Rule == "version-required" {
			found = true
		}
	}
	if !found {
		t.Error("expected version-required diagnostic")
	}
}

func TestReaderPlanCountMismatch(t *testing.T) {
	input := "TAP version 14\n1..3\nok 1 - a\nok 2 - b\n"
	_, diags, summary := collectEvents(input)

	if summary.Valid {
		t.Error("expected Valid=false for plan count mismatch")
	}
	found := false
	for _, d := range diags {
		if d.Rule == "plan-count-mismatch" {
			found = true
		}
	}
	if !found {
		t.Error("expected plan-count-mismatch diagnostic")
	}
}

func TestReaderDuplicatePlan(t *testing.T) {
	input := "TAP version 14\n1..1\nok 1 - a\n1..1\n"
	_, diags, _ := collectEvents(input)

	found := false
	for _, d := range diags {
		if d.Rule == "plan-duplicate" {
			found = true
		}
	}
	if !found {
		t.Error("expected plan-duplicate diagnostic")
	}
}

func TestReaderYAMLBlock(t *testing.T) {
	input := "TAP version 14\n1..1\nnot ok 1 - fail\n  ---\n  message: broken\n  severity: fail\n  ...\n"
	events, diags, _ := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}

	foundYAML := false
	for _, ev := range events {
		if ev.Type == EventYAMLDiagnostic {
			foundYAML = true
			if ev.YAML["message"] != "broken" {
				t.Errorf("YAML message = %q, want %q", ev.YAML["message"], "broken")
			}
		}
	}
	if !foundYAML {
		t.Error("expected YAML diagnostic event")
	}
}

func TestReaderBailOut(t *testing.T) {
	input := "TAP version 14\n1..3\nok 1 - a\nBail out! database down\n"
	_, _, summary := collectEvents(input)

	if !summary.BailedOut {
		t.Error("expected BailedOut=true")
	}
}

func TestReaderSkipAndTodo(t *testing.T) {
	input := "TAP version 14\n1..3\nok 1 - a\nok 2 - b # SKIP lazy\nnot ok 3 - c # TODO later\n"
	_, _, summary := collectEvents(input)

	if summary.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", summary.Skipped)
	}
	if summary.Todo != 1 {
		t.Errorf("expected 1 todo, got %d", summary.Todo)
	}
}

func TestReaderNumberSequenceWarning(t *testing.T) {
	input := "TAP version 14\n1..2\nok 1 - a\nok 5 - b\n"
	_, diags, _ := collectEvents(input)

	found := false
	for _, d := range diags {
		if d.Rule == "test-number-sequence" {
			found = true
		}
	}
	if !found {
		t.Error("expected test-number-sequence warning")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `just test-go`
Expected: FAIL — `NewReader`, `Reader` undefined.

**Step 3: Write minimal implementation**

Create `go/reader.go`:

```go
package tap

import (
	"bufio"
	"io"
	"strings"
)

type readerState int

const (
	stateStart readerState = iota
	stateHeader
	stateBody
	stateYAML
	stateDone
)

type frame struct {
	depth          int
	planSeen       bool
	planCount      int
	planLine       int
	testCount      int
	lastTestNumber int
}

// Reader is a streaming TAP-14 parser and validator.
type Reader struct {
	scanner  *bufio.Scanner
	state    readerState
	lineNum  int
	stack    []frame
	diags    []Diagnostic
	done     bool
	bailed   bool
	yamlBuf  map[string]string
	lastWasTestPoint bool
}

// NewReader creates a new TAP-14 reader from the given input.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		scanner: bufio.NewScanner(r),
		stack:   []frame{{depth: 0}},
	}
}

func (r *Reader) currentFrame() *frame {
	return &r.stack[len(r.stack)-1]
}

func (r *Reader) addDiag(severity Severity, rule, message string) {
	r.diags = append(r.diags, Diagnostic{
		Line:     r.lineNum,
		Severity: severity,
		Rule:     rule,
		Message:  message,
	})
}

// Next returns the next parsed event from the TAP stream.
// Returns io.EOF when the stream is exhausted.
func (r *Reader) Next() (Event, error) {
	for r.scanner.Scan() {
		r.lineNum++
		raw := r.scanner.Text()

		// Determine indentation depth
		trimmed := strings.TrimLeft(raw, " ")
		indent := len(raw) - len(trimmed)
		depth := indent / 4
		yamlRelativeIndent := indent - (depth * 4)

		// Handle YAML block state
		if r.state == stateYAML {
			expectedIndent := (r.currentFrame().depth * 4) + 2
			if raw == strings.Repeat(" ", expectedIndent)+"..." {
				r.state = stateBody
				yaml := r.yamlBuf
				r.yamlBuf = nil
				return Event{
					Type:  EventYAMLDiagnostic,
					Line:  r.lineNum,
					Depth: r.currentFrame().depth,
					Raw:   raw,
					YAML:  yaml,
				}, nil
			}
			// Accumulate YAML content
			content := raw
			if len(content) >= expectedIndent {
				content = content[expectedIndent:]
			}
			parts := strings.SplitN(content, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				r.yamlBuf[key] = val
			}
			continue
		}

		// Handle depth changes for subtests
		if depth > r.currentFrame().depth {
			r.stack = append(r.stack, frame{depth: depth})
			// Don't emit SubtestStart event here — we already have
			// the # Subtest comment or infer from indent change
		}
		for depth < r.currentFrame().depth && len(r.stack) > 1 {
			completed := r.stack[len(r.stack)-1]
			r.stack = r.stack[:len(r.stack)-1]
			// Validate completed subtest
			if completed.planSeen && completed.testCount != completed.planCount {
				r.addDiag(SeverityError, "plan-count-mismatch",
					"subtest plan count mismatch: plan declared "+
						strings.Repeat("", 0)+
						formatInt(completed.planCount)+
						" tests but "+formatInt(completed.testCount)+" ran")
			}
		}

		kind := classifyLine(trimmed)

		switch kind {
		case lineVersion:
			if r.state != stateStart {
				if r.currentFrame().depth > 0 {
					r.addDiag(SeverityWarning, "subtest-version",
						"subtests should omit version line for TAP13 compatibility")
				}
			}
			r.state = stateHeader
			r.lastWasTestPoint = false
			return Event{Type: EventVersion, Line: r.lineNum, Depth: depth, Raw: raw}, nil

		case linePlan:
			f := r.currentFrame()
			if f.planSeen {
				r.addDiag(SeverityError, "plan-duplicate", "duplicate plan line")
			}
			plan, _ := parsePlan(trimmed)
			f.planSeen = true
			f.planCount = plan.Count
			f.planLine = r.lineNum
			if r.state == stateStart {
				r.addDiag(SeverityError, "version-required", "first line must be TAP version 14")
			}
			if r.state == stateHeader {
				r.state = stateBody
			}
			r.lastWasTestPoint = false
			return Event{Type: EventPlan, Line: r.lineNum, Depth: depth, Raw: raw, Plan: &plan}, nil

		case lineTestPoint:
			if r.state == stateStart {
				r.addDiag(SeverityError, "version-required", "first line must be TAP version 14")
			}
			r.state = stateBody
			f := r.currentFrame()
			tp, tpDiags := parseTestPoint(trimmed)
			r.diags = append(r.diags, tpDiags...)
			f.testCount++

			if tp.Number == 0 {
				r.addDiag(SeverityWarning, "test-number-missing", "test point without explicit number")
			} else {
				if tp.Number != f.lastTestNumber+1 {
					r.addDiag(SeverityWarning, "test-number-sequence",
						"test number "+formatInt(tp.Number)+" out of sequence, expected "+formatInt(f.lastTestNumber+1))
				}
				f.lastTestNumber = tp.Number
			}

			r.lastWasTestPoint = true
			return Event{Type: EventTestPoint, Line: r.lineNum, Depth: depth, Raw: raw, TestPoint: &tp}, nil

		case lineYAMLStart:
			if !r.lastWasTestPoint {
				r.addDiag(SeverityWarning, "yaml-orphan", "YAML block not following a test point")
			}
			expectedIndent := (r.currentFrame().depth * 4) + 2
			if yamlRelativeIndent != 2 && indent != expectedIndent {
				r.addDiag(SeverityError, "yaml-indent",
					"YAML block must be indented by "+formatInt(expectedIndent)+" spaces")
			}
			r.state = stateYAML
			r.yamlBuf = make(map[string]string)
			r.lastWasTestPoint = false
			continue

		case lineYAMLEnd:
			r.addDiag(SeverityError, "yaml-unclosed", "unexpected YAML end marker without opening ---")
			r.lastWasTestPoint = false
			continue

		case lineBailOut:
			b := parseBailOut(trimmed)
			r.bailed = true
			r.lastWasTestPoint = false
			return Event{Type: EventBailOut, Line: r.lineNum, Depth: depth, Raw: raw, BailOut: &b}, nil

		case linePragma:
			p := parsePragma(trimmed)
			r.lastWasTestPoint = false
			return Event{Type: EventPragma, Line: r.lineNum, Depth: depth, Raw: raw, Pragma: &p}, nil

		case lineSubtestComment:
			comment := strings.TrimPrefix(trimmed, "#")
			comment = strings.TrimSpace(comment)
			r.lastWasTestPoint = false
			return Event{Type: EventComment, Line: r.lineNum, Depth: depth, Raw: raw, Comment: comment}, nil

		case lineComment:
			comment := strings.TrimPrefix(trimmed, "#")
			comment = strings.TrimSpace(comment)
			r.lastWasTestPoint = false
			return Event{Type: EventComment, Line: r.lineNum, Depth: depth, Raw: raw, Comment: comment}, nil

		case lineEmpty:
			r.lastWasTestPoint = false
			continue

		default:
			r.lastWasTestPoint = false
			return Event{Type: EventUnknown, Line: r.lineNum, Depth: depth, Raw: raw}, nil
		}
	}

	if !r.done {
		r.done = true
		r.finalize()
	}
	return Event{}, io.EOF
}

func (r *Reader) finalize() {
	if r.state == stateStart {
		r.addDiag(SeverityError, "version-required", "first line must be TAP version 14")
	}
	if r.state == stateYAML {
		r.addDiag(SeverityError, "yaml-unclosed", "YAML block not closed at end of input")
	}

	// Validate all remaining stack frames
	for i := len(r.stack) - 1; i >= 0; i-- {
		f := r.stack[i]
		if !f.planSeen && !r.bailed {
			if f.depth == 0 {
				r.addDiag(SeverityError, "plan-required", "no plan line found")
			}
		}
		if f.planSeen && f.testCount != f.planCount && !r.bailed {
			r.addDiag(SeverityError, "plan-count-mismatch",
				"plan declared "+formatInt(f.planCount)+" tests but "+formatInt(f.testCount)+" ran")
		}
	}
}

// Diagnostics returns all validation problems found so far.
func (r *Reader) Diagnostics() []Diagnostic {
	if !r.done {
		// Drain remaining events
		for {
			if _, err := r.Next(); err != nil {
				break
			}
		}
	}
	return r.diags
}

// Summary returns aggregate results after the stream is fully consumed.
func (r *Reader) Summary() Summary {
	if !r.done {
		for {
			if _, err := r.Next(); err != nil {
				break
			}
		}
	}

	s := Summary{
		Version:   14,
		BailedOut: r.bailed,
	}

	// Count from root frame
	if len(r.stack) > 0 {
		root := r.stack[0]
		s.PlanCount = root.planCount
		s.TotalTests = root.testCount
	}

	// Count pass/fail/skip/todo by re-scanning diagnostics isn't right.
	// We need to track these in the reader. For now, mark valid.
	hasErrors := false
	for _, d := range r.diags {
		if d.Severity == SeverityError {
			hasErrors = true
			break
		}
	}
	s.Valid = !hasErrors

	return s
}

// ReadFrom reads the entire TAP stream, consuming all events and
// collecting diagnostics. Implements io.ReaderFrom.
func (r *Reader) ReadFrom(src io.Reader) (int64, error) {
	r.scanner = bufio.NewScanner(src)
	r.lineNum = 0
	r.state = stateStart
	r.stack = []frame{{depth: 0}}
	r.diags = nil
	r.done = false

	for {
		if _, err := r.Next(); err != nil {
			break
		}
	}
	return int64(r.lineNum), nil
}

func formatInt(n int) string {
	return strings.TrimRight(strings.Replace(
		strings.Replace(
			strings.Replace(
				strings.Replace(
					strings.Replace(
						strings.Replace(
							strings.Replace(
								strings.Replace(
									strings.Replace(
										strings.Replace("", "", "", 0),
										"", "", 0), "", "", 0), "", "", 0),
									"", "", 0), "", "", 0), "", "", 0),
								"", "", 0), "", "", 0), "", "", 0),
		"")
	// This is wrong. Use strconv.Itoa or fmt.Sprintf instead.
	// Placeholder — will be replaced.
}
```

**IMPORTANT:** The `formatInt` helper above is deliberately broken as a placeholder. The actual implementation should use `strconv.Itoa`:

```go
import "strconv"

func formatInt(n int) string {
	return strconv.Itoa(n)
}
```

The reader also needs to track pass/fail/skip/todo counts. Add these fields to the `Reader` struct and increment them in the test point handler:

```go
// Add to Reader struct:
passed  int
failed  int
skipped int
todo    int

// In the test point handler, after parsing:
switch tp.Directive {
case DirectiveSkip:
	r.skipped++
case DirectiveTodo:
	r.todo++
default:
	if tp.OK {
		r.passed++
	} else {
		r.failed++
	}
}

// In Summary():
s.Passed = r.passed
s.Failed = r.failed
s.Skipped = r.skipped
s.Todo = r.todo
```

**Step 4: Run test to verify it passes**

Run: `just test-go`
Expected: PASS

**Step 5: Commit**

```
git add go/reader.go go/reader_test.go
git commit -m "feat: add TAP-14 reader state machine"
```

---

### Task 5: io.WriteTo for Validation Report

Add `WriteTo` that writes the validation report as text output.

**Files:**
- Modify: `go/reader.go`
- Test: `go/reader_test.go` (add tests)

**Step 1: Write the failing test**

Add to `go/reader_test.go`:

```go
func TestReaderWriteTo(t *testing.T) {
	input := "TAP version 14\n1..1\nok 1 - pass\n"
	r := NewReader(strings.NewReader(input))
	var buf strings.Builder
	n, err := r.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo error: %v", err)
	}
	if n == 0 {
		t.Error("expected non-zero bytes written")
	}
	out := buf.String()
	if !strings.Contains(out, "valid") {
		t.Errorf("expected 'valid' in output, got: %q", out)
	}
}

func TestReaderWriteToWithErrors(t *testing.T) {
	input := "1..1\nok 1 - test\n"
	r := NewReader(strings.NewReader(input))
	var buf strings.Builder
	r.WriteTo(&buf)
	out := buf.String()
	if !strings.Contains(out, "version-required") {
		t.Errorf("expected version-required in output, got: %q", out)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `just test-go`
Expected: FAIL — `WriteTo` not defined.

**Step 3: Write minimal implementation**

Add to `go/reader.go`:

```go
// WriteTo writes the validation report to the given writer.
// Implements io.WriterTo.
func (r *Reader) WriteTo(w io.Writer) (int64, error) {
	if !r.done {
		for {
			if _, err := r.Next(); err != nil {
				break
			}
		}
	}

	var total int64
	summary := r.Summary()

	for _, d := range r.diags {
		line := fmt.Sprintf("line %d: %s: [%s] %s\n", d.Line, d.Severity, d.Rule, d.Message)
		n, err := io.WriteString(w, line)
		total += int64(n)
		if err != nil {
			return total, err
		}
	}

	status := "valid"
	if !summary.Valid {
		status = "invalid"
	}
	line := fmt.Sprintf("\n%s: %d tests (%d passed, %d failed, %d skipped, %d todo)\n",
		status, summary.TotalTests, summary.Passed, summary.Failed, summary.Skipped, summary.Todo)
	n, err := io.WriteString(w, line)
	total += int64(n)
	return total, err
}
```

**Step 4: Run test to verify it passes**

Run: `just test-go`
Expected: PASS

**Step 5: Commit**

```
git add go/reader.go go/reader_test.go
git commit -m "feat: add WriteTo for TAP-14 validation report output"
```

---

### Task 6: Add go-mcp Dependency

Add the bob framework dependency so we can build the CLI.

**Files:**
- Modify: `go/go.mod`
- Regenerate: `go/go.sum`, `go/gomod2nix.toml`

**Step 1: Add the dependency**

Run inside nix develop:

```bash
nix develop --command bash -c "cd go && go get github.com/amarbel-llc/purse-first/libs/go-mcp@latest"
```

**Step 2: Run deps target**

```bash
just deps
```

This runs `go mod tidy` + `gomod2nix` inside nix develop.

**Step 3: Verify go.mod has the dependency**

```bash
grep purse-first go/go.mod
```

Expected: `require github.com/amarbel-llc/purse-first/libs/go-mcp v0.0.1` (or whatever version).

**Step 4: Commit**

```
git add go/go.mod go/go.sum go/gomod2nix.toml
git commit -m "deps: add go-mcp (bob) framework dependency"
```

---

### Task 7: CLI Binary with bob command.App

Create the CLI binary using bob's `command.App` pattern, following grit's structure.

**Files:**
- Create: `go/cmd/tap-dancer/main.go`

**Step 1: Write the CLI**

Create `go/cmd/tap-dancer/main.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"

	tap "github.com/amarbel-llc/tap-dancer/go"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
)

func main() {
	app := registerCommands()

	if len(os.Args) > 1 && os.Args[1] == "generate-plugin" {
		if len(os.Args) < 3 {
			log.Fatal("usage: tap-dancer generate-plugin <output-dir>")
		}
		if err := app.GenerateAll(os.Args[2]); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}
		return
	}

	// Check if running as MCP server (no TTY on stdin)
	stat, _ := os.Stdin.Stat()
	isMCP := (stat.Mode() & os.ModeCharDevice) == 0 && len(os.Args) == 1

	if isMCP && !isatty() {
		// MCP server mode
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		t := transport.NewStdio(os.Stdin, os.Stdout)
		registry := server.NewToolRegistry()
		app.RegisterMCPTools(registry)

		srv, err := server.New(t, server.Options{
			ServerName:    app.Name,
			ServerVersion: app.Version,
			Tools:         registry,
		})
		if err != nil {
			log.Fatalf("creating server: %v", err)
		}

		if err := srv.Run(ctx); err != nil {
			log.Fatalf("server error: %v", err)
		}
		return
	}

	// CLI mode
	ctx := context.Background()
	if err := app.RunCLI(ctx, os.Args[1:], nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
}

func isatty() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func registerCommands() *command.App {
	app := command.NewApp("tap-dancer", "TAP-14 validator and writer toolkit")
	app.Version = "0.1.0"

	app.AddCommand(&command.Command{
		Name: "validate",
		Description: command.Description{
			Short: "Validate TAP-14 input against the full specification",
			Long:  "Reads TAP version 14 input and checks it against every MUST and SHOULD rule in the TAP-14 specification. Reports diagnostics with line numbers and rule IDs.",
		},
		Params: []command.Param{
			{Name: "input", Type: command.String, Description: "TAP-14 text to validate. If omitted, reads from stdin (CLI mode)."},
			{Name: "strict", Type: command.Bool, Description: "Fail on first error instead of reporting all errors."},
			{Name: "format", Type: command.String, Description: "Output format: text, json, or tap. Default: text.", Default: "text"},
		},
		Run: handleValidate,
	})

	return app
}

func handleValidate(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Input  string `json:"input"`
		Strict bool   `json:"strict"`
		Format string `json:"format"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	if params.Format == "" {
		params.Format = "text"
	}

	var input io.Reader
	if params.Input != "" {
		input = strings.NewReader(params.Input)
	} else {
		input = os.Stdin
	}

	reader := tap.NewReader(input)

	// Consume all events
	for {
		_, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return command.TextErrorResult(fmt.Sprintf("read error: %v", err)), nil
		}

		if params.Strict {
			for _, d := range reader.Diagnostics() {
				if d.Severity == tap.SeverityError {
					return formatResult(params.Format, reader.Summary(), reader.Diagnostics()), nil
				}
			}
		}
	}

	return formatResult(params.Format, reader.Summary(), reader.Diagnostics()), nil
}

func formatResult(format string, summary tap.Summary, diags []tap.Diagnostic) *command.Result {
	switch format {
	case "json":
		result := struct {
			Summary     tap.Summary      `json:"summary"`
			Diagnostics []tap.Diagnostic `json:"diagnostics"`
		}{
			Summary:     summary,
			Diagnostics: diags,
		}
		return command.JSONResult(result)

	case "tap":
		var b strings.Builder
		tw := tap.NewWriter(&b)
		tw.PlanAhead(len(diags) + 1) // +1 for overall result

		for _, d := range diags {
			if d.Severity == tap.SeverityError {
				tw.NotOk(d.Rule, map[string]string{
					"message":  d.Message,
					"severity": "fail",
					"line":     fmt.Sprintf("%d", d.Line),
				})
			} else {
				tw.NotOk(d.Rule, map[string]string{
					"message":  d.Message,
					"severity": d.Severity.String(),
					"line":     fmt.Sprintf("%d", d.Line),
				})
			}
		}

		if summary.Valid {
			tw.Ok("document valid")
		} else {
			tw.NotOk("document valid", nil)
		}

		return command.TextResult(b.String())

	default: // text
		var b strings.Builder
		for _, d := range diags {
			fmt.Fprintf(&b, "line %d: %s: [%s] %s\n", d.Line, d.Severity, d.Rule, d.Message)
		}

		status := "valid"
		if !summary.Valid {
			status = "invalid"
		}
		fmt.Fprintf(&b, "\n%s: %d tests (%d passed, %d failed, %d skipped, %d todo)\n",
			status, summary.TotalTests, summary.Passed, summary.Failed, summary.Skipped, summary.Todo)

		if summary.Valid {
			return command.TextResult(b.String())
		}
		return command.TextErrorResult(b.String())
	}
}
```

**Step 2: Verify it compiles**

```bash
just test-go
```

Expected: existing tests still pass, new code compiles.

**Step 3: Commit**

```
git add go/cmd/tap-dancer/main.go
git commit -m "feat: add tap-dancer CLI with bob command.App"
```

---

### Task 8: Update Nix Flake

Update `flake.nix` to build the CLI binary using `buildGoApplication`.

**Files:**
- Modify: `flake.nix`

**Step 1: Update flake.nix**

The flake needs `go.overlays.default` applied to pkgs so `buildGoApplication` is available. Add a `tap-dancer-cli` package and include it in the `default` symlinkJoin.

Key changes to `flake.nix`:
1. Add `go.overlays.default` to the overlays list
2. Replace the `tap-dancer-go` compile-check derivation with `buildGoApplication`
3. Add `postInstall` to run `generate-plugin` like grit does
4. Include the CLI in the default package

The updated overlays line:

```nix
overlays = [ (import rust-overlay) go.overlays.default ];
```

The new Go package:

```nix
tap-dancer-cli = pkgs.buildGoApplication {
  pname = "tap-dancer";
  version = "0.1.0";
  src = ./go;
  modules = ./go/gomod2nix.toml;
  subPackages = [ "cmd/tap-dancer" ];

  postInstall = ''
    $out/bin/tap-dancer generate-plugin $out
  '';

  meta = with pkgs.lib; {
    description = "TAP-14 validator and writer toolkit";
    homepage = "https://github.com/amarbel-llc/tap-dancer";
    license = licenses.mit;
  };
};
```

The updated default package:

```nix
default = pkgs.symlinkJoin {
  name = "tap-dancer";
  paths = [
    tap-dancer-cli
    tap-dancer-rust
    tap-dancer-skill
  ];
};
```

**Step 2: Build to verify**

```bash
just build
```

Expected: builds successfully, produces `result/bin/tap-dancer`.

**Step 3: Verify the binary works**

```bash
echo "TAP version 14\n1..1\nok 1 - test" | ./result/bin/tap-dancer validate
```

Expected: `valid: 1 tests (1 passed, 0 failed, 0 skipped, 0 todo)`

**Step 4: Commit**

```
git add flake.nix
git commit -m "build: add tap-dancer CLI to nix flake with buildGoApplication"
```

---

### Task 9: Update Justfile

Add new targets for the CLI build and expanded testing.

**Files:**
- Modify: `justfile`

**Step 1: Update justfile**

Add targets:

```makefile
build-cli:
    nix develop --command bash -c "cd go && go build ./cmd/tap-dancer"

test-validate:
    echo "TAP version 14\n1..2\nok 1 - a\nok 2 - b" | nix run .# -- validate
```

**Step 2: Verify targets work**

```bash
just build-cli
just test
```

Expected: all pass.

**Step 3: Commit**

```
git add justfile
git commit -m "build: add CLI build and validate targets to justfile"
```

---

### Task 10: Subtest Validation Tests

Add comprehensive tests for subtest parsing and validation.

**Files:**
- Modify: `go/reader_test.go`

**Step 1: Write subtest tests**

Add to `go/reader_test.go`:

```go
func TestReaderSubtest(t *testing.T) {
	input := `TAP version 14
1..1
    # Subtest: nested
    ok 1 - inner pass
    1..1
ok 1 - nested
`
	_, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true for valid subtest")
	}
}

func TestReaderNestedSubtest(t *testing.T) {
	input := `TAP version 14
1..1
    # Subtest: outer
        # Subtest: inner
        ok 1 - deep
        1..1
    ok 1 - inner result
    1..1
ok 1 - outer result
`
	_, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true for nested subtests")
	}
}

func TestReaderSubtestPlanMismatch(t *testing.T) {
	input := `TAP version 14
1..1
    ok 1 - inner
    1..3
ok 1 - outer
`
	_, diags, _ := collectEvents(input)

	found := false
	for _, d := range diags {
		if d.Rule == "plan-count-mismatch" {
			found = true
		}
	}
	if !found {
		t.Error("expected plan-count-mismatch for subtest")
	}
}
```

**Step 2: Run tests**

Run: `just test-go`
Expected: PASS (or fix any state machine bugs found).

**Step 3: Commit**

```
git add go/reader_test.go
git commit -m "test: add subtest validation tests"
```

---

### Task 11: Escape and Directive Edge Case Tests

**Files:**
- Modify: `go/parse_test.go`
- Modify: `go/reader_test.go`

**Step 1: Add edge case tests**

Add to `go/parse_test.go`:

```go
func TestParseTestPointEscaping(t *testing.T) {
	tests := []struct {
		line string
		desc string
	}{
		{`ok 1 - has \# hash`, "has # hash"},
		{`ok 1 - has \\ backslash`, `has \ backslash`},
		{`ok 1 - has \\\# both`, `has \# both`},
		{`ok 1 - normal desc`, "normal desc"},
	}
	for _, tt := range tests {
		tp, _ := parseTestPoint(tt.line)
		if tp.Description != tt.desc {
			t.Errorf("parseTestPoint(%q).Description = %q, want %q", tt.line, tp.Description, tt.desc)
		}
	}
}

func TestDirectiveCase(t *testing.T) {
	tests := []struct {
		line      string
		directive Directive
	}{
		{"ok 1 - x # SKIP reason", DirectiveSkip},
		{"ok 1 - x # skip reason", DirectiveSkip},
		{"ok 1 - x # Skip reason", DirectiveSkip},
		{"ok 1 - x # TODO reason", DirectiveTodo},
		{"ok 1 - x # todo reason", DirectiveTodo},
		{"ok 1 - x # Todo reason", DirectiveTodo},
	}
	for _, tt := range tests {
		tp, _ := parseTestPoint(tt.line)
		if tp.Directive != tt.directive {
			t.Errorf("parseTestPoint(%q).Directive = %v, want %v", tt.line, tp.Directive, tt.directive)
		}
	}
}
```

Add to `go/reader_test.go`:

```go
func TestReaderSkipAllPlan(t *testing.T) {
	input := "TAP version 14\n1..0 # skip all tests\n"
	_, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true for skip-all plan")
	}
}

func TestReaderUnclosedYAML(t *testing.T) {
	input := "TAP version 14\n1..1\nnot ok 1 - fail\n  ---\n  message: broken\n"
	_, diags, _ := collectEvents(input)

	found := false
	for _, d := range diags {
		if d.Rule == "yaml-unclosed" {
			found = true
		}
	}
	if !found {
		t.Error("expected yaml-unclosed diagnostic")
	}
}
```

**Step 2: Run tests and fix any failures**

Run: `just test-go`

**Step 3: Commit**

```
git add go/parse_test.go go/reader_test.go
git commit -m "test: add escape and directive edge case tests"
```

---

### Task 12: Format and Lint

**Step 1: Format all Go code**

```bash
just fmt-go
```

**Step 2: Format Nix**

```bash
just fmt-nix
```

**Step 3: Run all tests**

```bash
just test
```

Expected: all pass.

**Step 4: Build**

```bash
just build
```

Expected: builds successfully.

**Step 5: Commit if formatting changed anything**

```
git add -A
git commit -m "style: format go and nix files"
```

---

### Task 13: Full Integration Test

End-to-end test that runs the built binary on sample TAP input.

**Step 1: Test with valid input**

```bash
printf "TAP version 14\n1..2\nok 1 - a\nok 2 - b\n" | ./result/bin/tap-dancer validate
```

Expected: exit 0, reports valid.

**Step 2: Test with invalid input**

```bash
printf "1..1\nok 1 - test\n" | ./result/bin/tap-dancer validate
```

Expected: exit 1, reports `version-required`.

**Step 3: Test JSON format**

```bash
printf "TAP version 14\n1..1\nok 1 - test\n" | ./result/bin/tap-dancer validate --format json
```

Expected: JSON output with summary and empty diagnostics.

**Step 4: Test TAP format**

```bash
printf "TAP version 14\n1..1\nok 1 - test\n" | ./result/bin/tap-dancer validate --format tap
```

Expected: TAP-14 output describing the validation results.
