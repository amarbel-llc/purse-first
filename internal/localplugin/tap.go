package localplugin

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// tapWriter is a minimal TAP-14 writer for progress output.
// It replaces the external tap-dancer/go dependency with just the
// subset needed by localplugin: Ok, NotOk, Skip, PlanAhead.
type tapWriter struct {
	w io.Writer
	n int
}

func newTAPWriter(w io.Writer) *tapWriter {
	fmt.Fprintln(w, "TAP version 14")
	return &tapWriter{w: w}
}

func (tw *tapWriter) PlanAhead(n int) {
	fmt.Fprintf(tw.w, "1..%d\n", n)
}

func (tw *tapWriter) Ok(description string) {
	tw.n++
	fmt.Fprintf(tw.w, "ok %d - %s\n", tw.n, description)
}

func (tw *tapWriter) NotOk(description string, diagnostics map[string]string) {
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
				for _, line := range strings.Split(v, "\n") {
					fmt.Fprintf(tw.w, "    %s\n", line)
				}
			} else {
				fmt.Fprintf(tw.w, "  %s: %s\n", k, v)
			}
		}

		fmt.Fprintln(tw.w, "  ...")
	}
}

func (tw *tapWriter) Skip(description, reason string) {
	tw.n++
	fmt.Fprintf(tw.w, "ok %d - %s # SKIP %s\n", tw.n, description, reason)
}
