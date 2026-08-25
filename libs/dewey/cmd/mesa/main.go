// Command mesa renders a List-Table NDJSON stream (RFC 0003).
//
// It reads the NDJSON stream defined by RFC 0003 on stdin and renders it:
// styled (colored, bordered) when stdout is a terminal, plain TAB-separated
// text otherwise. It is the out-of-process front door that non-Go producers
// pipe into.
//
//	producer --format ndjson | mesa
package main

import (
	"flag"
	"fmt"
	"os"

	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/mesa"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("mesa", flag.ContinueOnError)
	plain := fs.Bool("plain", false, "force plain TAB-separated output")
	forceStyle := fs.Bool("force-style", false, "force styled ANSI output even when stdout is not a terminal")
	width := fs.Int("width", 0, "target terminal width for styled output (0 = auto)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	var opts []mesa.RenderOpt
	if *plain {
		opts = append(opts, mesa.ForcePlain())
	}
	if *forceStyle {
		opts = append(opts, mesa.ForceStyle())
	}
	if *width > 0 {
		opts = append(opts, mesa.Width(*width))
	}

	if err := mesa.RenderStream(os.Stdin, os.Stdout, opts...); err != nil {
		fmt.Fprintf(os.Stderr, "mesa: %v\n", err)
		return 1
	}
	return 0
}
