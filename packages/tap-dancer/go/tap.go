package tap

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type Writer struct {
	w     io.Writer
	n     int
	depth int
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
}

func (tw *Writer) Plan() {
	fmt.Fprintf(tw.w, "1..%d\n", tw.n)
}

func (tw *Writer) BailOut(reason string) {
	fmt.Fprintf(tw.w, "Bail out! %s\n", reason)
}

func (tw *Writer) Comment(text string) {
	fmt.Fprintf(tw.w, "# %s\n", text)
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
