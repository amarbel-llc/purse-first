// Command dewey is the dewey library's command-line surface.
//
// Subcommands:
//
//	dewey table   render a List-Table NDJSON stream from stdin (RFC 0003)
//
// `dewey table` reads the NDJSON stream defined by RFC 0003 on stdin and
// renders it: styled (colored, bordered) when stdout is a terminal, plain
// TAB-separated text otherwise. It is the out-of-process front door that
// non-Go producers pipe into.
package main

import (
	"flag"
	"fmt"
	"os"

	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/mesa"
)

const usage = `usage: dewey <subcommand> [flags]

subcommands:
  table   render a List-Table NDJSON stream from stdin (RFC 0003)`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	switch args[0] {
	case "table":
		return runTable(args[1:])
	case "help", "-h", "--help":
		fmt.Fprintln(os.Stdout, usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "dewey: unknown subcommand %q\n\n%s\n", args[0], usage)
		return 2
	}
}

func runTable(args []string) int {
	fs := flag.NewFlagSet("dewey table", flag.ContinueOnError)
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
		fmt.Fprintf(os.Stderr, "dewey table: %v\n", err)
		return 1
	}
	return 0
}
