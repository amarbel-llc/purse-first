golangci-lint-dewey(1)

# NAME

golangci-lint-dewey - golangci-lint with the dewey static analyzers linked in

# SYNOPSIS

*golangci-lint-dewey run* [_golangci-lint run flags_] [_packages..._]

*golangci-lint-dewey linters*

*golangci-lint-dewey version*

# DESCRIPTION

*golangci-lint-dewey* is stock golangci-lint with the dewey module plugin (_libs/dewey/gclplugin_) compiled in. It is the pure-Nix equivalent of building a custom binary with *golangci-lint custom*, which needs network access and module resolution at build time. The plugin and the binary are compiled together against the single golangci-lint version pinned in _cmd/golangci-lint-dewey/go.mod_, so the plugin/binary version match that module plugins require holds by construction.

The program's _main_ blank-imports the plugin and then hands control to golangci-lint's own command entry point. Its command line is therefore golangci-lint's; this page documents only what is specific to the dewey build. For the rest, see the documentation of the golangci-lint version pinned in the module.

The plugin contributes one linter, named _dewey_, that runs up to six analyzers. Which analyzers exist, what each flags and how to suppress findings is described in *dewey-analyzers*(7).

# COMMANDS

*run*
	Lint the given packages with the linters enabled by the configuration. A finding from the plugin exits non-zero and is reported with the linter name _dewey_ in parentheses.

*linters*
	List the available linters. In a directory whose configuration enables it, _dewey_ appears in the list.

*version*
	Print the build version. A Nix build reports a version of the form _<golangci-lint version>-dewey.<purse-first version>_; a binary built without the Nix ldflags (the _build-go-gcl_ dev recipe) reports _unknown_.

# CONFIGURATION

The configuration is golangci-lint's own _.golangci.yml_ (configuration format _version: "2"_). Enable the plugin by name and, optionally, toggle each analyzer under _settings.custom.dewey.settings_:

```
version: "2"
linters:
  default: none
  enable:
    - dewey
  settings:
    custom:
      dewey:
        type: module
        description: dewey static analyzers
        settings:
          defererr: true
          seqerror: true
          repool: true
```

*defererr*, *seqerror*, *repool* and *testui* are on unless set to _false_. *actx* and *paramobj* are off unless set to _true_. The plugin offers no other settings. See *dewey-analyzers*(7).

# ENVIRONMENT

_GOWORK_
	The linter loads packages with the Go toolchain. When the module being linted lies under a _go.work_ file that does not list it, set *GOWORK=off* so the module is loaded in module mode. The repository's own recipes and acceptance tests do this.

# FILES

_.golangci.yml_
	The configuration described above, found by golangci-lint's usual lookup.

# EXAMPLES

Build the binary from the purse-first flake and lint the dewey module with it, as the repository does:

```
nix build .#golangci-lint-dewey -o result-gcl
cd libs/dewey && GOWORK=off ../../result-gcl/bin/golangci-lint-dewey run ./...
```

# SEE ALSO

*dewey-analyzers*(7), *dewey*(7), *purse-first-nix*(7)
