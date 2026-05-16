# catgut

String format read/write interfaces for ring buffer operations.

## Key Interfaces

- `StringFormatReader`: Read formatted strings from ring buffer
- `StringFormatWriter`: Write formatted strings
- `StringFormatReadWriter`: Combined read/write interface

## Known `go vet` warning: `noescape` in string.go

`go vet ./libs/dewey/...` will report `possible misuse of unsafe.Pointer`
at `string.go::noescape`. **This is intentional** and matches the pattern
used inside the stdlib (`strings.Builder`, `internal/abi.NoEscape`,
`unique`, the runtime's specialized maps, etc.) to defeat Go's escape
analyzer.

The `String.addr = (*String)(noescape(unsafe.Pointer(b)))` line in
`copyCheck()` exists to enable copy-by-value detection without forcing
the receiver onto the heap. Plain `b.addr = b` would heap-allocate every
`String` because of [golang/go#7921][i7921] (still open). The `noescape`
helper round-trips through `uintptr` to hide that data dependency from
escape analysis; the `uintptr → unsafe.Pointer` cast in the body is
exactly what `unsafe.Pointer(uintptr(...))` looks like, which is what
the vet check is designed to flag.

### Why we don't silence it today

- A public `runtime.NoEscape` / `unsafe.NoEscape` has been proposed
  ([golang/go#58625][i58625], [golang/go#70471][i70471]) but neither
  has landed.
- `//go:linkname` to stdlib's `internal/abi.NoEscape` is blocked by
  the Go 1.23+ allowlist (`invalid reference to internal/abi.NoEscape`).
- `go vet -unsafeptr=false` would disable the check *module-wide* and
  hide future genuine misuse.
- Per-arch assembly stubs would work but cost an `.s` file per Go
  architecture we ever cross-compile to.

Adopting `golangci-lint` (which supports per-line `//nolint:govet`)
is being explored in [purse-first#89][issue89]. Until that decision
lands, the warning stays — and this section is the prior-art reference
that future contributors should look at before touching `noescape`.

[i7921]:    https://github.com/golang/go/issues/7921
[i58625]:   https://github.com/golang/go/issues/58625
[i70471]:   https://github.com/golang/go/issues/70471
[issue89]:  https://github.com/amarbel-llc/purse-first/issues/89
