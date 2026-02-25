package tap

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Writer struct {
	w      io.Writer
	n      int
	failed bool
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
	tw.failed = true
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

func (tw *Writer) PlanAhead(n int) {
	fmt.Fprintf(tw.w, "1..%d\n", n)
}

func (tw *Writer) Plan() {
	fmt.Fprintf(tw.w, "1..%d\n", tw.n)
}

func (tw *Writer) HasFailures() bool {
	return tw.failed
}

// Subtest creates a child Writer that buffers its output. The child does NOT
// emit a TAP version header (subtests omit it per TAP-14 spec). Call
// EndSubtest on the parent when done.
func (tw *Writer) Subtest(name string) *Writer {
	return &Writer{w: &bytes.Buffer{}}
}

// EndSubtest writes the buffered subtest output (indented 4 spaces) under a
// "# Subtest:" comment, then emits the parent test point as ok/not ok based
// on whether the subtest had failures.
func (tw *Writer) EndSubtest(name string, sub *Writer) int {
	buf := sub.w.(*bytes.Buffer)

	fmt.Fprintf(tw.w, "    # Subtest: %s\n", name)
	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}
		fmt.Fprintf(tw.w, "    %s\n", line)
	}

	tw.n++
	if sub.HasFailures() {
		fmt.Fprintf(tw.w, "not ok %d - %s\n", tw.n, name)
		tw.failed = true
	} else {
		fmt.Fprintf(tw.w, "ok %d - %s\n", tw.n, name)
	}
	return tw.n
}
