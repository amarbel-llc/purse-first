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
	"errors"
	"flag"
	"fmt"
	"os"

	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/mesa"
)

const synopsis = `mesa - render a List-Table NDJSON stream (RFC 0003)

usage:
  producer --format ndjson | mesa [flags]

mesa reads an NDJSON list-table stream on stdin and renders it: a styled,
bordered, colored table when stdout is a terminal, plain TAB-separated text
otherwise. Styling is carried semantically in the stream, so producers emit
data, never terminal escapes.

flags:
`

const seeAlso = `
exit status:
  0  rendered (an empty table is not an error)
  1  a protocol error in the stream
  2  a usage error (e.g. an unknown flag)

See mesa(1) and RFC 0003 for the stream format.
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("mesa", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, synopsis)
		fs.PrintDefaults()
		fmt.Fprint(os.Stderr, seeAlso)
	}
	plain := fs.Bool("plain", false, "force plain TAB-separated output")
	forceStyle := fs.Bool("force-style", false, "force styled ANSI output even when stdout is not a terminal")
	width := fs.Int("width", 0, "target width for styled output (0 = auto: terminal width, or content on a pipe)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0 // -h / --help is not an error
		}
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
