purse-first(1)

# NAME

purse-first - generate and validate per-package Claude Code plugin manifests

# SYNOPSIS

*purse-first generate-plugin* [*--root* _dir_] [*--output* _dir_] [*--skills-dir* _dir_]

*purse-first validate* [*--type* _plugin|mapping|mcp_] [*--strict*] [_path_]

*purse-first validate-mcp* _binary_ [_args..._]

# DESCRIPTION

*purse-first* is the producer toolkit for purse-first packages: it writes a package's _plugin.json_ from a _package.toml_, and validates the documents a package ships. It does not aggregate packages into a marketplace; combining the per-package outputs under _share/purse-first/<name>/_ is left to external tools.

A purse-first package is laid out as _share/purse-first/<name>/_ and described by a _plugin.json_; see _docs/purse-first-protocol.md_ in the purse-first repository for the protocol.

All diagnostics and success messages are written to standard error. Standard output is not used by any subcommand.

# SUBCOMMANDS

## generate-plugin

Read _package.toml_ from the package root and write a _plugin.json_.

*--root* _dir_
	Package root containing _package.toml_. Defaults to the current directory.

*--output* _dir_
	Output directory. Defaults to the package root.

*--skills-dir* _dir_
	Directory of skills to discover and copy. Every _\*/SKILL.md_ directly beneath it is listed in the manifest as _./skills/<name>_ (sorted), and the whole directory is copied to _skills/_ beside the manifest when at least one skill is found.

The manifest is written to _<output>/share/purse-first/<name>/.claude-plugin/plugin.json_, where _<name>_ is the _name_ in _package.toml_. Missing directories are created.

_package.toml_ is read for these keys:

*name*, *description*, *version*
	Package identity.

*author.name*
	Author; written to the manifest only when non-empty.

*mcp.<server>*: *command*, *args*
	Each entry becomes an _mcpServers_ entry of type _stdio_.

*hooks.<event>*: *matcher*, *command*, *timeout*
	Each array entry becomes a _command_ hook under that event.

## validate

Validate a plugin manifest or a mapping document.

_path_ may be a file, a directory, or *-* for standard input; it defaults to the current directory.

*--type* _plugin|mapping|mcp_
	Force the document type instead of detecting it. An unknown value is an error.

*--strict*
	Treat warnings as errors: the command fails if any warning is reported.

For a file, the type is taken from *--type* if given, otherwise from the base name (_plugin.json_ is a plugin, _mappings.json_ is a mapping), otherwise from the content: an object with both _mappings_ and _server_ is a mapping, and an object with _name_ is a plugin. If none applies, validation fails.

For a directory, *validate* checks _.claude-plugin/plugin.json_ and _mappings.json_ in it, whichever exist, and fails if neither does.

Standard input requires *--type*.

With *--type mcp*, _path_ is required and names a binary, which is handled as in *validate-mcp* (without extra arguments).

Plugin checks include: _name_ is required and must match _^[a-z][a-z0-9-]\*$_; _author.name_ is required when _author_ is present; each _mcpServers_ entry must have type _stdio_ and a _command_ (a command containing spaces is a warning); a manifest declaring no MCP servers or skills is a warning.

Mapping checks include: _server_ is required and must match the same pattern as a plugin name; at least one mapping is required; each mapping needs _replaces_, _reason_, at least one tool with _name_ and _use_when_, and extensions or command prefixes for scoping (without them it is a warning, or an error under *--strict*).

## validate-mcp

Spawn _binary_ with any further arguments as an MCP server on standard input and output, and check its protocol responses. The probe is cut off after 10 seconds. It checks:

- the _initialize_ handshake (protocol V1 is requested; a V0 server negotiates down);
- _tools/list_: it must return at least one tool; a tool without annotations is a warning;
- _resources/list_ and _resources/templates/list_: the responses must decode; a server that reports the method as unsupported is accepted.

# EXIT STATUS

*0*
	Validation passed, or the manifest was generated.

*1*
	An error occurred: a validation error (or a warning under *--strict*), an unreadable input, an unknown *--type*, or a failure during generation.

# EXAMPLES

Generate the manifest for the package in the current directory, with skills:

```
purse-first generate-plugin --skills-dir skills
```

Validate a generated package directory strictly:

```
purse-first validate --strict result/share/purse-first/mypkg
```

Probe an MCP server binary:

```
purse-first validate-mcp ./bin/mypkg-mcp
```

# SEE ALSO

*mesa*(1), *dagnabit*(1), *go-mcp*(7)

The protocol is specified in _docs/purse-first-protocol.md_ in the purse-first repository.
