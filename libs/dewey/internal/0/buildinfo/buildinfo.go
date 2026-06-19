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

// Print writes the eng-versioning(7) version-subcommand output. Dewey
// binaries pin no downstream components, so per the spec they emit only
// the self-identification line — no blank line, no component table:
//
//	name VERSION+COMMIT
func Print(w io.Writer, argv0 string) {
	name := filepath.Base(argv0)
	fmt.Fprintf(w, "%s %s+%s\n", name, Version, Commit)
}
