package tap

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
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
	scanner          *bufio.Scanner
	state            readerState
	lineNum          int
	stack            []frame
	diags            []Diagnostic
	done             bool
	bailed           bool
	yamlBuf          map[string]string
	lastWasTestPoint bool
	passed           int
	failed           int
	skipped          int
	todo             int
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
		}
		for depth < r.currentFrame().depth && len(r.stack) > 1 {
			completed := r.stack[len(r.stack)-1]
			r.stack = r.stack[:len(r.stack)-1]
			if completed.planSeen && completed.testCount != completed.planCount {
				r.addDiag(SeverityError, "plan-count-mismatch",
					"subtest plan count mismatch: plan declared "+
						strconv.Itoa(completed.planCount)+
						" tests but "+strconv.Itoa(completed.testCount)+" ran")
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
						"test number "+strconv.Itoa(tp.Number)+" out of sequence, expected "+strconv.Itoa(f.lastTestNumber+1))
				}
				f.lastTestNumber = tp.Number
			}

			// Track pass/fail/skip/todo
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

			r.lastWasTestPoint = true
			return Event{Type: EventTestPoint, Line: r.lineNum, Depth: depth, Raw: raw, TestPoint: &tp}, nil

		case lineYAMLStart:
			if !r.lastWasTestPoint {
				r.addDiag(SeverityWarning, "yaml-orphan", "YAML block not following a test point")
			}
			expectedIndent := (r.currentFrame().depth * 4) + 2
			if indent != expectedIndent {
				r.addDiag(SeverityError, "yaml-indent",
					"YAML block must be indented by "+strconv.Itoa(expectedIndent)+" spaces")
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
				"plan declared "+strconv.Itoa(f.planCount)+" tests but "+strconv.Itoa(f.testCount)+" ran")
		}
	}
}

// Diagnostics returns all validation problems found so far.
func (r *Reader) Diagnostics() []Diagnostic {
	if !r.done {
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
		Passed:    r.passed,
		Failed:    r.failed,
		Skipped:   r.skipped,
		Todo:      r.todo,
	}

	if len(r.stack) > 0 {
		root := r.stack[0]
		s.PlanCount = root.planCount
		s.TotalTests = root.testCount
	}

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
// collecting diagnostics.
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

// WriteTo writes the validation report to the given writer.
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
