// go-mcp-docs generates section 7 manpages for the go-mcp library.
// It is a build-time-only binary, not shipped to users.
//
// Usage:
//
//	go-mcp-docs <output-dir>
package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"

	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/command"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: go-mcp-docs <output-dir>")
	}
	dir := os.Args[1]

	app := command.NewUtility("go-mcp", "Zero-dependency Go library for MCP servers and CLI tools")
	app.Version = "0.0.9"

	entries, err := fs.ReadDir(manpageFS, "doc")
	if err != nil {
		log.Fatalf("reading embedded doc directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		app.ExtraManpages = append(app.ExtraManpages, command.ManpageFile{
			Source:  manpageFS,
			Path:    "doc/" + entry.Name(),
			Section: 7,
			Name:    entry.Name(),
		})
	}

	if err := app.InstallExtraManpages(dir); err != nil {
		log.Fatalf("installing extra manpages: %v", err)
	}

	fmt.Fprintf(os.Stderr, "installed %d manpages in %s\n", len(app.ExtraManpages), dir)
}
