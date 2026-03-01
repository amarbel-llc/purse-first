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
// If skipEmpty is true, packages with no tests emit a SKIP directive instead of not ok.
// If color is true, ok/not ok keywords are ANSI-colorized.
// Returns an exit code: 0 for all pass, 1 for any failure, 2 for build errors.
func ConvertGoTest(r io.Reader, w io.Writer, verbose bool, skipEmpty bool, color bool) int {
	// TODO wrap with bufio.Reader
	scanner := bufio.NewScanner(r)

	packages := make(map[string]*packageResult)
	var packageOrder []string

	tw := NewColorWriter(w, color)
	tw.Pragma("streamed-output", true)
	exitCode := 0

	// TODO switch to json.Decoder
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var ev testEvent
		// TODO extract / prepare package full path
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
				if len(pkg.tests) == 0 {
					emitEmptyPackage(tw, pkg, skipEmpty)
					if !skipEmpty && exitCode < 1 {
						exitCode = 1
					}
				} else {
					emitPackage(tw, pkg, verbose)
				}
			case "fail":
				pkg.failed = true
				pkg.elapsed = ev.Elapsed
				emitPackage(tw, pkg, verbose)
				if exitCode < 1 {
					exitCode = 1
				}
			case "skip":
				pkg.elapsed = ev.Elapsed
				emitEmptyPackage(tw, pkg, skipEmpty)
				if !skipEmpty && exitCode < 1 {
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

func emitEmptyPackage(tw *Writer, pkg *packageResult, skipEmpty bool) {
	if skipEmpty {
		reason := emptyPackageReason(pkg.output.String())
		tw.Skip(pkg.name, reason)
	} else {
		tw.NotOk(pkg.name, nil)
	}
}

func emptyPackageReason(output string) string {
	if strings.Contains(output, "[no test files]") {
		return "no test files"
	}
	if strings.Contains(output, "no tests to run") {
		return "no tests to run"
	}
	return "no tests"
}

func emitPackage(tapWriter *Writer, pkg *packageResult, verbose bool) {
	subtest := tapWriter.Subtest(pkg.name)

	for _, testResult := range pkg.tests {
		// Skip subtests -- they are emitted by their parent
		if strings.Contains(testResult.name, "/") {
			continue
		}

		emitTest(subtest, pkg, testResult, verbose)
	}

	subtest.Plan()

	if pkg.failed {
		tapWriter.NotOk(pkg.name, nil)
	} else {
		tapWriter.Ok(pkg.name)
	}
}

func emitTest(tapWriter *Writer, pkg *packageResult, testRezult *testResult, verbose bool) {
	// Check for child subtests
	prefix := testRezult.name + "/"
	var children []*testResult
	for _, child := range pkg.tests {
		if strings.HasPrefix(child.name, prefix) && !strings.Contains(child.name[len(prefix):], "/") {
			children = append(children, child)
		}
	}

	if len(children) > 0 {
		sub := tapWriter.Subtest(testRezult.name)
		for _, child := range children {
			emitTest(sub, pkg, child, verbose)
		}
		sub.Plan()
		if testRezult.action == "fail" {
			tapWriter.NotOk(testRezult.name, nil)
		} else {
			tapWriter.Ok(testRezult.name)
		}
		return
	}

	// Leaf test
	name := testRezult.name
	// For display, use just the last segment
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = testRezult.name[idx+1:]
	}

	output := cleanTestOutput(testRezult.output.String())

	switch testRezult.action {
	case "pass":
		if verbose && output != "" {
			for _, line := range strings.Split(output, "\n") {
				if line != "" {
					tapWriter.StreamedOutput(line)
				}
			}
		}
		tapWriter.Ok(name)
	case "fail":
		if output != "" {
			for _, line := range strings.Split(output, "\n") {
				if line != "" {
					tapWriter.StreamedOutput(line)
				}
			}
		}
		diag := map[string]string{
			"elapsed": fmt.Sprintf("%.3f", testRezult.elapsed),
			"package": pkg.name,
		}
		file, line := parseFileLine(output)
		if file != "" {
			diag["file"] = file
			diag["line"] = line
		}
		tapWriter.NotOk(name, diag)
	case "skip":
		reason := extractSkipReason(output)
		tapWriter.Skip(name, reason)
	default:
		tapWriter.Ok(name)
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
