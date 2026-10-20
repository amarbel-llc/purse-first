dewey-cli(7)

# NAME

dewey-cli - dewey's scaffolding for building command-line applications

# DESCRIPTION

*dewey*(7) publishes a group of packages that other repositories use to build command-line programs: a command registry that can also emit manual pages, completions and plugin manifests; a flag-set fork; shared global flags; output helpers; terminal printers; and a progress viewport. All are imported from _code.linenisgreat.com/purse-first/libs/dewey/pkgs/<package>_. This page describes what each package provides, as read from the facade sources in _libs/dewey/pkgs_ and the packages behind them.

Of these, only *command* is classified by the maintainers: it is listed as experimental in _libs/dewey/CLAUDE.md_, which says to expect rough edges and missing features. The others have no recorded classification there.

# command

A _Utility_ is the registry and top-level metadata of one application. _NewUtility(name, short)_ creates one and registers two built-in commands, _dev-mcp_ and _\_\_complete_. Commands are added either as a declarative _Command_ value with _AddCommand_, or as a type implementing _Cmd_ (a _Run(Request)_ method, with optional interfaces for a description, parameters, a structured result and MCP annotations) with _AddCmd_.

Parameters are declared with the generic _Flag[V]_ and _Arg[V]_ types and with _ArrayFlag_ and _ObjectFlag_; _OldParam_ and _OldArg_ are deprecated. From a single definition the utility can produce, among others:

- _GenerateAll(dir)_, which writes the plugin manifest, tool mappings and hooks under _share/purse-first/<name>/_, manual pages under _share/man/man1/_ and bash, zsh and fish completions;
- _InstallExtraManpages(dir)_, which copies hand-written pages listed in _Utility.ExtraManpages_ to _share/man/man<section>/<name>_ without modifying them;
- _RegisterMCPTools_, _RunCLI_ and _HandleHook_ for serving the same commands as MCP tools, from the command line, and as a Claude Code tool-interception hook.

_Utility.Version_ is a plain string field set by the caller.

# flags and flag\_policy

*flags* is a variant of the Go standard library's _flag_ package (its documentation is carried over) with additions: _FlagWithPolicy_ and _MakeWithPolicy_, and _SplitCommasAndTrim_. *flag\_policy* defines the policy type _FlagPolicy_ with two values, _FlagPolicyAppend_ and _FlagPolicyReset_. A _FlagWithPolicy_ built with _FlagPolicyReset_ calls its reset function before applying each new value.

# cli

*cli* aliases _FlagSet_ from *flags* and defines the _CLICompleter_ interface, whose _GetCLICompletion()_ returns a map of completion candidates, and _FlagSetVarWithCompletion_, which registers a flag whose usage text is the comma-separated list of those candidates' keys.

# config\_cli

*config\_cli* defines _Config_, the common global options of a dewey-based program: _Debug_, _Verbose_, _Quiet_, _Todo_ and a dry-run setting. Its _SetFlagDefinitions_ registers the flags _debug_, _todo_, _dry-run_, _verbose_ and _quiet_. _CustomOut_ and _CustomErr_ override standard output and error when set, which the source says is used by MCP handlers to capture command output.

# output

*output* limits the size of tool output. _LimitText_ applies _TextLimits_ in the order head or tail, then line count, then byte count; _LimitArray_ paginates a slice with _ArrayLimits_; _LimitStderr_ applies default byte truncation to captured standard error; _StandardDefaults_ returns the default limits. Zero limits mean unlimited.

# ui

*ui* provides terminal printers for a program: the package-level _Out_, _Err_, _Log_ and _Debug_ printers, _MakePrinter_ and related constructors, _SetVerbose_ and _IsVerbose_, a prefixing printer, prioritized _Todo_ printers (_TodoP0_ to _TodoP5_), _HumanReadableBytes_ for sizes given on a command line, and _CLIErrorTreeEncoder_ for rendering nested errors as a tree. It also holds test helpers (_T_, _MakeT_, _TestCase_) built on *test\_ui*.

# operation\_viewport

*operation\_viewport* runs a subprocess, or a batch of operations, under a terminal viewport made of a spinner and a rolling tail of output, built on bubbletea. _Run_ runs one command; _RunBatch_ drives a work loop with an _Emitter_ and a progress bar; _Model_ is the underlying bubbletea model for custom event sources. On a non-terminal standard output both fall back to plain text on standard error. On Ctrl-C, _Run_ kills the child and prints the captured transcript to standard error, and _RunBatch_ cancels the context it hands to the work loop. The design record is FDR 0010, whose status is _proposed_.

# SEE ALSO

*dewey*(7), *dewey-analyzers*(7), *go-mcp*(7), *go-mcp-docs*(1)
