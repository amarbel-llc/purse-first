mesa(1)

# NAME

mesa - render a List-Table NDJSON stream

# SYNOPSIS

*mesa* [_--plain_] [_--force-style_] [_--width_ _N_]

# DESCRIPTION

*mesa* reads a List-Table NDJSON stream (RFC 0003) on standard input and
renders it as a table on standard output. On a terminal it draws a styled,
bordered, colored table; on a pipe it writes plain TAB-separated text.

Styling is carried semantically in the stream -- a fixed severity vocabulary
the renderer colors -- so producers emit data, never terminal escapes. Any
language can feed *mesa* the same way.

# OPTIONS

*--plain*
	Force plain TAB-separated output regardless of whether standard output
	is a terminal.

*--force-style*
	Force styled ANSI output even when standard output is not a terminal,
	e.g. when piping into a file or a pager.

*--width* _N_
	Target total width, in columns, for styled output. _0_ (the default)
	sizes to content on a pipe, or to the terminal width when standard
	output is a terminal. Flexible columns shrink in priority order and
	ellipsize to fit.

# INPUT

Standard input is a newline-delimited JSON stream: one header record that
describes the columns (and an optional legend, empty-state text, and palette
override), then one record per row. A row cell is a bare JSON string (a plain
neutral value) or an object carrying styled spans. See *RFC 0003* for the full
schema and the rendering rules.

# EXIT STATUS

*0*
	The stream was rendered. An empty table (a header with no rows) is not
	an error.

*1*
	A protocol error in the stream: a missing or malformed header, an
	invalid column role, or a row whose cell count does not match the
	columns.

*2*
	A usage error, such as an unknown flag.

# EXAMPLES

Render a producer's stream on the terminal:

```
some-tool list --format ndjson | mesa
```

Force color into a pager:

```
some-tool list --format ndjson | mesa --force-style | less -R
```

# SEE ALSO

The normative stream format is specified in RFC 0003 (the List-Table NDJSON
Protocol), in _docs/rfcs/_ of the purse-first repository. The renderer library
is the Go package _mesa_ at
_code.linenisgreat.com/purse-first/libs/dewey/pkgs/mesa_.
