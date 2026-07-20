// Command golangci-lint-dewey is the dewey custom golangci-lint binary:
// stock golangci-lint with dewey's gclplugin module plugin linked in.
//
// It replaces the `golangci-lint custom` build (which wants network and
// module resolution at build time) with the plain Go main that command
// generates anyway: blank-import the plugin module, then hand off to
// golangci-lint's public command entrypoint. Because the plugin
// and the binary compile together against the single golangci-lint
// version pinned in this module's go.mod, the module-plugin ABI
// constraint (plugin and binary must share a golangci-lint module
// version) holds by construction.
//
// This is a standalone module, deliberately NOT in go.work: golangci-lint
// is a lint tool's dependency closure, not a product dependency, so it is
// vendored through this directory's own gomod2nix.toml instead of the
// shared workspace lockfile. See purse-first#134.
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/golangci/golangci-lint/v2/pkg/commands"
	"github.com/golangci/golangci-lint/v2/pkg/exitcodes"

	_ "code.linenisgreat.com/purse-first/libs/dewey/gclplugin"
)

// Populated at build time via -ldflags (see gomod.nix).
var (
	version = "unknown"
	commit  = "?"
	date    = ""
)

func main() {
	info := commands.BuildInfo{
		GoVersion: runtime.Version(),
		Version:   version,
		Commit:    commit,
		Date:      date,
	}

	if err := commands.Execute(info); err != nil {
		fmt.Fprintf(os.Stderr, "The command is terminated due to an error: %v\n", err)
		os.Exit(exitcodes.Failure)
	}
}
