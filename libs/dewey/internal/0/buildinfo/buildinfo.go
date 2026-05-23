// Package buildinfo holds build-time variables shared across dewey's
// command binaries. Values are injected at link time via -ldflags -X.
package buildinfo

import (
	"fmt"
	"io"
	"path/filepath"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

// Print writes the eng-versioning(7) version-subcommand output:
// a self-identification line, a blank line, then the component table
// header. Dewey binaries pin no downstreams, so the table is empty.
func Print(w io.Writer, argv0 string) {
	name := filepath.Base(argv0)
	fmt.Fprintf(w, "%s %s+%s\n\n", name, Version, Commit)
	fmt.Fprintln(w, "COMPONENT            VERSION      REV")
}
