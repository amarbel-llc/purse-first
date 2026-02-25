package tap

import (
	"fmt"
	"io"
	"iter"
	"sort"
	"strings"
)

type Writer struct {
	w           io.Writer
	n           int
	depth       int
	planEmitted bool
}

func NewWriter(w io.Writer) *Writer {
	fmt.Fprintln(w, "TAP version 14")
	return &Writer{w: w}
}

func (tw *Writer) Ok(description string) int {
	tw.n++
	fmt.Fprintf(tw.w, "ok %d - %s\n", tw.n, description)
	return tw.n
}

func (tw *Writer) OkDiag(description string, diagnostics *Diagnostics) int {
	tw.n++
	fmt.Fprintf(tw.w, "ok %d - %s\n", tw.n, description)
	writeDiagnostics(tw.w, diagnostics)
	return tw.n
}

func (tw *Writer) NotOk(description string, diagnostics map[string]string) int {
	tw.n++
	fmt.Fprintf(tw.w, "not ok %d - %s\n", tw.n, description)
	if len(diagnostics) > 0 {
		fmt.Fprintln(tw.w, "  ---")
		keys := make([]string, 0, len(diagnostics))
		for k := range diagnostics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := diagnostics[k]
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
	fmt.Fprintf(tw.w, "ok %d - %s # SKIP %s\n", tw.n, description, reason)
	return tw.n
}

func (tw *Writer) Todo(description, reason string) int {
	tw.n++
	fmt.Fprintf(tw.w, "not ok %d - %s # TODO %s\n", tw.n, description, reason)
	return tw.n
}

func (tw *Writer) PlanAhead(n int) {
	fmt.Fprintf(tw.w, "1..%d\n", n)
	tw.planEmitted = true
}

func (tw *Writer) Plan() {
	if tw.planEmitted {
		return
	}
	tw.planEmitted = true
	fmt.Fprintf(tw.w, "1..%d\n", tw.n)
}

func (tw *Writer) BailOut(reason string) {
	fmt.Fprintf(tw.w, "Bail out! %s\n", reason)
}

func (tw *Writer) Comment(text string) {
	fmt.Fprintf(tw.w, "# %s\n", text)
}

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
	return &Writer{w: iw, depth: tw.depth + 1}
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
