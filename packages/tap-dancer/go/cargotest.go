package tap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// cargoEvent represents a JSON event from `cargo test -- --format json`.
type cargoEvent struct {
	Type        string  `json:"type"`
	Event       string  `json:"event"`
	Name        string  `json:"name"`
	TestCount   int     `json:"test_count"`
	ExecTime    float64 `json:"exec_time"`
	Stdout      string  `json:"stdout"`
	Passed      int     `json:"passed"`
	Failed      int     `json:"failed"`
	Ignored     int     `json:"ignored"`
	Measured    int     `json:"measured"`
	FilteredOut int     `json:"filtered_out"`
}

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

// ConvertCargoTest reads cargo test --format json events from r and writes TAP-14 to w.
// If verbose is true, passing tests include output diagnostics.
// If skipEmpty is true, suites with no tests emit a SKIP directive instead of not ok.
// Returns an exit code: 0 for all pass, 1 for any failure.
func ConvertCargoTest(r io.Reader, w io.Writer, verbose bool, skipEmpty bool) int {
	scanner := bufio.NewScanner(r)
	tw := NewWriter(w)
	exitCode := 0

	var suiteCount int
	var current *cargoSuiteResult

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var ev cargoEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// Non-JSON line -- check if it names a test binary
			name := parseCargoBinaryLine(line)
			if name != "" && current != nil {
				current.name = name
			} else if name != "" {
				current = &cargoSuiteResult{name: name}
			} else {
				tw.Comment(fmt.Sprintf("unparseable: %s", line))
			}
			continue
		}

		switch ev.Type {
		case "suite":
			switch ev.Event {
			case "started":
				if current == nil {
					current = &cargoSuiteResult{}
				}
				current.testCount = ev.TestCount
				if current.name == "" {
					current.name = fmt.Sprintf("suite-%d", suiteCount+1)
				}
			case "ok", "failed":
				if current == nil {
					continue
				}
				current.elapsed = ev.ExecTime
				current.failed = ev.Event == "failed"
				suiteCount++

				emitCargoSuite(tw, current, verbose, skipEmpty)
				if current.failed && exitCode < 1 {
					exitCode = 1
				}
				if current.testCount == 0 && !skipEmpty && exitCode < 1 {
					exitCode = 1
				}
				current = nil
			}

		case "test":
			if current == nil {
				continue
			}
			switch ev.Event {
			case "started":
				// nothing to track yet
			case "ok", "failed", "ignored":
				current.tests = append(current.tests, &cargoTestResult{
					name:    ev.Name,
					event:   ev.Event,
					stdout:  ev.Stdout,
					elapsed: ev.ExecTime,
				})
			}
		}
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
		diag := map[string]string{
			"elapsed": fmt.Sprintf("%.3f", tr.elapsed),
		}
		stdout := strings.TrimSpace(tr.stdout)
		if stdout != "" {
			diag["message"] = stdout
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
