lux-config(5)

# NAME

lux-config - configuration files for the lux LSP multiplexer

# DESCRIPTION

Lux reads its configuration from TOML files at both global and project
levels. Global configuration lives in _~/.config/lux/_ and project-level
configuration lives in _.lux/_ at the project root.

# FILES

_~/.config/lux/lsps.toml_
	Global LSP server configuration.

_~/.config/lux/formatters.toml_
	Global formatter configuration.

_.lux/lsps.toml_
	Project-level LSP server configuration. Merged with and overrides
	global configuration.

_.lux/formatters.toml_
	Project-level formatter configuration. Merged with and overrides
	global configuration.

# LSP CONFIGURATION

The LSP configuration file (_lsps.toml_) defines language servers that lux
will manage and multiplex.

## Top-level fields

*socket* = _path_
	Unix domain socket path for the lux control server. Project-level
	socket overrides global.

## Per-LSP fields

Each language server is defined in a *[[lsp]]* array entry.

*name* = _string_ (required)
	Unique name for this language server.

*flake* = _string_ (required)
	Nix flake reference providing the language server binary.

*binary* = _string_
	Custom binary name or path within the flake output. Useful when the
	flake provides multiple binaries.

*extensions* = [_string_, ...]
	File extensions this server handles (e.g., ["go", "mod"]).

*patterns* = [_string_, ...]
	Glob patterns for matching files (e.g., ["Makefile"]).

*language_ids* = [_string_, ...]
	LSP language identifiers this server supports (e.g., ["go", "gomod"]).

At least one of *extensions*, *patterns*, or *language_ids* is required.

*args* = [_string_, ...]
	Additional arguments passed to the language server binary.

*env* = {_key_ = _value_, ...}
	Environment variables set when launching the language server. Keys must
	match [a-zA-Z\_][a-zA-Z0-9\_]\\.

*init_options* = {_key_ = _value_, ...}
	LSP initialization options sent during the initialize request. Must be
	valid JSON-serializable values.

*capabilities.disable* = [_string_, ...]
	LSP capabilities to disable for this server.

*capabilities.enable* = [_string_, ...]
	LSP capabilities to force-enable for this server.

## Example

```
socket = "/tmp/lux.sock"

[[lsp]]
name = "gopls"
flake = "nixpkgs#gopls"
extensions = ["go", "mod", "sum"]
language_ids = ["go", "gomod", "gosum"]

[[lsp]]
name = "lua-language-server"
flake = "nixpkgs#lua-language-server"
extensions = ["lua"]
language_ids = ["lua"]
args = ["--logpath=/tmp/lua-ls"]
env = { LUA_PATH = "/usr/share/lua/5.4/?.lua" }
```

# FORMATTER CONFIGURATION

The formatter configuration file (_formatters.toml_) defines external
programs used for formatting files.

## Per-formatter fields

Each formatter is defined in a *[[formatter]]* array entry.

*name* = _string_ (required)
	Unique name for this formatter.

*flake* = _string_
	Nix flake reference providing the formatter binary. Mutually exclusive
	with *path*.

*path* = _string_
	Direct path to the formatter binary. Mutually exclusive with *flake*.

*binary* = _string_
	Custom binary name within the flake output.

*extensions* = [_string_, ...]
	File extensions this formatter handles.

*patterns* = [_string_, ...]
	Glob patterns for matching files.

At least one of *extensions* or *patterns* is required.

*args* = [_string_, ...]
	Arguments passed to the formatter.

*env* = {_key_ = _value_, ...}
	Environment variables set when launching the formatter.

*mode* = _"stdin"_ | _"filepath"_
	How the formatter receives input. In *stdin* mode (the default), file
	content is piped to stdin and formatted output is read from stdout. In
	*filepath* mode, the file path is passed as an argument and the
	formatter modifies the file in-place.

*disabled* = _true_ | _false_
	When set to *true* in a project-level config, disables a formatter
	inherited from global config.

## Example

```
[[formatter]]
name = "gofmt"
flake = "nixpkgs#go"
binary = "gofmt"
extensions = ["go"]
mode = "stdin"

[[formatter]]
name = "shfmt"
path = "/usr/bin/shfmt"
extensions = ["sh", "bash"]
patterns = [".envrc"]
args = ["-i", "2", "-ci"]
mode = "stdin"
```

# MERGING BEHAVIOR

Project-level configurations are merged with global configurations.

For _lsps.toml_:
- LSPs with matching names are deeply merged, with project values
  overriding global values.
- Map fields (_env_, _init\_options_) are merged key-by-key.
- LSPs defined only in the project config are added.
- The project _socket_ value overrides the global socket.

For _formatters.toml_:
- Formatters with matching names are overridden entirely by the
  project-level definition.
- Formatters defined only in the project config are added.
- A project-level formatter with *disabled* = _true_ removes the
  corresponding global formatter.

# PROJECT ROOT DETECTION

Lux locates the project root by walking up from the current working
directory and looking for any of the following markers:

- _.lux/_
- _.git/_
- _go.mod_
- _package.json_
- _Cargo.toml_
- _pyproject.toml_

# SEE ALSO

*lux*(1)
