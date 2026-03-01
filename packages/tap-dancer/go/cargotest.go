package tap

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type cargoTestResult struct {
	name    string
	event   string // ok, failed, ignored
	stdout  string
	elapsed float64
}

type cargoSuiteResult struct {
	name      string
	tests     []*cargoTestResult
	testCount int
	failed    bool
	elapsed   float64
}

var rustFileLineRe = regexp.MustCompile(`([\w][\w_/]*\.rs):(\d+):`)

func parseRustFileLine(output string) (file string, line string) {
	m := rustFileLineRe.FindStringSubmatch(output)
	if m != nil {
		return m[1], m[2]
	}
	return "", ""
}

// Patterns for cargo test pretty output.
var (
	// "running 36 tests" or "running 0 tests" or "running 1 test"
	runningTestsRe = regexp.MustCompile(`^running (\d+) tests?$`)
	// "test tests::test_a ... ok" / "test tests::test_b ... FAILED" / "test tests::test_c ... ignored"
	testResultRe = regexp.MustCompile(`^test (.+) \.\.\. (ok|FAILED|ignored)$`)
	// "test result: ok. 36 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s"
	testSummaryRe = regexp.MustCompile(`^test result: (ok|FAILED)\. (\d+) passed; (\d+) failed; (\d+) ignored;`)
	// "---- tests::test_fail stdout ----"
	failureStdoutHeaderRe = regexp.MustCompile(`^---- (.+) stdout ----$`)
)

// ConvertCargoTest reads cargo test pretty output from r and writes TAP-14 to w.
// If verbose is true, passing tests include output diagnostics.
// If skipEmpty is true, suites with no tests emit a SKIP directive instead of not ok.
// If color is true, ok/not ok keywords are ANSI-colorized.
// Returns an exit code: 0 for all pass, 1 for any failure.
func ConvertCargoTest(r io.Reader, w io.Writer, verbose bool, skipEmpty bool, color bool) int {
	scanner := bufio.NewScanner(r)
	tw := NewColorWriter(w, color)
	tw.Pragma("streamed-output", true)
	exitCode := 0

	var suiteCount int
	var current *cargoSuiteResult

	// Failure stdout is printed in a block after all tests run but before the
	// summary line. We collect it per-test, then attach to the test results.
	failureStdout := make(map[string]string)
	var capturingFailure string

	for scanner.Scan() {
		line := scanner.Text()

		// Check for suite binary name lines first.
		if name := parseCargoBinaryLine(line); name != "" {
			// If we have a pending suite waiting for results, that shouldn't
			// happen in well-formed output — but handle it gracefully.
			if current != nil && current.testCount >= 0 {
				// Suite from previous binary without a result line — emit what we have.
			}
			current = &cargoSuiteResult{name: name}
			capturingFailure = ""
			continue
		}

		// "running N tests"
		if m := runningTestsRe.FindStringSubmatch(line); m != nil {
			if current == nil {
				current = &cargoSuiteResult{}
			}
			fmt.Sscanf(m[1], "%d", &current.testCount)
			if current.name == "" {
				current.name = fmt.Sprintf("suite-%d", suiteCount+1)
			}
			capturingFailure = ""
			continue
		}

		// "test <name> ... ok/FAILED/ignored"
		if m := testResultRe.FindStringSubmatch(line); m != nil {
			if current == nil {
				continue
			}
			event := m[2]
			switch event {
			case "FAILED":
				event = "failed"
			case "ignored":
				// already lowercase
			default:
				event = "ok"
			}
			current.tests = append(current.tests, &cargoTestResult{
				name:  m[1],
				event: event,
			})
			continue
		}

		// Failure stdout capture: "---- tests::test_fail stdout ----"
		if m := failureStdoutHeaderRe.FindStringSubmatch(line); m != nil {
			capturingFailure = m[1]
			failureStdout[capturingFailure] = ""
			continue
		}

		// If we're capturing failure stdout, accumulate lines until we hit
		// another failure header, the "failures:" marker, or the summary.
		if capturingFailure != "" {
			if line == "failures:" || testSummaryRe.MatchString(line) {
				capturingFailure = ""
				// Fall through to handle these lines below.
			} else {
				if failureStdout[capturingFailure] != "" {
					failureStdout[capturingFailure] += "\n"
				}
				failureStdout[capturingFailure] += line
				continue
			}
		}

		// "test result: ok/FAILED. N passed; N failed; ..."
		if m := testSummaryRe.FindStringSubmatch(line); m != nil {
			if current == nil {
				continue
			}
			current.failed = m[1] == "FAILED"
			suiteCount++

			// Attach failure stdout to test results.
			for _, tr := range current.tests {
				if stdout, ok := failureStdout[tr.name]; ok {
					tr.stdout = stdout
				}
			}
			failureStdout = make(map[string]string)

			emitCargoSuite(tw, current, verbose, skipEmpty)
			if current.failed && exitCode < 1 {
				exitCode = 1
			}
			if current.testCount == 0 && !skipEmpty && exitCode < 1 {
				exitCode = 1
			}
			current = nil
			continue
		}

		// "failures:" section listing test names — skip these lines.
		// Other unrecognized lines (blank, compiler output, etc.) — skip.
	}

	tw.Plan()
	return exitCode
}

func parseCargoBinaryLine(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "Running ") {
		rest := strings.TrimPrefix(line, "Running ")
		if idx := strings.Index(rest, " ("); idx > 0 {
			return rest[:idx]
		}
		return rest
	}
	if strings.HasPrefix(line, "Doc-tests ") {
		return line
	}
	return ""
}

func emitCargoSuite(tw *Writer, suite *cargoSuiteResult, verbose bool, skipEmpty bool) {
	if len(suite.tests) == 0 {
		if skipEmpty {
			tw.Skip(suite.name, "no tests")
		} else {
			tw.NotOk(suite.name, nil)
		}
		return
	}

	sub := tw.Subtest(suite.name)
	for _, tr := range suite.tests {
		emitCargoTest(sub, tr, verbose)
	}
	sub.Plan()

	if suite.failed {
		tw.NotOk(suite.name, nil)
	} else {
		tw.Ok(suite.name)
	}
}

func emitCargoTest(tw *Writer, tr *cargoTestResult, verbose bool) {
	switch tr.event {
	case "ok":
		tw.Ok(tr.name)
	case "failed":
		stdout := strings.TrimSpace(tr.stdout)
		if stdout != "" {
			for _, line := range strings.Split(stdout, "\n") {
				if line != "" {
					tw.StreamedOutput(line)
				}
			}
		}
		diag := map[string]string{}
		if stdout != "" {
			file, line := parseRustFileLine(stdout)
			if file != "" {
				diag["file"] = file
				diag["line"] = line
			}
		}
		tw.NotOk(tr.name, diag)
	case "ignored":
		tw.Skip(tr.name, "ignored")
	default:
		tw.Ok(tr.name)
	}
}
