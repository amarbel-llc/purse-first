package tap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

type testEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"`
	Output  string    `json:"Output"`
}

type testResult struct {
	name    string
	action  string // pass, fail, skip
	elapsed float64
	output  strings.Builder
}

type packageResult struct {
	name    string
	tests   []*testResult
	testMap map[string]*testResult
	output  strings.Builder
	failed  bool
	elapsed float64
}

var fileLineRe = regexp.MustCompile(`(\w[\w_]*\.go):(\d+):`)

func parseFileLine(output string) (file string, line string) {
	m := fileLineRe.FindStringSubmatch(output)
	if m != nil {
		return m[1], m[2]
	}
	return "", ""
}

// ConvertGoTest reads go test -json events from r and writes TAP-14 to w.
// If verbose is true, passing tests include output diagnostics.
// Returns an exit code: 0 for all pass, 1 for any failure, 2 for build errors.
func ConvertGoTest(r io.Reader, w io.Writer, verbose bool) int {
	scanner := bufio.NewScanner(r)

	packages := make(map[string]*packageResult)
	var packageOrder []string

	tw := NewWriter(w)
	exitCode := 0

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var ev testEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			tw.Comment(fmt.Sprintf("unparseable: %s", line))
			continue
		}

		pkg := packages[ev.Package]
		if pkg == nil {
			pkg = &packageResult{
				name:    ev.Package,
				testMap: make(map[string]*testResult),
			}
			packages[ev.Package] = pkg
			packageOrder = append(packageOrder, ev.Package)
		}

		if ev.Test == "" {
			// Package-level event
			switch ev.Action {
			case "output":
				pkg.output.WriteString(ev.Output)
			case "pass":
				pkg.elapsed = ev.Elapsed
				emitPackage(tw, pkg, verbose)
			case "fail":
				pkg.failed = true
				pkg.elapsed = ev.Elapsed
				emitPackage(tw, pkg, verbose)
				if exitCode < 1 {
					exitCode = 1
				}
			}
			continue
		}

		// Test-level event
		tr := pkg.testMap[ev.Test]
		if tr == nil {
			tr = &testResult{name: ev.Test}
			pkg.testMap[ev.Test] = tr
			pkg.tests = append(pkg.tests, tr)
		}

		switch ev.Action {
		case "output":
			tr.output.WriteString(ev.Output)
		case "pass":
			tr.action = "pass"
			tr.elapsed = ev.Elapsed
		case "fail":
			tr.action = "fail"
			tr.elapsed = ev.Elapsed
		case "skip":
			tr.action = "skip"
			tr.elapsed = ev.Elapsed
		}
	}

	tw.Plan()
	return exitCode
}

func emitPackage(tw *Writer, pkg *packageResult, verbose bool) {
	sub := tw.Subtest(pkg.name)

	for _, tr := range pkg.tests {
		// Skip subtests -- they are emitted by their parent
		if strings.Contains(tr.name, "/") {
			continue
		}
		emitTest(sub, pkg, tr, verbose)
	}

	sub.Plan()

	if pkg.failed {
		tw.NotOk(pkg.name, nil)
	} else {
		tw.Ok(pkg.name)
	}
}

func emitTest(tw *Writer, pkg *packageResult, tr *testResult, verbose bool) {
	// Check for child subtests
	prefix := tr.name + "/"
	var children []*testResult
	for _, child := range pkg.tests {
		if strings.HasPrefix(child.name, prefix) && !strings.Contains(child.name[len(prefix):], "/") {
			children = append(children, child)
		}
	}

	if len(children) > 0 {
		sub := tw.Subtest(tr.name)
		for _, child := range children {
			emitTest(sub, pkg, child, verbose)
		}
		sub.Plan()
		if tr.action == "fail" {
			tw.NotOk(tr.name, nil)
		} else {
			tw.Ok(tr.name)
		}
		return
	}

	// Leaf test
	name := tr.name
	// For display, use just the last segment
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = tr.name[idx+1:]
	}

	output := cleanTestOutput(tr.output.String())

	switch tr.action {
	case "pass":
		tw.Ok(name)
	case "fail":
		diag := map[string]string{
			"elapsed": fmt.Sprintf("%.3f", tr.elapsed),
			"package": pkg.name,
		}
		if output != "" {
			diag["message"] = output
		}
		file, line := parseFileLine(output)
		if file != "" {
			diag["file"] = file
			diag["line"] = line
		}
		tw.NotOk(name, diag)
	case "skip":
		reason := extractSkipReason(output)
		tw.Skip(name, reason)
	default:
		tw.Ok(name)
	}
}

func cleanTestOutput(raw string) string {
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		// Skip go test framework lines
		if strings.HasPrefix(trimmed, "=== RUN") ||
			strings.HasPrefix(trimmed, "=== PAUSE") ||
			strings.HasPrefix(trimmed, "=== CONT") ||
			strings.HasPrefix(trimmed, "--- PASS") ||
			strings.HasPrefix(trimmed, "--- FAIL") ||
			strings.HasPrefix(trimmed, "--- SKIP") ||
			trimmed == "PASS" || trimmed == "FAIL" ||
			trimmed == "" {
			continue
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	return strings.Join(lines, "\n")
}

func extractSkipReason(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--- SKIP") {
			continue
		}
		if trimmed != "" &&
			!strings.HasPrefix(trimmed, "=== RUN") &&
			!strings.HasPrefix(trimmed, "=== PAUSE") &&
			!strings.HasPrefix(trimmed, "=== CONT") {
			return trimmed
		}
	}
	return ""
}
