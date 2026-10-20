go-mcp-docs(1)

# NAME

go-mcp-docs - install the go-mcp manual pages into a directory tree

# SYNOPSIS

*go-mcp-docs* _output-dir_

# DESCRIPTION

*go-mcp-docs* writes the section 7 manual pages of the go-mcp library, which are embedded in the binary, to _output-dir_/share/man/man7/. It is a build-time tool: the purse-first flake runs it to produce the go-mcp(7) page tree that is joined into the *purse-first* package, and it is not itself part of any installed package. Start with *go-mcp*(7) for the pages it installs.

The embedded pages are hand-written roff. Each carries the placeholder _@VERSION@_ in its _.TH_ header (as _go-mcp @VERSION@_); *go-mcp-docs* replaces every occurrence with the version stamped into the binary and refuses to install a page that does not contain the placeholder. The Nix build stamps the version from the repository's _version.env_. A binary built without that stamp substitutes the string _dev_.

The output directory and the _share/man/man7_ directory below it are created if missing. Existing files with the same names are overwritten.

# OPERANDS

_output-dir_
	The root under which _share/man/man7/_ is created. Exactly one operand is required; any other argument count prints a usage message and exits non-zero.

# OUTPUT

On success *go-mcp-docs* prints a line of the form _installed N manpages in output-dir_ to standard error.

# EXIT STATUS

0
	All pages were installed.

non-zero
	The operand count was wrong, the embedded pages could not be read, a page lacked the version placeholder, or a file or directory could not be written.

# EXAMPLES

Install the pages into a scratch directory and read one:

```
go-mcp-docs ./out
man -l ./out/share/man/man7/go-mcp.7
```

# SEE ALSO

*go-mcp*(7), *go-mcp-jsonrpc*(7), *purse-first-nix*(7), *purse-first*(1)
