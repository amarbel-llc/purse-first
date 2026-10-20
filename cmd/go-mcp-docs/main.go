// go-mcp-docs generates section 7 manpages for the go-mcp library.
// It is a build-time-only binary, not shipped to users.
//
// Usage:
//
//	go-mcp-docs <output-dir>
//
// The embedded pages carry the placeholder @VERSION@ in their .TH header;
// go-mcp-docs substitutes the version stamped at link time (-X main.version,
// from the repo-root version.env) when it installs them.
package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// version is injected via -ldflags "-X main.version=..." by gomod.nix from
// version.env. A plain go build leaves it at "dev".
var version = "dev"

const versionPlaceholder = "@VERSION@"

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: go-mcp-docs <output-dir>")
	}
	dir := os.Args[1]

	entries, err := fs.ReadDir(manpageFS, "doc")
	if err != nil {
		log.Fatalf("reading embedded doc directory: %v", err)
	}

	manDir := filepath.Join(dir, "share", "man", "man7")
	if err := os.MkdirAll(manDir, 0o755); err != nil {
		log.Fatalf("creating %s: %v", manDir, err)
	}

	installed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := fs.ReadFile(manpageFS, "doc/"+entry.Name())
		if err != nil {
			log.Fatalf("reading %s: %v", entry.Name(), err)
		}

		page := string(data)
		if !strings.Contains(page, versionPlaceholder) {
			log.Fatalf("%s: missing %s in its .TH header", entry.Name(), versionPlaceholder)
		}
		page = strings.ReplaceAll(page, versionPlaceholder, version)

		dst := filepath.Join(manDir, entry.Name())
		if err := os.WriteFile(dst, []byte(page), 0o644); err != nil {
			log.Fatalf("writing %s: %v", dst, err)
		}
		installed++
	}

	fmt.Fprintf(os.Stderr, "installed %d manpages in %s\n", installed, dir)
}
