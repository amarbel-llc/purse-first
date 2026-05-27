---
status: exploring
date: 2026-05-06
decision-makers: Sasha F
---

> **Provenance.** Originally captured at
> `amarbel-llc/madder/docs/decisions/0007-failure-propagation-via-typed-panics.md`
> (commit `d499f22`, 2026-05-06). Relocated to dewey on 2026-05-27 as
> the canonical home for this pattern. The original framing was
> scoped to madder's `BlobStore`; this version is broadened to
> dewey's general panic-as-channel mechanism, with madder's
> `BlobStore` retained as the motivating case study.
>
> The parallel ADR in dodder (open issue
> [amarbel-llc/dodder#27](https://github.com/amarbel-llc/dodder/issues/27))
> and the concrete cleanup pass in
> [amarbel-llc/madder#20](https://github.com/amarbel-llc/madder/issues/20)
> are now downstream references to this one. The story is
> consolidated here so future contributors do not have to
> re-derive it across three repos.

# Typed panics as a failure-propagation channel

## Context and Problem Statement

dewey provides `errors.Context` plus a structured-throw machinery
(`Cancel`, `runRetry`, `ContextContinueOrPanic`) that lets call
sites abort via panic + recover at known boundaries rather than via
error-return all the way up. Consumers (madder, dodder, the
purse-first command framework) increasingly use this for
control-flow failures where threading `error` through every layer
would force function coloring on middleware that has no business
interpreting failure.

This pattern was not documented as a decision; consumers
re-derived it from neighboring code. The concrete trigger was
madder's `BlobStore` interface (the worked example below): a mix
of error-returning and no-error methods, where the no-error
methods could nevertheless fail in backends that resolved state
lazily. Issue
[amarbel-llc/madder#134](https://github.com/amarbel-llc/madder/issues/134)
surfaced this when an unreachable SFTP store crashed
`madder-mcp serve` because dewey's `ctx.Cancel(err)` structured
throw escaped from outside any `Run` frame.

The same question is open in dodder: when should a subsystem
return errors and when should it cancel / panic? Twelve TODOs
across the dodder tree (catalogued in
[amarbel-llc/dodder#27](https://github.com/amarbel-llc/dodder/issues/27))
express migration intent toward panic semantics without a
documented decision behind them.

This ADR captures the answer so future contributors do not
re-derive it from scratch.

## Decision Drivers

* **Function-coloring cost of error returns.** Adding `error` to
  every method forces every layer in a call tree to inherit the
  error-handling discipline of its leaves. Middleware that just
  orchestrates becomes noise: `if err := ...; err != nil { return err }` ×N.
  Languages with effects/exceptions don't pay this cost;
  intermediate code can stay polymorphic over what its inner
  functions might raise.
* **Context-dependent failure semantics.** The same low-level event
  ("SSH dial failed") means structurally different things at
  different call sites: in a walk-the-stores loop it is "skip this
  store, try the next"; in a `fsck` run it is "this is a
  verification failure"; in a debug introspection tool it might be
  "log it and continue." If the leaf method returns `error`, the
  *interpretation* of the error has to be repeated at every call
  site. We want the leaf to *describe what happened* and the
  surrounding scope to *decide what it means*.
* **Go's idiomatic baseline.** Stdlib's convention is "method
  returns `error` if any plausible implementation could fail at
  it." We are consciously diverging.
* **Practical surface size.** dewey's consumer set is small and
  internal (madder, dodder, purse-first, future internal
  embeddings). The cost of a documented unenforced convention is
  bounded; the cost of threading `error` through every call site
  is not.
* **Existing dewey machinery.** dewey already commits to
  panic-as-control-flow: `Cancel(err)` is a structured throw
  caught by `runRetry`. Adding a parallel discipline for typed
  data-propagation panics layers cleanly on top.

## Considered Options

1. **Add `error` returns to every method that can plausibly fail.**
   Threads context through call sites; matches Go stdlib idiom.
   Touches every implementation, every caller, every test stub.
2. **Keep no-error signatures; leaves call `ctx.Cancel(err)` to
   surface failures.** dewey's pre-#134 convention. Works inside a
   `Run` frame; breaks for long-lived embeddings.
3. **Keep no-error signatures; leaves `panic` with a typed value;
   callers `recover` at known boundaries and assign meaning per
   context.** A poor-man's algebraic effect system in Go.
4. **Hybrid: `MustX` convenience wrappers around an
   error-returning interface.** Stdlib's `regexp.Compile` /
   `regexp.MustCompile` pattern. Caller chooses the ergonomic
   shape per call site.

## Decision Outcome

Chosen option: **3 (typed panics with named recover boundaries)**,
because it preserves middleware ergonomics, lets the *call-site
context* decide what a failure means rather than baking that
decision into the leaf, and aligns with established discipline
from Common Lisp condition systems and modern algebraic effects
(Koka, Eff, OCaml 5 effect handlers). We accept that Go's lack of
static effect typing means the discipline is enforced by
convention and code review rather than by the compiler.

### Consequences

* **Good, because** middleware between leaf and handler does not
  need to declare or propagate the failure channel. Defers handle
  cleanup; signatures stay clean.
* **Good, because** the *handler* — not the leaf — decides what a
  failure means in context. The same low-level event becomes a
  "skip" in one caller and a "fault" in another.
* **Good, because** stack traces are free. A panic carries the
  call stack at the throw site, which is often more useful than a
  wrapped error chain.
* **Good, because** the discipline composes with dewey's existing
  `ctx.Cancel` / `runRetry` panic mechanism. Calls inside a `Run`
  frame absorb leaf panics the same shape as `ctx.Cancel`-driven
  aborts.
* **Bad, because** Go gives no static guarantee that handlers
  exist. A panic with no surrounding `recover` crashes the
  process. Compiler does not flag this.
* **Bad, because** Go gives no type discipline on the panic
  payload. Every recover boundary does its own type switch. A
  typo in a payload type silently turns into "fall through and
  re-panic."
* **Bad, because** Go has no resumable exceptions / restarts.
  Recover can only unwind, not resume from the throw site. Some
  patterns (Common Lisp's `restart-case`) are not expressible.
* **Bad, because** the convention is unenforced and easily
  violated by an author who adds a new no-error method without
  documenting which panic types it raises.

### Confirmation

* Interface comments name the convention and the panic-payload
  contract at each site that adopts it.
* Each long-lived embedding (madder-mcp today; future library
  mode tomorrow; dodder server endpoints when they adopt) establishes
  recover boundaries at the granularity its caller-context needs.
* CI for consuming packages exercises the typed-panic paths
  end-to-end. For madder: the bats SFTP integration suite (33
  tests, real local sshd) plus the manufactured "unreachable
  SFTP" smoke test in `just debug-mcp-resources`.
* A small `Handle[T]` helper standardising the recover +
  type-switch shape so every boundary site uses the same
  scaffolding is desirable (out of scope for this ADR; tracked as
  a [madder#134](https://github.com/amarbel-llc/madder/issues/134)
  follow-up).

## Pros and Cons of the Options

### 1. Add `error` returns everywhere

* Good, because it matches Go stdlib idiom — `io.Reader.Read`,
  `fs.FS.Open`, `net.Dial` all return `error` for the same reason
  (some implementation might fail).
* Good, because failure is statically legible: every caller has
  to acknowledge it can fail, and the compiler enforces it.
* Good, because debugging is grep-able: error messages flow
  through named return paths.
* Bad, because it forces function coloring across the entire call
  tree. Every layer between leaf and consumer has to thread
  `error`, even when the layer has no business interpreting
  failure.
* Bad, because it bakes a single failure interpretation into the
  leaf signature. The same `HasBlob` returning `(bool, error)`
  can't cleanly say "this is a skip in openBlob, a fault in
  fsck"; the interpretation moves to every call site.
* Bad, because the rewrite is large. Touches every implementation,
  every caller, every test stub. Not small enough to land
  alongside other work.

### 2. `ctx.Cancel(err)` from leaves (status quo pre-madder#134)

* Good, because it composes with dewey's existing
  structured-throw machinery and its `Run` frame catch.
* Good, because leaves don't need to know about per-call ctx
  threading — they hold the env's ctx and signal against it.
* Neutral, because in a CLI context with a single Run frame
  around the whole command, this works fine and is invisible to
  most consumers.
* Bad, because it conflates "abort this run" (control flow) with
  "report this error" (data). A leaf that wants to report an
  error has no choice but to also abort everything sharing the
  context.
* Bad, because long-lived embeddings (madder-mcp serve, future
  library mode) sit outside a `Run` frame; the throw escapes and
  crashes the host. This is exactly the bug madder#134 captured.
* Bad, because the panic payload carries dewey's
  `ContextStateSucceeded` sentinel rather than the underlying
  error, due to a separate dewey TODO at `GetState` (closed-Done
  is always reported as Succeeded). Error messages mislead.

### 3. Typed panics with named recover boundaries (chosen)

* Good, because middleware stays clean. Layers between leaf and
  handler don't have to declare or propagate the failure channel.
* Good, because the *handler* assigns meaning. The same low-level
  event (SSH dial fail) becomes a "skip" in openBlob and a
  "fault" in fsck — same leaf, different handlers.
* Good, because it composes with dewey's existing panic
  conventions. CLI commands inside a Run frame absorb leaf panics
  the same way they absorb `ctx.Cancel` panics.
* Good, because it captures stack traces and lets `recover`
  decide whether to log, wrap, suppress, or re-panic.
* Neutral, because the convention is documented but not
  statically enforced. Code review and CLAUDE.md notes carry the
  weight.
* Bad, because there is no compile-time guarantee that every
  panic-raising leaf has a covering recover. A new long-lived
  embedding can ship with the same bug madder-mcp had pre-#134.
* Bad, because Go has no resumable exceptions; recovers only
  unwind. Patterns like CL's `invoke-restart 'use-cached-value`
  are not expressible.

### 4. `MustX` wrappers around an error-returning interface

* Good, because it gives callers the ergonomic shape they want
  at the call site (panic-style or error-style) while keeping
  the interface honest.
* Good, because it's the stdlib pattern (`regexp.MustCompile` /
  `regexp.Compile`, `template.Must`, `time.Date` /
  `time.MustParse`).
* Neutral, because it doubles the API surface. Each method gets
  two shapes.
* Bad, because the leaf still has to commit to *one*
  interpretation of failure (the error type). The "context
  decides meaning" win of option 3 is lost.
* Bad, because it doesn't fix function coloring — the underlying
  interface is still error-returning, so middleware that wants
  to call the error variant pays the same threading cost as
  option 1.

## More Information

### Worked example: madder's `BlobStore`

`BlobStore` (in
[amarbel-llc/madder](https://github.com/amarbel-llc/madder/blob/master/go/internal/0/domain_interfaces/blob_store.go))
exposes a mix of methods: some carry an explicit error return
(`MakeBlobReader`, `MakeBlobWriter`, `AllBlobs`), some do not
(`HasBlob`, `GetBlobIOWrapper`, `GetDefaultHashType`,
`GetBlobStoreConfig`, `GetBlobStoreDescription`). The remote SFTP
backend can fail at no-error methods because its state is
fetched lazily from a remote host.

The fix that landed (commits `9b74186` + `b3e0ae9` in
amarbel-llc/madder):

- SFTP panics directly with the underlying error.
- `madder-mcp` adds two recover boundaries: `tryOpenInStore`
  (per-store) and `ReadResource` (per-request).

This pattern — leaf panics with typed value; embedding's outer
scopes recover at meaningful granularity — is the chosen
option 3 in action. madder's `BlobStore` is the motivating case
but the same discipline applies anywhere a dewey-using consumer
wants context-dependent failure interpretation.

### The lineage

The pattern chosen here is not novel. It traces back to **Common
Lisp's condition system** (1980s, formalised in CLtL2). A
function `signal`s a condition (typed value); the *outer dynamic
scope* establishes handlers via `handler-case` / `handler-bind`.
The handler decides what the condition means in this context.
Crucially, handlers can offer **restarts** — they can resume the
signaler at known recovery points, not just unwind. The signaler
describes what happened; the handler decides what it means and
whether to continue.

The modern revival is **algebraic effects** (Koka, Eff, OCaml 5
effect handlers). Same shape, statically typed, with proper
compiler support. The leaf says "I might perform effect E"; the
surrounding handler says "I handle E by doing X." Different
handlers in different scopes give different semantics for the
same leaf. Effect systems were literally invented to fix the
function-coloring critique that motivates this ADR.

### Java's typed exceptions and why they don't fit

Java's checked exceptions are statically declared at the leaf
(`throws BackendUnreachableException`), and every intermediate
layer has to either catch or re-declare the throws clause. This
is Go's error-return problem with a different syntax — the leaf
bakes in the exception type and middleware pays. What we want
is the *type discipline* of Java's checked exceptions but with
the *handler-decides-meaning* property of CL conditions: i.e.,
algebraic effects. Java does not offer this; Go doesn't either,
but Go's panic-recover with disciplined typed payloads gets
closer than Java's `throws`.

### What Go offers and what it lacks

Used disciplined-ly, Go's panic-recover is a poor-man's effect
system:

- Leaf panics with a typed value.
- Middleware passes through.
- Boundaries `recover()` and type-switch the value.

What Go lacks:

1. **No static guarantee that handlers exist.** Compiler doesn't
   help.
2. **No type discipline on the panic payload.** Each recover does
   its own switch; payload-type typos silently fall through.
3. **No restart machinery.** Recovers can only unwind.

### Relationship to purse-first#107 / RFC 0002

This ADR concerns *control flow* — when failure should propagate
via panic vs. error-return.
[purse-first#107](https://github.com/amarbel-llc/purse-first/issues/107)
(RFC 0002, "HTTP status as semantics") concerns *data shape* —
when an error value should carry the HTTP status code as identity
in its `Error()` string vs. as semantic metadata reachable via
`errors.As`.

Both apply the same principle: **the call-site context decides
what a failure means; the leaf describes what happened.** This
ADR applies it to the control-flow axis (panic with the
description, let the outer scope decide whether to skip / abort /
log). RFC 0002 applies it to the data axis (carry the status code
as metadata, let the HTTP handler decide whether to render it on
the wire). They compose: a leaf can panic with an
`HTTPStatusError` payload and the recover boundary can both
interpret the control-flow implication and use `AsHTTPError` for
the wire response.

### Where this lands

The dewey-side machinery (`errors.Context`, `Cancel`, `runRetry`,
`ContextContinueOrPanic`) is sufficient to implement this
pattern. Consumers that adopt it should:

1. Document the convention on each interface that uses no-error
   signatures with panic-as-channel semantics, naming the
   panic-payload types that may be raised.
2. Define a small set of exported panic-payload types in a known
   package per consumer (madder's `BackendUnreachableError`,
   future analogs in dodder).
3. Establish recover boundaries at long-lived embedding entry
   points (servers, daemons, library callers).
4. Sketch a `Handle[T]` helper that standardises the recover +
   type-switch shape so every boundary site uses the same
   scaffolding.

### References

- purse-first ADR 0016
  (`docs/decisions/0016-structured-operation-context.md`, this
  repo) — the related decision to use panic-based control flow
  inside `libs/go-mcp/operation/` for skip/abort/retry semantics.
  Same philosophy, narrower scope.
- purse-first RFC 0002
  (`docs/rfcs/0002-http-status-as-semantics.md`, this repo) — the
  data-shape application of "let context decide what it means."
- amarbel-llc/dodder#27 — the parallel ADR-to-be for dodder.
  Catalogues 12 in-tree TODOs expressing migration intent.
- amarbel-llc/madder#20 — concrete cleanup pass for madder's
  error-return vs panic conventions.
- amarbel-llc/madder#134 — symptom report and the corrected
  analysis comment that motivated this ADR.
- amarbel-llc/madder commit `9b74186` —
  `remoteSftp.initializeOnce` panics directly with the
  underlying error.
- amarbel-llc/madder commit `b3e0ae9` — madder-mcp adds
  `tryOpenInStore` and `ReadResource` recover boundaries.
- Common Lisp the Language, 2nd Ed., Ch. 29 (Conditions). Steele,
  G., 1990. — The canonical reference for the
  signal-handler-restart shape.
- Koka language documentation, "Algebraic Effects." Leijen, D. —
  Modern statically-typed effect handlers; the closest existing
  language match for what this ADR describes informally.
- OCaml 5 effect handlers (RFC and OCaml manual). — Production
  language adoption of algebraic effects.
- Pretnar, M. "An Introduction to Algebraic Effects and Handlers"
  (2015). — Tutorial-style introduction; good for the conceptual
  framing.
- Dewey context machinery:
  `libs/dewey/internal/bravo/errors/context.go`, particularly
  `(*context).Cancel` and `runRetry`. The `GetState` "TODO
  identify the right terminal state" comment is a separate
  concrete bug worth filing.
