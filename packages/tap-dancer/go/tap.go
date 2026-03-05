package tap

import (
	"fmt"
	"io"
	"iter"
	"sort"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// ANSI color codes for TTY output.
const (
	ansiGreen  = "\033[32m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiReset  = "\033[0m"
)

type Writer struct {
	w           io.Writer
	n           int
	depth       int
	planEmitted bool
	failed      bool
	color       bool
	locale      language.Tag
	printer     *message.Printer
}

func NewWriter(w io.Writer) *Writer {
	fmt.Fprintln(w, "TAP version 14")
	return &Writer{w: w}
}

// NewColorWriter creates a Writer that colorizes ok/not ok when color is true.
func NewColorWriter(w io.Writer, color bool) *Writer {
	fmt.Fprintln(w, "TAP version 14")
	return &Writer{w: w, color: color}
}

func NewLocaleWriter(w io.Writer, locale language.Tag) *Writer {
	fmt.Fprintln(w, "TAP version 14")
	fmt.Fprintf(w, "pragma +locale-formatting:%s\n", locale)
	return &Writer{
		w:       w,
		locale:  locale,
		printer: message.NewPrinter(locale),
	}
}

func (tw *Writer) formatNumber(n int) string {
	if tw.printer != nil {
		return tw.printer.Sprintf("%d", n)
	}
	return fmt.Sprintf("%d", n)
}

func (tw *Writer) colorOk() string {
	if tw.color {
		return ansiGreen + "ok" + ansiReset
	}
	return "ok"
}

func (tw *Writer) colorNotOk() string {
	if tw.color {
		return ansiRed + "not ok" + ansiReset
	}
	return "not ok"
}

func (tw *Writer) colorSkip() string {
	if tw.color {
		return ansiYellow + "# SKIP" + ansiReset
	}
	return "# SKIP"
}

func (tw *Writer) colorTodo() string {
	if tw.color {
		return ansiYellow + "# TODO" + ansiReset
	}
	return "# TODO"
}

func (tw *Writer) colorBailOut() string {
	if tw.color {
		return ansiRed + "Bail out!" + ansiReset
	}
	return "Bail out!"
}

func (tw *Writer) Ok(description string) int {
	tw.n++
	fmt.Fprintf(tw.w, "%s %s - %s\n", tw.colorOk(), tw.formatNumber(tw.n), description)
	return tw.n
}

func (tw *Writer) OkDiag(description string, diagnostics *Diagnostics) int {
	tw.n++
	fmt.Fprintf(tw.w, "%s %s - %s\n", tw.colorOk(), tw.formatNumber(tw.n), description)
	writeDiagnostics(tw.w, diagnostics, tw.color)
	return tw.n
}

func (tw *Writer) HasFailures() bool {
	return tw.failed
}

func (tw *Writer) NotOk(description string, diagnostics map[string]string) int {
	tw.n++
	tw.failed = true
	fmt.Fprintf(tw.w, "%s %s - %s\n", tw.colorNotOk(), tw.formatNumber(tw.n), description)
	if len(diagnostics) > 0 {
		fmt.Fprintln(tw.w, "  ---")
		keys := make([]string, 0, len(diagnostics))
		for k := range diagnostics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := sanitizeYAMLValue(diagnostics[k], tw.color)
			if strings.Contains(v, "\n") {
				fmt.Fprintf(tw.w, "  %s: |\n", k)
				lines := strings.Split(v, "\n")
				for len(lines) > 0 && lines[len(lines)-1] == "" {
					lines = lines[:len(lines)-1]
				}
				for _, line := range lines {
					fmt.Fprintf(tw.w, "    %s\n", line)
				}
			} else {
				fmt.Fprintf(tw.w, "  %s: %s\n", k, v)
			}
		}
		fmt.Fprintln(tw.w, "  ...")
	}
	return tw.n
}

func (tw *Writer) Skip(description, reason string) int {
	tw.n++
	fmt.Fprintf(tw.w, "%s %s - %s %s %s\n", tw.colorOk(), tw.formatNumber(tw.n), description, tw.colorSkip(), reason)
	return tw.n
}

func (tw *Writer) SkipDiag(description, reason string, diagnostics *Diagnostics) int {
	tw.n++
	fmt.Fprintf(tw.w, "%s %s - %s %s %s\n", tw.colorOk(), tw.formatNumber(tw.n), description, tw.colorSkip(), reason)
	writeDiagnostics(tw.w, diagnostics, tw.color)
	return tw.n
}

func (tw *Writer) Todo(description, reason string) int {
	tw.n++
	fmt.Fprintf(tw.w, "%s %s - %s %s %s\n", tw.colorNotOk(), tw.formatNumber(tw.n), description, tw.colorTodo(), reason)
	return tw.n
}

func (tw *Writer) PlanAhead(n int) {
	fmt.Fprintf(tw.w, "1..%s\n", tw.formatNumber(n))
	tw.planEmitted = true
}

func (tw *Writer) Plan() {
	if tw.planEmitted {
		return
	}
	tw.planEmitted = true
	fmt.Fprintf(tw.w, "1..%s\n", tw.formatNumber(tw.n))
}

func (tw *Writer) BailOut(reason string) {
	fmt.Fprintf(tw.w, "%s %s\n", tw.colorBailOut(), reason)
}

func (tw *Writer) Comment(text string) {
	fmt.Fprintf(tw.w, "# %s\n", text)
}

func (tw *Writer) Pragma(key string, enabled bool) {
	sign := "-"
	if enabled {
		sign = "+"
	}
	fmt.Fprintf(tw.w, "pragma %s%s\n", sign, key)
}

func (tw *Writer) StreamedOutput(text string) {
	fmt.Fprintf(tw.w, "# %s\n", text)
}

type Diagnostics struct {
	Message  string
	Severity string
	File     string
	Line     int
	Extras   map[string]any
}

func sanitizeYAMLValue(value string, color bool) string {
	if color {
		return stripNonSGR(value)
	}
	return stripANSI(value)
}

func writeDiagnostics(w io.Writer, d *Diagnostics, color bool) {
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
		entries = append(entries, struct{ k, v string }{"message", sanitizeYAMLValue(d.Message, color)})
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
			entries = append(entries, struct{ k, v string }{k, sanitizeYAMLValue(fmt.Sprintf("%v", d.Extras[k]), color)})
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

type indentWriter struct {
	w      io.Writer
	prefix string
}

func (iw *indentWriter) Write(p []byte) (int, error) {
	lines := strings.Split(string(p), "\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			break
		}
		out := iw.prefix + line + "\n"
		if _, err := iw.w.Write([]byte(out)); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (tw *Writer) Subtest(name string) *Writer {
	prefix := "    "
	fmt.Fprintf(tw.w, "%s# Subtest: %s\n", prefix, name)
	iw := &indentWriter{w: tw.w, prefix: prefix}
	child := &Writer{
		w:       iw,
		depth:   tw.depth + 1,
		color:   tw.color,
		locale:  tw.locale,
		printer: tw.printer,
	}
	if tw.printer != nil {
		fmt.Fprintf(iw, "pragma +locale-formatting:%s\n", tw.locale)
	}
	return child
}

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
			tw.SkipDiag(tp.Description, tp.Skip, tp.Diagnostics)
		} else if tp.Todo != "" {
			tw.Todo(tp.Description, tp.Todo)
		} else if tp.Ok {
			tw.n++
			fmt.Fprintf(tw.w, "%s %s - %s\n", tw.colorOk(), tw.formatNumber(tw.n), tp.Description)
			writeDiagnostics(tw.w, tp.Diagnostics, tw.color)
		} else {
			tw.n++
			tw.failed = true
			fmt.Fprintf(tw.w, "%s %s - %s\n", tw.colorNotOk(), tw.formatNumber(tw.n), tp.Description)
			writeDiagnostics(tw.w, tp.Diagnostics, tw.color)
		}
	}
	if !tw.planEmitted {
		tw.Plan()
	}
}
