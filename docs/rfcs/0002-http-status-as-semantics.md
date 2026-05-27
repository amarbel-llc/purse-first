---
status: accepted
date: 2026-05-27
---

# HTTP Status as Semantics, Not Identity

## Abstract

This specification defines how the `libs/dewey/internal/bravo/errors`
package (re-exported via `libs/dewey/pkgs/errors`) represents HTTP
status codes attached to Go errors. It replaces the pre-existing
identity-based representation — where `BadRequestf("invalid x")`
rendered `Error()` as `"errors.HTTP: 400 Bad Request"` and hid the
underlying message behind a `ShouldHideUnwrap` flag — with a
semantics-based representation in which `Error()` returns the
underlying user message and the HTTP status code is carried as
typed metadata, detectable via `errors.As`.

Domain code that constructs HTTP-tagged errors gets useful CLI/log
rendering for free. HTTP handler code that needs the on-the-wire
"HTTP: <status>" rendering calls an explicit boundary transform
(`AsHTTPError`).

## Introduction

The pre-#107 implementation tied HTTP status semantics to the
`Error()` string. A typical CLI consumer printing
`fmt.Sprintf("%s: %s", utility, err)` would see
`"diff: errors.HTTP: 400 Bad Request"` instead of the actual error
message — the useful message
(`invalid -color value "rainbow"`) was hidden inside
`err.Unwrap()` behind a `ShouldHideUnwrap` flag, requiring every
non-HTTP consumer to walk the chain past hidden-wrapper layers.

A concrete instance of this footgun: cutting-garden's CLI bridge
contained a custom `userFacingErrorMessage` walker (see
`internal/command/utility_run.go` pre-#107) that existed solely to
dig past the `ShouldHideUnwrap` layers introduced by dewey's HTTP
constructors. Other downstream consumers (madder, future
framework users) would need the same walker.

This RFC moves the status code to be *semantic metadata* on the
error chain, leaving the user message in `Error()`. The
identity-style rendering ("HTTP: <status>") remains available
through an explicit boundary transformer that HTTP handlers call
when producing the response.

## Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT",
"SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this
document are to be interpreted as described in RFC 2119.

## Specification

### 1. Scope

This specification applies to the `errors` package at
`libs/dewey/internal/bravo/errors` and its facade at
`libs/dewey/pkgs/errors`. Downstream consumers using these
packages (cutting-garden, madder, moxy, etc.) MUST follow the
contracts here when constructing or consuming HTTP-tagged errors.

### 2. Types

#### 2.1 `HTTPStatusError` (concrete)

The package MUST export a struct type `HTTPStatusError` carrying an
HTTP status code and an underlying error. Its `Error()` method MUST
return the underlying error's message (or the status string when
the underlying is nil).

```go
type HTTPStatusError struct { /* unexported fields */ }

func (e HTTPStatusError) Error() string             // underlying message
func (e HTTPStatusError) Unwrap() error             // for errors.Is/As
func (e HTTPStatusError) HTTPStatusCode() hs.Code   // semantic tag
func (e HTTPStatusError) HTTPRender() error         // wire form
func (e HTTPStatusError) Errorf(string, ...any) HTTPStatusError
func (e HTTPStatusError) Wrap(error) HTTPStatusError
func (e HTTPStatusError) WithStack() error
```

`HTTPStatusError` MUST NOT implement `interfaces.ErrorHiddenWrapper`.
Tree-rendering transparency is handled by the renderer (see §4.1).

#### 2.2 Interfaces

```go
type HTTPStatusCarrier interface {
    error
    HTTPStatusCode() hs.Code
}

type HTTPRenderer interface {
    HTTPStatusCarrier
    HTTPRender() error
}
```

`HTTPStatusCarrier` is the semantic tag used by detection helpers
(`IsHTTPError`, `Is400BadRequest`, etc.) via `errors.As`. Any error
that wants to participate in HTTP-status detection MUST implement
this interface.

`HTTPRenderer` is the explicit boundary transformer. HTTP wire
layers MUST call `AsHTTPError(err)` (which uses this interface) to
produce the on-the-wire rendering. Domain code SHOULD NOT call
`HTTPRender()` directly.

#### 2.3 Wire form (unexported)

The package MUST define an unexported `httpRendering` struct
returned by `HTTPRender()`. Its `Error()` MUST render
`"HTTP: <status>"`. It MUST implement `HTTPStatusCarrier` and
`HTTPRenderer` (self-returning) so that the wire form remains
detectable as a status-carrying error after the boundary
transform.

### 3. Constructors

#### 3.1 Status-specific constructors

The package MUST export the following top-level constructors,
each returning `HTTPStatusError`:

```go
func BadRequest(err error) HTTPStatusError      // tag with 400
func BadRequestf(format, args ...any) HTTPStatusError
func BadRequestWrapf(format, args ...any) HTTPStatusError  // = BadRequestf
func Conflictf(format, args ...any) HTTPStatusError
func UnprocessableEntityf(format, args ...any) HTTPStatusError
func NotImplementedf(format, args ...any) HTTPStatusError
```

`BadRequest(err)` MUST be idempotent: if `err`'s chain already
carries a 400 tag, the existing error is returned unchanged.

`BadRequestWrapf` is preserved as an alias for `BadRequestf`;
pre-#107 they were already identical.

Additional status-specific constructors MAY be added without
breaking compatibility.

#### 3.2 Sentinel-receiver constructors

Each sentinel (§3.3) MUST expose `Errorf(format, args)` and
`Wrap(err)` methods returning a new `HTTPStatusError` sharing the
sentinel's status. This preserves the receiver-pattern call style
(`Err422UnprocessableEntity.Errorf("tampered")`) used by existing
code.

#### 3.3 Sentinels

The package MUST export the following package-level variables of
type `HTTPStatusError`. Each MUST be a valid error whose `Error()`
returns its status-line string (e.g. `"400 Bad Request"`):

```go
Err400BadRequest, Err405MethodNotAllowed, Err409Conflict,
Err422UnprocessableEntity, Err499ClientClosedRequest,
Err500InternalServerError, Err501NotImplemented
```

Sentinels are usable as bare error values (e.g.
`ctx.Cancel(Err499ClientClosedRequest)`) and as receiver-style
constructors (§3.2).

### 4. Detection and Transformation

#### 4.1 `IsHTTPError(err, code)` and specific helpers

```go
func IsHTTPError(err error, code hs.Code) bool
func Is400BadRequest(err error) bool
func Is499ClientClosedRequest(err error) bool
```

`IsHTTPError` MUST walk `err`'s chain via `errors.As` looking for
any `HTTPStatusCarrier`, and MUST return true iff one matches the
given code. The specific helpers (`Is400BadRequest`,
`Is499ClientClosedRequest`) are thin wrappers.

These helpers MUST work transparently through `Wrap`,
`WithoutStack`, `WithHelp`, and any other wrapper that implements
`Unwrap`.

#### 4.2 `AsHTTPError(err) (error, bool)`

```go
func AsHTTPError(err error) (error, bool)
```

`AsHTTPError` MUST walk `err`'s chain via `errors.As` looking for
any `HTTPRenderer`. On match, it MUST return the renderer's wire
form (`r.HTTPRender()`) and `true`. On no match, it MUST return
`(nil, false)`.

HTTP handlers MUST call this at the wire boundary to obtain the
response error shape. Domain code SHOULD NOT depend on the wire
form.

### 5. Tree-rendering integration

The CLI tree encoder (`internal/charlie/ui/cli_error_tree_state.go`)
MUST collapse any single-child wrapper whose `Error()` is
byte-identical to its child's `Error()`. This implements
"transparent passthrough" generically — covering `HTTPStatusError`
and any future wrapper that delegates `Error()` to its underlying —
without requiring those wrappers to implement
`interfaces.ErrorHiddenWrapper`.

Wrappers that contribute information to the tree (`errWithoutStack`,
`helpful`, `httpRendering`) MUST have `Error()` strings distinct
from their underlying's `Error()` and so will not be collapsed.

### 6. Removed surface

The following pre-#107 symbols MUST NOT be present:

- The unexported `http` struct, `httpErrDisamb`, the `exposeHTTP`
  field, and the `ShouldHideUnwrap`/`GetErrorType`/`GetStatusCode`/
  `Is(target error) bool`/`WrapIncludingHTTP` methods on it.
- The rendering `"errors.HTTP: <status>"` (replaced by
  `"HTTP: <status>"`).

`interfaces.ErrorHiddenWrapper` and its `ShouldHideUnwrap()` method
remain — they're still implemented by `errWithoutStack` and
`helpful`. Removing them is out of scope for this RFC.

## Migration

### Downstream consumers

| Repo | Action |
|---|---|
| `cutting-garden` | After dewey bump, `internal/command/utility_run.go`'s `userFacingErrorMessage` collapses to `err.Error()`. Removal is a one-line cleanup. |
| `madder` | No code change required. Bump dewey version. |
| `moxy` | No usage of affected helpers. No action. |

### Compatibility surface preserved

- `BadRequest`, `BadRequestf`, `BadRequestWrapf` — same names, return
  type widened from `error` to concrete `HTTPStatusError` (still
  assignable to `error`).
- `Is400BadRequest`, `Is499ClientClosedRequest`, `IsHTTPError` —
  unchanged.
- `Err4xx`/`Err5xx` sentinels — same names. Type changes from
  unexported `http` to exported `HTTPStatusError`. Receiver methods
  `Errorf` and `Wrap` retained.
- Context-cancel helpers (`ContextCancelWith499ClientClosedRequest`,
  `ContextCancelWithBadRequestError`, `ContextCancelWithBadRequestf`,
  `CancelWithNotImplemented`) — unchanged.

### Compatibility surface broken

- `Err501NotImplemented.WrapIncludingHTTP(inner)` — replace with
  `Err501NotImplemented.Wrap(inner).HTTPRender()` or
  `AsHTTPError(Err501NotImplemented.Wrap(inner))`.
- Direct dependence on `Error()` returning `"errors.HTTP: <status>"`
  — now returns the underlying user message; the wire form must be
  obtained via `AsHTTPError`.

## Followups (out of scope for this RFC)

1. **Unify tree-rendering interfaces.** `UnwrapMany`,
   `ErrorsAndFramesGetter`, and the transparent-passthrough
   special case all describe parent/children relationships. A
   single `ErrorTreeNode` interface (e.g.
   `ChildErrors() []error`) decoupled from `Unwrap()` would
   eliminate the special-casing in the encoder switch.
2. **Drop `ShouldHideUnwrap`.** After (1), the
   `interfaces.ErrorHiddenWrapper` interface is dead — both
   `errWithoutStack` and `helpful` already have
   `Error() == underlying.Error()` and would be collapsed by the
   generic mechanism. Removing this interface is a clean follow-on.
3. **Add status-specific helpers for 405/500/499.** Currently those
   statuses are usable only via the receiver pattern
   (`Err500InternalServerError.Errorf(...)`). Adding top-level
   `InternalServerErrorf(...)` etc. is purely additive.

## References

- Issue: amarbel-llc/purse-first#107
- Motivating downstream issue: amarbel-llc/cutting-garden#35 (closed
  with a workaround that this RFC obviates).
