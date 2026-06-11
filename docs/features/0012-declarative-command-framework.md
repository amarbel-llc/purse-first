---
status: proposed
date: 2026-06-11
promotion-criteria: One real dewey binary (e.g. dagnabit) authors all its commands on the framework; flags AND grammar-shaped positional args (including a repeating heterogeneous group) complete in bash/fish/zsh, all generated from each command's Run signature; MCP input schema + manpage generated from the same signature with zero hand-written duplication; generated-code drift gated in CI; no `any`-typed config seam.
---

# Declarative command framework (futility → dewey)

## Problem Statement

dewey, dodder, cutting-garden, and madder each carry a near-identical CLI command framework descended from dewey's own `golf/command` — but every copy leaves the same promises half-built: argument metadata is decorative (it feeds only the manpage while parsing and completion are hand-rolled and drift from it), heterogeneous positional-argument completion is declared but unreachable, arg parsing is either eager or imperatively re-stated inside `Run`, and the config layer is erased to `any` and recovered with unguarded runtime type-assertions. We want one framework, in dewey, where **a command's `Run` signature is the single authoritative declaration** — driving lazy parsing, a real positional-argument grammar, shell completion, MCP tool schemas, and manpages — with the type-erasure designed out entirely.

## Background / lineage

This is not a greenfield framework; it is a consolidation of one that already exists in four places:

- madder's `futility` was **copied from** dewey's `golf/command` (now `internal/echo/command`) as a deliberate incubation (madder #35, refs purse-first#63) to iterate on params + manpage support without slow upstream round-trips. The plan was always to upstream it back.
- purse-first#63 is **closed** with the explicit mandate: `golf/command` *and* go-mcp's `command` framework are **superseded by upstreaming `futility`**. This FDR is the realization of that mandate.
- cutting-garden's `internal/command` and dodder's `delta/command` are parallel forks of the same ancestor. `futility` is already **dewey-only** in its dependencies (it imports `pkgs/*` + stdlib, zero madder domain code), so the relocation is near-mechanical at the framework layer.

Consequently this framework **replaces** `libs/dewey/internal/echo/command` and go-mcp's `command`. Migration is incremental: the new package is built alongside; existing consumers move over command-by-command.

A first draft of this FDR proposed a runtime `GetParams()` declaration (futility's `Flag[V]`/`Arg[V]` model, improved). Design review killed it for two structural reasons, recorded here so they aren't relitigated:

1. **Args broke the single-source promise.** Flags bound to struct fields once, but positional args were declared in `GetParams()` *and* re-stated in `Run` via `PopArg(name)` — name, type, and order duplicated across declaration and handler. Field-binding args would fix the drift but make parsing **eager**, losing `PopArg`'s intentional laziness (don't resolve expensive identifiers the handler may never use; let consumption depend on runtime state).
2. **The positional-index completion model only covered fixed-position heterogeneity.** The arg shape these tools actually use — content-classified streams like `capture (STORE_ID BLOB_ID*)*`, where a token's kind is decided by its lexical form, not its position — fell back to an imperative runtime classifier that the declarative completion machinery never saw.

The signature-as-grammar design below resolves both, at the cost of an author-facing generate step — a trade-off accepted deliberately (see Interface § Codegen).

## Interface

### The Run signature is the declaration

A command is a plain Go struct with one generated-against method. The **entire CLI surface is derived from the method signature** (receiver fields → flags; typed parameters after `Request` → the positional-argument grammar) by `futility`'s generator:

```go
//go:generate futility command
func (c *Capture) Run(req futility.Request, args interfaces.SeqError[CaptureGroup])
```

The generator (driven by `go/types`, not string templates over names) emits, per command:

- the **flag binding + parse** code (every exported receiver field becomes a flag),
- the **positional-argument parser** implementing the grammar below, surfaced lazily,
- the **shell-completion dispatch** (flag names, flag values, and grammar-position-aware argument candidates),
- the **MCP `InputSchema`**, and
- the **manpage SYNOPSIS/OPTIONS** fragments.

Because all five surfaces are generated from one signature, they cannot drift from each other or from the handler — the handler *consumes the same types the declaration is made of*.

### Flags: receiver fields

Every exported field of the command struct is a flag. The flag name is the kebab-cased field name; the description comes from the field's doc comment; the schema type and validation come from the field's type (dewey `interfaces.FlagValue` implementers get `Set()` validation; plain `bool`/`int`/`string` map directly).

```go
// Capture ingests blobs into one or more stores.
type Capture struct {
    // All lists every store containing each blob.
    All bool
    // Format selects the output encoding.
    Format values.Format
}
```

There is no `SetFlagDefinitions` and no `GetParams()` — the dual-declaration drift class (madder #143, #166) is structurally impossible because there is only one place a flag can be declared.

### Flag extras: typed homes first, directives for the residue

Beyond name/description/type, flags need defaults, enum values, value completion, and occasionally presentation metadata (short names, hidden, deprecated). The design rule: **push each extra into the type system wherever it is semantic; only the purely presentational residue gets an annotation.** Struct tags were evaluated and rejected — they are an unchecked string mini-DSL (unknown keys compile clean and silently no-op, the worst failure mode), they duplicate what the value type already knows (`enum=text|json` re-states `values.Format`'s domain; `default=text` can drift from `values.FormatText`), and a tag cannot reference code, so completer functions are inexpressible.

| Extra | Home | Checked by |
|---|---|---|
| description | field doc comment | — (prose) |
| **default** | the field's initial value at registration (`&Capture{Format: values.FormatText}`) | compiler |
| **enum values** | `EnumValues() []string` on the value type | compiler + generator |
| **value completion** | `CompleteCLI(req) interfaces.SeqError[Completion]` on the value type — the *same* mechanism positional symbol types use | compiler + generator |
| short name, hidden, deprecated | `//futility:flag key=value` doc-comment directive | generator (unknown key = hard error) |

- **Defaults need no mechanism at all.** The registered literal *is* the default — an ordinary compile-checked Go expression, refactor-safe, nothing novel to learn. Consequence: default text in help/manpages is rendered at runtime from the live registry via `String()` (the binary-owns-the-grammar generation model already works this way), and a command type registered twice with different literals legitimately documents two different defaults.
- **Enum values and completion attach to the value type, once,** and every command using that type — as a flag *or* a positional grammar symbol — gets them for free. Two flags of the same Go type complete identically by construction; when that's wrong, the answer is a newtype ("completes differently ⇒ is a different thing"). The completer interface lives in dewey's low `interfaces` tier (where `FlagValue` already lives) so domain packages implement it without importing the framework; the registration-table alternative was rejected because a type-keyed lookup is a soft form of the erasure this design exists to kill.
- **The directive grammar is minimal and closed:** presentation-only, enumerated keys, no escape hatch. Any semantic capability that leaks into the directive channel migrates metadata from the compiler's jurisdiction back to a string grammar — rebuilding struct tags with extra steps.

### UX and AX of the metadata model

The allocation above was chosen by analyzing developer experience and **agent experience** together. Agents discover conventions by grep and neighboring examples, verify through feedback loops (compiler > generator > CI > runtime), and carry training-data priors that can actively mislead: struct tags invite confidently hallucinated keys from *other* tag grammars (`futility:"required"`), which then fail *silently* — the worst possible failure mode for an agent loop, since nothing in the feedback channel says "wrong."

Every chosen mechanism is either **hallucination-resistant** (interface satisfaction and plain Go initial values — nothing novel exists to get wrong, and a wrong method signature fails the generator's interface check loudly) or **hallucination-loud** (an invented directive key hard-fails the generator, and the error lists the valid key set — the error channel doubles as the documentation). Directives are first-class greppable (`rg '//futility:'` surfaces every usage as a learnable example); defaults-by-initial-value requires no convention at all beyond Go itself.

The model's known weakness is that type-method metadata is **invisible at the use site** — reading `Format values.Format` reveals nothing about enum or completion, and the *absence* of an annotation isn't greppable. The generated-file summary header (see Codegen) is the deliberate mitigation: everything the generator inferred is readable in one drift-gated place.

### Positional args: a grammar encoded in types

The parameters after `req` encode an argument grammar by convention:

| Signature shape | Grammar meaning |
|---|---|
| scalar field in a tuple struct | exactly one token of that symbol type |
| slice field in a tuple struct | zero-or-more tokens of that symbol type |
| tuple struct fields, in order | sequence |
| `interfaces.SeqError[Tuple]` parameter | the group repeats (zero-or-more) |
| plain `Tuple` parameter | the group appears exactly once |

So the content-classified shape every one of these tools actually has — cutting-garden's `capture [STORE_ID \| BLOB_ID]...`, madder's `arg_resolver` streams — is written:

```go
// CaptureGroup is one grammar group: STORE_ID BLOB_ID*
type CaptureGroup struct {
    StoreId madder.StoreId
    BlobIds []madder.BlobId
}
```

giving the grammar `( STORE_ID BLOB_ID* )*` — declaratively, in the type system, where previously this lived in a hand-rolled runtime classifier invisible to completion and docs. Fixed-position heterogeneity is just the degenerate case (a single non-repeating tuple of scalars).

**Determinism precondition:** group-boundary symbols must be content-distinguishable — the generator's parser decides "next group or more of the current slice?" by classifying the token (cheap, lexical; e.g. the symbol type's `Set()` accepting/rejecting). This is the same precondition the existing runtime classifiers already rely on; the grammar makes it explicit, and the generator rejects grammars whose adjacent symbols it cannot distinguish.

### Lazy by construction: `interfaces.SeqError`

The handler receives the parsed groups as `interfaces.SeqError[T]` (`= iter.Seq2[T, error]`) — a **pull-based** stream:

```go
func (c *Capture) Run(req futility.Request, args interfaces.SeqError[CaptureGroup]) {
    for group, err := range args {
        if err != nil { req.Cancel(err); return } // classify/parse failure for THIS group
        // ... use group.StoreId / group.BlobIds
    }
}
```

Nothing is parsed before `Run`; each group is classified when pulled, and per-group failures arrive in-band on the error channel. This preserves both lazinesses `PopArg` existed for: no expensive resolution for args the handler never reaches, and consumption that can stop early. (Assumption: token *classification* is cheap/lexical while *resolution* — opening the store, hitting the index — is the expensive part and stays on the handler/symbol-type side. See Tuning Levers.)

### Grammar-driven completion (the keystone)

Every ancestor framework had typed arg metadata, a command-line cursor, and completer interfaces — but none connected them, so positional completion was uniform or dead (madder #161/#48/#47). Here completion is computed **from the grammar**: at the cursor's position the generated dispatch replays the grammar over the already-typed tokens, determines the set of *grammatically valid next symbols*, and unions their completers — e.g. at a group boundary in `( STORE_ID BLOB_ID* )*`, both the store-id and blob-id completers fire; mid-group only blob-id's does.

Completers attach to **symbol types** (the same mechanism for flag values and positional args), so a type like `madder.StoreId` carries its completion source once and every command using it completes correctly with zero per-command completion code.

Shell integration keeps the proven model: generated bash/zsh/fish stubs hold zero per-command knowledge and shell out to a hidden `__complete` subcommand; the binary owns the grammar. All three shells get the same dispatch (fixing the zsh-only-subcommands asymmetry).

### Context and error flow

`Run` receives a `Request` that **embeds `errors.ActiveContext`** — the consumer subset of dewey's context (stdlib `context.Context` + `Cause() error`, `GetState()`, `Cancel(error)`, `After(...)`). Error reporting is **cancel-the-context**: `Run` has no `error` return; a handler reports failure with `req.Cancel(err)` and returns. The framework observes the cancellation cause and maps it to an exit code (diff(1)/git-style: 0 ok, 1 mismatch, 2 trouble, 64 EX_USAGE for bad-request errors). The framework retains the `errors.Context` supervisor internally (signal-arming, sub-contexts); a handler that genuinely needs supervisor powers reaches it through an escape hatch (see Open Questions).

### Config injection without `any`

The `Utility` (the name→command registry + dispatch loop) is **parameterized by the binary's config type** — no `GetConfigAny() any`, no `FromAny` assertion:

```go
type Utility[C any] struct { /* ... */ }
func (u Utility[C]) Config() C // compile-time-typed
```

dodder couldn't do this because `delta/command` sits below the concrete config in a strict NATO tier DAG; a fresh dewey home with a type parameter is not bound by that and deletes the erasure layer.

### Codegen: embraced, bounded, drift-gated

The first draft targeted "no author-facing codegen." That goal is **deliberately dropped**: codegen is what makes full type safety, a real grammar, lazy parsing, and free completion simultaneously achievable. The generated code references `madder.StoreId` etc. *concretely* — so the stringly-typed schema round-trip (`V → jsonSchemaType() → new(bool/int/string)`) and every runtime type-assert simply do not exist.

Bounds on the cost:

- One marker per command (`//go:generate futility command`); the generator reads the signature via `go/types`.
- Generated files are committed and **drift-gated in CI** with a `--check` mode, exactly like the existing `dagnabit export --check` / `lint-dewey_pkgs_drift` lane — a stale signature fails the gate, it cannot silently skew.
- The grammar parser and the completion dispatch are emitted from the *same* grammar IR, eliminating the two-copies-in-two-languages drift madder's hand-emitted bash positional logic has.

Two requirements promoted from nice-to-have by the AX analysis:

- **The generated file opens with a summary header** — a per-command manifest of everything the generator inferred: each flag with the source of its description/enum/completer/default, and the rendered arg grammar (`flag --format: enum [text json] via values.Format.EnumValues; args: ( STORE_ID BLOB_ID* )*`). This is the inspection surface that compensates for type-method metadata being invisible at the use site, for humans and agents alike; the drift gate keeps it truthful.
- **Generator errors must teach.** Unknown directive key → error listing the valid keys. Symbol type missing a required interface → error naming the type and the interface. Undecidable grammar → error naming the two indistinguishable symbols. The generator's error channel is the convention's primary documentation.

### Drawbacks deliberately designed out

- **Three-layer type erasure** — `Utility.config any` + unguarded `FromAny` panics + the deref-copy write trap → `Utility[C]` and concretely-typed generated code.
- **Declarative/imperative drift** — `GetParams()` vs `SetFlagDefinitions` (madder #143), `GlobalParams` vs `GlobalFlagDefiner` (madder #166), and the first draft's `GetParams()` vs `PopArg` → one declaration site (the signature).
- **Hand-written MCP schemas** drifting from arg metadata (dodder's `GetArgs()` had one real consumer) → schema generated from the signature.
- **Unreachable positional completion** (madder #161/#48/#47) → grammar-driven dispatch.
- **Eager or re-stated arg parsing** → `SeqError` lazy pull.
- **Package-level mutable `commandComponentWriters` map** → gone with `SetFlagDefinitions`.
- **Asymmetric shell support** (zsh completing only subcommand names) → one generated dispatch for all shells.

## Examples

A capture-style command — flags from receiver fields, a repeating heterogeneous group grammar, lazy consumption. (Domain types like `madder.StoreId` are illustrative; the framework only requires symbol types implement dewey's `interfaces.FlagValue` plus optionally a completer.)

```go
package capture

import (
    "github.com/amarbel-llc/purse-first/libs/dewey/pkgs/futility"
    "github.com/amarbel-llc/purse-first/libs/dewey/pkgs/interfaces"
)

// Capture ingests blobs into one or more stores.
type Capture struct {
    // All lists every store containing each blob.
    //
    //futility:flag short=a
    All bool
    // Format selects the output encoding.
    Format values.Format // enum values + completion come from the type
}

// CaptureGroup is one grammar group: STORE_ID BLOB_ID*
type CaptureGroup struct {
    StoreId madder.StoreId // positional completion comes from the type, too
    BlobIds []madder.BlobId
}

//go:generate futility command
func (c *Capture) Run(req futility.Request, args interfaces.SeqError[CaptureGroup]) {
    for group, err := range args {          // lazy: classified per pull
        if err != nil {
            req.Cancel(err)                 // cancel-the-context error flow
            return
        }

        store, err := openStore(req, group.StoreId) // expensive resolution: handler-side, on demand
        if err != nil {
            req.Cancel(err)
            return
        }

        for _, id := range group.BlobIds {
            if err := store.Capture(req, id); err != nil {
                req.Cancel(err)
                return
            }
        }
    }
}
```

Registration (no codegen at the registry level):

```go
func Build() futility.Utility[AppConfig] {
    u := futility.MakeUtility[AppConfig]("myapp", appConfig)
    u.AddCmd("capture", &capture.Capture{Format: values.FormatText}) // the literal IS the defaults
    return u
}

func main() { os.Exit(Build().Run(os.Args)) } // Run never calls os.Exit itself
```

Resulting behavior, all derived from the one signature:

```
$ myapp capture --a<TAB>                  # → --all  (-a)    (receiver field + directive, free)
$ myapp capture --format <TAB>            # → text  json     (Format's value completion)
$ myapp capture <TAB>                     # → <store ids>     (grammar: group must start with STORE_ID)
$ myapp capture store-a <TAB>             # → <blob ids> ∪ <store ids>
                                          #   (grammar: BLOB_ID continues the group OR a new STORE_ID begins one)
$ myapp capture store-a b1 b2 store-b b3  # parses as (store-a [b1 b2]) (store-b [b3])
```

`myapp capture --help`, the `myapp-capture(1)` manpage, and the MCP tool schema all render `capture [--all] [--format FORMAT] (STORE_ID BLOB_ID*)*` from the same source. A fixed-position command (e.g. `diff RECEIPT_ID DIR`) is the degenerate case: a single non-repeating tuple of two scalar fields.

## Limitations

- **Replaces, does not wrap.** This supersedes `internal/echo/command` and go-mcp's `command`; it is not a compatibility shim. Consumers migrate command-by-command.
- **Author-facing codegen is required for the command surface.** Accepted deliberately; the generated files are committed and drift-gated, but adding/changing a command's signature requires re-running the generator.
- **The grammar is static per command.** A signature cannot express "if `--foo` was passed, expect different positional args." Commands needing runtime-conditional consumption use an imperative escape (a raw lazy cursor on `Request`) and forfeit generated arg completion for the conditional part.
- **Grammar determinism is the author's responsibility (checked, not solved):** adjacent symbol types at decision points must be content-distinguishable. The generator rejects grammars it cannot decide; it does not implement backtracking or lookahead beyond one token.
- **Laziness is bounded by classification cost.** If classifying a token is itself expensive for some symbol type, pulling a group pays that cost; only *resolution* is deferred to the handler.

## Open Questions

- **Accepted `Run` signature shapes.** The canonical set (Request-only; Request + plain tuple; Request + `SeqError[tuple]`; scalars directly?) needs pinning so the generator's contract is small and explainable.
- **ActiveContext escape hatch.** Exact shape for a handler needing supervisor powers (arm signals, spawn sub-contexts): `req.Supervisor() (errors.Context, bool)` accessor vs a capability interface.
- **MCP result return.** Given `Run` has no error return, how a command surfaces a structured MCP `*Result`: an opt-in `RunResult` interface vs a result sink on `Request`.
- **Migration sequencing.** Order for moving `internal/echo/command` consumers (dewey's own CLIs) and go-mcp `command` consumers (the MCP packages); whether the two old packages overlap a release or die together.

## Tuning Levers

| Lever | Current | Rationale | Change signal |
|---|---|---|---|
| grammar power | regular (sequences, `*`, one-token decisions) | covers every real arg shape in dodder/madder/cutting-garden; trivially explainable | a real command needs lookahead >1 or value-dependent grammar |
| classification mechanism | symbol type's `Set()` accept/reject (lexical) | reuses `FlagValue`; no parallel classifier API | a symbol type's `Set()` is too expensive or too permissive to classify with |
| generated-code policy | committed + CI `--check` drift gate | matches `dagnabit export --check` precedent; reviewable diffs | generated files dominate diffs → consider build-time-only generation |
| context interface on `Request` | `errors.ActiveContext` (consumer subset) | narrowest supporting cancel-the-context flow; aligns with #150 | many handlers reach for the supervisor escape hatch |
| flag declaration source | exported receiver fields | zero ceremony; doc comments double as help text | commands need many non-flag exported fields → opt-out marker |
| directive key set | `short`, `hidden`, `deprecated` (closed, presentation-only) | hallucination-loud, greppable, nothing semantic can drift through it | a key request that is semantic → it belongs on a type, not in the directive |
| completer attachment | `CompleteCLI` method on the value type; interface in the `interfaces` tier | compile-checked, write-once-per-type, same mechanism for flags + positional symbols | a domain package that cannot implement the method → revisit a registration fallback (accepting its type-keyed lookup) |

## More Information

- **purse-first#63** (closed) — the mandate: `golf/command` and go-mcp `command` superseded by upstreaming `futility`.
- **madder#35** — the incubation that copied `golf/command` into madder as `futility`.
- **madder#161 / #48 / #47** — `Arg[V]` lacks a `Completer`; positional-arg completion unreachable (closed here by grammar-driven dispatch).
- **madder#143 / #166** — dual-declaration drift (structurally impossible here).
- **madder#52** — pivot `Utility.GlobalFlags` from `any` to a type parameter (adopted as `Utility[C]`).
- **madder#49** — missing tests for the completion/manpage generators (addressed by generating parser + completion from one IR, plus the drift gate).
- **purse-first#150** — ecosystem-wide `context.Context` → `errors.ActiveContext` migration; `Request` adopts `ActiveContext` in line with it.
- Research synthesized from live sessions: dodder (`delta/command`), cutting-garden (`internal/command`), madder (`futility`). The first-draft runtime-`GetParams()` design and its rejection rationale are recorded in Background.
