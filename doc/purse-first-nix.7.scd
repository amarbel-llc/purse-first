purse-first-nix(7)

# NAME

purse-first-nix - the purse-first flake's outputs and its reusable conformist modules

# DESCRIPTION

The purse-first flake builds the repository's tools and publishes two kinds of reusable conformist module for other repositories that use the dewey layout (see *dewey*(7), *dagnabit*(7)). This page lists the flake outputs and documents the exported modules. Everything here was read from _flake.nix_, _gomod.nix_ and _nix/_ in the purse-first repository.

The flake supports the systems _x86_64-linux_, _aarch64-linux_, _aarch64-darwin_ and _x86_64-darwin_.

# PACKAGES

These are the _packages.<system>_ outputs.

*default*, *purse-first*
	The *purse-first*(1) CLI joined with the manual pages the repository ships: *purse-first*(1), the go-mcp(7) tree, *dagnabit*(1), *dagnabit*(7), *mesa*(1), *dewey*(7), *dewey-analyzers*(7), *dewey-cli*(7), *golangci-lint-dewey*(1), *go-mcp-docs*(1) and *purse-first-nix*(7). Only the compiled pages of dagnabit and mesa are joined in, not their binaries.

*dagnabit*
	The *dagnabit*(1) tool with its pages. A Nix build stamps the version and commit into the binary.

*mesa*
	The *mesa*(1) renderer with its page.

*actx*, *defererr*, *paramobj*, *repool*, *seqerror*, *testui*
	One binary per dewey analyzer, for use as a *go vet* tool. See *dewey-analyzers*(7).

*golangci-lint-dewey*
	golangci-lint with the dewey plugin linked in. See *golangci-lint-dewey*(1).

*reflexive-interface-generator*
	A dewey code-generation tool.

*manpages*
	The go-mcp(7) manual page tree on its own.

*go-pkgs*, *go-pkgs-test*
	The published Go workspace source (the RFC 0001 outputs) and its test-only variant.

*conformist-config*
	The generated conformist configuration for this repository.

*conformist-facade-config*
	The configuration built from *lib.conformistModules.dagnabit-facade* (see below).

*conformist-impure-config*, *conformist-impure*
	The configuration for the repository's whole-tree checks that need the live working tree, and a wrapper that runs conformist with it.

*conformist-pre-commit*, *conformist-repair*
	The per-commit and merge-time conformist repair hooks.

Other outputs are _apps.<system>.default_ (the CLI), _devShells.<system>.{default,go,shell,bats,rust}_, _formatter.<system>_ and _checks.<system>.formatting_.

# lib.conformistLinters

*lib.conformistLinters* is a system-independent attribute set of conformist linter modules, each a path. A consuming repository imports one into its own *conformist.lib.evalModule* call and enables it under _linters.<name>_:

```
conformist.lib.evalModule pkgs {
  imports = [ purse-first.lib.conformistLinters.dewey-facade-export ];
  package = conformist.packages.${system}.default;
  linters.dewey-facade-export = {
    enable = true;
    deweyDir = "go";
    library = false;
    dagnabitPackage = purse-first.packages.${system}.dagnabit;
    conformistConfig = facadeConfig;
  };
}
```

All three are whole-tree linters: each runs once, takes no file arguments, and is triggered when a file matching its _includes_ glob is in scope. Each does nothing if the expected directory or file is absent. Each runs *dagnabit*(1), which invokes the Go toolchain, so they need the live working tree and the module cache and cannot run in a sandboxed check; purse-first runs them in its impure lane (_lint-worktree_).

Options common to all three:

_enable_
	Enable the linter.

_deweyDir_
	String. Path, relative to the tree root conformist runs from, of the dewey-layout module root. Default _libs/dewey_.

_dagnabitPackage_
	Null or a package. When null the scripts run whichever *dagnabit* is on the caller's PATH, and fail with a message if there is none. When set, that package's binary is used, making the run independent of PATH. Default null.

## dewey-facade-export

Checks that the generated _pkgs/_ facades match _internal/_. The check runs *dagnabit export --check*; the repair command runs *dagnabit export*. Both run from _deweyDir_ and add *--library* when the _library_ option is true. Triggered by _<deweyDir>/\*\*/\*.go_.

_library_
	Boolean, default true. Pass *--library* to *dagnabit export*, which exports every package under _internal/_ and fails if a _//go:generate_ export directive exists. Set false for a repository that uses directives.

_conformistConfig_
	Path, required. Store path of the formatters-only configuration that dagnabit's format pass uses, passed through _DAGNABIT_CONFORMIST_CONFIG_. Build it from *lib.conformistModules.dagnabit-facade*. See *dagnabit*(1) for the contract dagnabit enforces on it.

## dewey-init-smoke

Checks that the generated per-architecture tests in _initsmoke/_ match the package graph. The check runs *dagnabit init-smoke --check*; the repair command runs *dagnabit init-smoke*. Does nothing if _<deweyDir>/dagnabit.toml_ is absent. Triggered by _<deweyDir>/\*\*/\*.go_ and _<deweyDir>/dagnabit.toml_. Takes _conformistConfig_ with the same meaning as above.

## dewey-reposition

Checks that every package sits at the NATO level its dependencies imply. The check runs *dagnabit -n internal* in _deweyDir_ and fails if it prints anything; the repair command runs *dagnabit internal*, which moves the packages. Triggered by _<deweyDir>/\*\*/\*.go_.

# lib.conformistModules

*lib.conformistModules.dagnabit-facade* is a conformist configuration module, not a linter. It enables the *goimports* formatter at priority 1 and the *gofumpt* formatter at priority 2 and declares nothing else. Evaluate it with the consumer's own conformist and nixpkgs pins to obtain the configuration file that dagnabit uses to format the files it generates:

```
facadeEval = conformist.lib.evalModule pkgs {
  imports = [ purse-first.lib.conformistModules.dagnabit-facade ];
  package = conformist.packages.${system}.default;
};
# facadeEval.config.build.configFile
```

Generated files belong to the generator: the consumer's own conformist configuration should exclude them (_<deweyDir>/pkgs/\*\*_ and the generated _initsmoke_ tests), and dagnabit formats them with this configuration instead. dagnabit refuses a configuration that declares linters, sets a tree root or *skip-generated*, gives a formatter a working directory, or has an exclude matching a generated file; see *dagnabit*(1), *ENVIRONMENT*.

# NOT EXPORTED

_lib/mkGoWorkspaceModule.nix_ is the helper _gomod.nix_ uses to build the repository's Go binaries from the workspace source. It is an internal file; the flake does not export it under _lib_ or anywhere else.

# SEE ALSO

*dagnabit*(7), *dewey*(7), *dewey-analyzers*(7), *conformist*(1), *dagnabit*(1), *golangci-lint-dewey*(1), *mesa*(1), *purse-first*(1)
