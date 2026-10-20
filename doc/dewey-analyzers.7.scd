dewey-analyzers(7)

# NAME

dewey-analyzers - the six dewey static analyzers: what each flags and how to fix or suppress it

# DESCRIPTION

*dewey*(7) ships six analyzers built on _golang.org/x/tools/go/analysis_. Each lives in a package _libs/dewey/pkgs/analyzer\_<name>_ that exports a single _Analyzer_ value, and each is available two ways:

- as a module plugin linked into the *golangci-lint-dewey*(1) binary, which is how the repository itself runs them (the _lint-dewey-self_ recipe); and
- as a standalone checker for *go vet*: the flake packages _actx_, _defererr_, _paramobj_, _repool_, _seqerror_ and _testui_ each build a binary around one analyzer (see *purse-first-nix*(7)), used as *go vet -vettool=*_path_ _./..._.

Four analyzers are on by default in the plugin: *defererr*, *seqerror*, *repool* and *testui*. Two are opt-in and are off unless set to _true_: *actx* and *paramobj*. They are opt-in because they encode project conventions rather than general Go correctness.

# ENABLING THROUGH GOLANGCI-LINT-DEWEY

The plugin is registered under the name _dewey_. Enable it in _.golangci.yml_ and, optionally, toggle each analyzer by name:

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
          testui: true
          actx: true
          paramobj: true
```

Omitting a key leaves the analyzer at its default (on for the first four, off for *actx* and *paramobj*); an explicit _false_ turns a default-on analyzer off. The plugin's settings structure has exactly those six fields. See *golangci-lint-dewey*(1) for invoking the binary.

# SUPPRESSION

Every analyzer honours a line comment directive. The directive must be on the same line as the flagged construct or on the line immediately above it. The directives are:

[[ *Analyzer*
:< *Directive*
|  defererr
:  //defer:err-checked
|  seqerror
:  //seq:err-checked
|  repool
:  //repool:owned, //repool:suppress
|  testui
:  //testui:allow
|  actx
:  //actx:allow
|  paramobj
:  //paramobj:allow

# defererr

*Flags.* A *defer* statement whose call returns an error (alone or as one element of a tuple), so the error is silently dropped. Deferred function literals are skipped, since the error can be handled inside the closure. The diagnostic differs for calls named _Close_ and for other calls.

*Fix.* Wrap the call so the error is captured, with a named return. For a read-only file descriptor, the diagnostic suggests _files.CloseReadOnly_ (package _pkgs/files_); otherwise _errors.DeferredCloser(&err, x)_ or another _errors.Deferred\*_ helper (package _pkgs/errors_). Alternatively write the deferred call as a closure that checks or assigns the error. Suppress with _//defer:err-checked_ where dropping the error is deliberate.

# seqerror

*Flags.* A *range* over a function of the iterator shape _func(yield func(T, error) bool)_ (that is, an _iter.Seq2[T, error]_) where the error value is discarded: ranged with no second variable, ranged into _\__, never checked, or checked in an *if* whose body does not return, break, continue, panic or pass the error to a call.

*Fix.* Bind the error and handle it in the loop body, for example _if err != nil { return err }_, or pass it on to another call. Suppress with _//seq:err-checked_.

# repool

*Flags.* The _interfaces.FuncRepool_ returned by a pool's _GetWithRepool()_ is discarded with the blank identifier, or is not used on every path out of the function (reported as a possible pool leak). A function that references the variable from a *defer* is treated as covered on all paths. A path that ends in *panic* is not reported. The analyzer does nothing in a package that does not import a package whose path ends in _interfaces_.

*Fix.* Call the repool function when finished with the pooled object, typically with *defer*. Where ownership is passed on deliberately, suppress a discarded result with _//repool:owned_, or a path-analysis report with _//repool:suppress_.

# testui

*Flags.* A use of the standard library _testing.T_ or _\*testing.T_ as a function parameter, function result, interface method signature parameter or result, or struct field, where dewey's _test\_ui.T_ (package _pkgs/test\_ui_) is intended. The first parameter of a free function whose name is a Go test entry point (_Test_ followed by a non-lowercase rune or nothing) is exempt, because the testing runtime fixes that signature; methods and ordinary helpers are not exempt.

*Fix.* Take _test\_ui.T_ in helpers, fixtures and test-support interfaces, constructing it from the runtime-provided value. At seams that genuinely need the standard type (third-party helpers, closures handed a _\*testing.T_ by the runtime), suppress with _//testui:allow_.

# actx

*Flags.* A function parameter, function result or struct field typed exactly as the standard library _context.Context_, where dewey's _interfaces.ActiveContext_ is intended. Opt-in.

*Fix.* Use _interfaces.ActiveContext_ (package _pkgs/interfaces_). At interop boundaries that require the standard type, such as _exec.CommandContext_ or _net/http_, suppress with _//actx:allow_.

# paramobj

*Flags.* Groups of function and method declarations in one package that share the same ordered parameter type list, once a group is large enough. The receiver, parameter names and result types are ignored. A declaration is considered only if it has at least _min-params_ parameters (default 3), and a group is reported only if it has at least _min-group_ members (default 3). Each member of a reporting group is flagged. It is advisory; there is no autofix. Opt-in.

*Fix.* Extract the shared parameters into a struct. Suppress a single declaration with _//paramobj:allow_ on its signature line or the line above.

The thresholds are defined as analyzer flags named _min-group_ and _min-params_. The plugin's settings have no field for them, so under *golangci-lint-dewey*(1) the defaults apply.

# SEE ALSO

*dewey*(7), *purse-first-nix*(7), *golangci-lint-dewey*(1)
