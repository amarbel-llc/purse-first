---
status: proposed
date: 2026-05-23
promotion-criteria: one shipping caller of `Run` (single-op), one shipping
  caller of `RunBatch` (many-op with progress), and one shipping caller of
  the raw `Model` (custom event source)
---

# Operation Viewport

## Motivation

Several tools in this ecosystem run opaque child processes and need to
show progress on a TTY without dumping the entire transcript inline:

- **clown** runs `podman load -i <tarball>` on first launch; the
  hand-rolled `cmd/clown/tent_loader.go` (~210 lines) renders a spinner
  plus a 5-line rolling tail until podman exits.
- **tap-dancer** wants the same UX for streamed TAP-14 Output Blocks
  (see [`amarbel-llc/tap` issue #21][tap-21]) so wrappers like
  `tap-dancer go-test`, `cargo-test`, `exec`, `exec-parallel` collapse
  per-test-point output into a viewport above the in-progress test
  point.
- **madder fsck**, **cutting-garden capture**, **maneater indexing**,
  **nebulous indexing** all iterate over a known set of work items
  (blobs, captures, documents). They want a progress bar driven by item
  count *plus* a rolling tail of the current item's output.

Three near-duplicates would be worse than one. Two would be excusable;
clown's tent_loader is already shipping. The third would set the
precedent. Promoting the pattern into dewey gives every future caller
the same UX without re-inventing the bubbletea/lipgloss layer, and
gives the existing caller (clown) a migration target.

This FDR scopes the dewey-side primitive. The tap-dancer-side TAP
integration is a separate FDR in `amarbel-llc/tap` because it carries
TAP-specific semantics (collapse-on-`ok N`, hold-on-`not ok N` with
YAML diagnostic, subtest depth tracking) that have no place in a
generic primitive.

[tap-21]: https://github.com/amarbel-llc/tap/issues/21

## Design

### Package shape

Proposed location: `libs/dewey/pkgs/operation_viewport`.

Public surface (thin facade over `internal/.../operation_viewport`):

| Symbol | Purpose |
|---|---|
| `Model` | bubbletea `tea.Model`; composable, driven by message types |
| `Run` | high-level: spawn one child, render until exit, return its error |
| `RunBatch` | high-level: drive many ops with progress bar over count |
| `Op` / `Batch` | option structs for `Run` / `RunBatch` |
| `LogLine`, `OperationStarted`, `OperationProgress`, `OperationDone`, `BatchDone` | message types for callers driving the raw `Model` |
| `Style` / `WithStyle` | opinionated default styles, overridable |

### Two-tier API

The primitive is intentionally tiered:

1. **Raw `Model`** — bubbletea model accepting the message types above.
   Callers with a bespoke event source (tap-dancer's TAP parser) feed
   it by sending messages on the `tea.Program`.
2. **`Run` / `RunBatch` helpers** — own the child-process lifecycle:
   allocate the PTY, scan output lines into `LogLine` messages, wire
   cancellation, dump the captured transcript on failure. Callers
   describe *what* they want done; the helper picks the layout.

### Message protocol

```go
type LogLine struct {
    Text string
}

type OperationStarted struct {
    Name  string  // human-readable label, e.g. "blob abc123"
    Index int     // 1-based position in the batch
    Total int     // total operations in the batch; 0 means "unknown"
}

type OperationProgress struct {
    Current int   // within-op progress (rare; most ops are atomic)
    Total   int
}

type OperationDone struct {
    Err error     // nil = success; tail collapses
}

type BatchDone struct {
    Err error     // nil = success summary; non-nil dumps captured buffer
}
```

`Index`/`Total` on `OperationStarted` is what wires up the
`bubbles/progress` bar. For a single op (clown), `Total=1` and the
progress bar is hidden — the layout collapses to spinner + tail. For a
many-op batch (madder fsck), `Total` drives the progress bar at the
top.

`OperationProgress` is for callers that want a secondary in-op bar
(maneater indexing: lines-processed-per-document). Most callers will
never send it. Including it in the protocol from v0 is cheap and
avoids a breaking change later.

### Layout

```
<spinner> <title>                            [████████░░░░░░] 12/24
│ line N-4
│ line N-3
│ line N-2
│ line N-1
│ line N   (most recent)
```

The progress bar is only rendered when `Total > 1`. The tail keeps the
most recent `Lines` rows (default 5) and replaces oldest on overflow.

On `BatchDone{Err: nil}`: collapse to a single success line
(`✓ <title>` style — opinionated). On non-nil err: render a failure
line and dump the captured buffer to stderr so the user sees what the
child actually said.

### PTY allocation (v0)

`Run` / `RunBatch` allocate a PTY for the child via
[`charmbracelet/x/xpty`][xpty] (Unix + Windows ConPTY) so the child
sees `isatty(stdout) == true` and emits SGR colors / Unicode glyphs as
intended. Without PTY allocation, most CLIs (cargo, go test, podman)
disable colors when piped.

The PTY output stream is line-scanned and forwarded as `LogLine`
messages. Bytes are also tee'd to a capture buffer so failure dumps
have the full transcript regardless of how much of it the user saw.

SGR escape sequences pass through unchanged. lipgloss + `x/ansi` are
ANSI-aware for width math, so colored lines do not break the layout.
Non-SGR CSI sequences (cursor positioning, erase, scroll) are stripped
on the consumer side per the security considerations in tap's
`ansi-yaml-output-amendment.md`. The practical effect: commands that
emit `\r`-overwritten progress bars degrade to "last line wins" in the
tail — acceptable for v0, matches tent_loader's current behavior.

[xpty]: https://github.com/charmbracelet/x/tree/main/xpty

### Terminal emulation (v1, opt-in)

For callers wrapping commands that depend on real cursor / line-erase
behavior (cargo's "Building [=> ] 30%" bar, nix's status line, pip's
`\r`-overwrites), v1 adds an opt-in "pane mode" backed by
[`charmbracelet/x/vt`][vt]:

- `WithPaneMode()` option on `Op` / `Batch` (and the raw `Model`).
- PTY bytes feed into a `vt.Emulator` instead of a line scanner.
- Each frame, the visible cell region is rendered into the tail area
  via `bubbles/viewport`.

`x/vt` is a full VT terminal emulator (cell buffer, CSI / SGR / cursor
/ screen modes, scrollback, alt-screen detection) built atop
`charmbracelet/ultraviolet`. Staying inside the charmbracelet
dep graph means one vendor's release cadence rather than mixing
`hinshun/vt10x` or rolling our own.

Pane mode is opt-in because (a) the dep cost is non-trivial
(`x/vt` + `ultraviolet`), and (b) most callers in the motivating list
(madder, maneater, cutting-garden, nebulous, clown) emit clean
newline-terminated output and don't need it. Pane mode primarily
benefits future callers that wrap third-party build tools.

[vt]: https://github.com/charmbracelet/x/tree/main/vt

### Cancellation

Ctrl-C in the program triggers `context.CancelFunc` (provided by the
caller, threaded through `Run` / `RunBatch`). The child is killed via
context cancellation; the program waits for `OperationDone` /
`BatchDone` so the final state renders (or the failure dump fires)
before `tea.Quit`. Direct lift of clown tent_loader's pattern.

### TTY autodetect

`Run` / `RunBatch` check `isatty(stdout)` via the existing
`pkgs/primordial.IsTTY()`. On a non-TTY (CI, redirected stdout) they
fall back to streaming the child's combined stdout/stderr to stderr
unchanged — same fallback shape as tent_loader's `loadStreaming`.
Callers do not need to handle this case; the helpers do it
internally.

### Bubbletea version target

This FDR targets **bubbletea v1**
(`github.com/charmbracelet/bubbletea`). v2
(`charm.land/bubbletea/v2`) is still pre-1.0 with a `retract`
directive in its go.mod for `v2.0.0-beta1`. clown uses v1; pinning
dewey to v1 keeps the migration target stable. Revisit when v2 hits
GA.

### Style

Opinionated defaults match clown's existing tent_loader (cyan spinner,
dim adaptive-color tail, green success, red failure) so the visual
language is consistent across tools. `WithStyle(Style{...})` lets
callers override; lipgloss styles are first-class fields on the
`Style` struct.

## Interface

### Single op

```go
import "github.com/amarbel-llc/purse-first/libs/dewey/pkgs/operation_viewport"

err := operation_viewport.Run(ctx, operation_viewport.Op{
    Title: "Loading tent image…",
    Cmd:   exec.CommandContext(ctx, "podman", "load", "-i", tarball),
    Lines: 5, // default
})
```

### Many ops with progress bar

```go
err := operation_viewport.RunBatch(ctx, operation_viewport.Batch{
    Title: "fsck",
    Total: len(blobs),
    Lines: 5,
    Run: func(emit operation_viewport.Emitter) error {
        for i, blob := range blobs {
            emit.OperationStarted(blob.Name, i+1)
            if err := fsckBlob(ctx, blob, emit.LogLine); err != nil {
                emit.OperationDone(err)
                return err
            }
            emit.OperationDone(nil)
        }
        return nil
    },
})
```

`Emitter` is the caller-facing handle on the underlying `tea.Program`;
it exposes `LogLine`, `OperationStarted`, `OperationProgress`,
`OperationDone`. `RunBatch` sends `BatchDone` itself from the wrapped
`Run` func's return value.

### Raw model (custom event source)

```go
m := operation_viewport.NewModel(
    operation_viewport.WithTotal(planTotal),
    operation_viewport.WithLines(5),
)
p := tea.NewProgram(m)

go func() {
    for tapMsg := range tapStream {
        switch tapMsg := tapMsg.(type) {
        case tap.TestPointStarted:
            p.Send(operation_viewport.OperationStarted{...})
        case tap.OutputBlockLine:
            p.Send(operation_viewport.LogLine{Text: tapMsg.Text})
        case tap.TestPointDone:
            p.Send(operation_viewport.OperationDone{Err: tapMsg.Err})
        case tap.PlanComplete:
            p.Send(operation_viewport.BatchDone{Err: tapMsg.Err})
        }
    }
}()

p.Run()
```

### Pane mode (v1)

```go
err := operation_viewport.Run(ctx, operation_viewport.Op{
    Title:    "cargo build",
    Cmd:      exec.CommandContext(ctx, "cargo", "build"),
    Lines:    10,
    PaneMode: true, // route PTY bytes through x/vt instead of line-scanning
})
```

## Examples

### clown (post-migration, v0)

```go
func runTentImageLoad(podmanPath, tarball string) error {
    return operation_viewport.Run(context.Background(), operation_viewport.Op{
        Title: "Loading tent image (first run)…",
        Cmd:   exec.Command(podmanPath, "load", "-i", tarball),
    })
}
```

Replaces ~210 lines of hand-rolled bubbletea wiring in
`clown:cmd/clown/tent_loader.go`.

### madder fsck (v0)

```go
return operation_viewport.RunBatch(ctx, operation_viewport.Batch{
    Title: "fsck",
    Total: blobCount,
    Run: func(emit operation_viewport.Emitter) error {
        for i, blob := range blobs {
            emit.OperationStarted(blob.Name, i+1)
            err := verifyBlob(ctx, blob, func(line string) {
                emit.LogLine(line)
            })
            emit.OperationDone(err)
            if err != nil {
                return err
            }
        }
        return nil
    },
})
```

### maneater indexing with within-op progress (v0)

```go
return operation_viewport.RunBatch(ctx, operation_viewport.Batch{
    Title: "indexing",
    Total: docCount,
    Run: func(emit operation_viewport.Emitter) error {
        for i, doc := range docs {
            emit.OperationStarted(doc.Path, i+1)
            err := indexDoc(ctx, doc, indexCallbacks{
                onLine: emit.LogLine,
                onProgress: func(current, total int) {
                    emit.OperationProgress(current, total)
                },
            })
            emit.OperationDone(err)
            if err != nil {
                return err
            }
        }
        return nil
    },
})
```

## Limitations

- **Sequential ops only in v0.** Multiple operations may not run
  concurrently with overlapping tails. The progress bar tracks
  sequential index. Parallel ops with one tail visible (latest-started)
  or fan-out (multiple tails) is deferred to a follow-up FDR.

- **No second within-op progress bar in the layout.**
  `OperationProgress` is part of the protocol from v0 but the visual
  rendering of within-op progress (a secondary bar between the header
  and the tail) is deferred. Until then, callers can render in-op
  progress textually via `LogLine`.

- **No terminal emulation in v0.** Commands that use `\r`-overwrite
  progress bars, `\x1b[K` line-erase, or cursor positioning render
  imperfectly — typically "last line wins" in the tail with literal
  escape bytes stripped. v1 pane mode (opt-in via `WithPaneMode`)
  resolves this for callers that need it.

- **No mouse / scroll interaction.** The viewport is a passive tail.
  Callers cannot scroll back into history mid-run. Failure dumps go
  to stderr after the program exits; that is the recovery path.

- **Bubbletea v1 only.** v2 (`charm.land/bubbletea/v2`) is pre-GA with
  a retracted beta. dewey will migrate to v2 when it stabilizes; until
  then, callers must use v1 to interop with this primitive.

- **No styling per operation.** Each batch / op uses the configured
  `Style` for its entire lifetime. Callers that want per-op coloring
  (e.g. "this blob is suspicious, render its tail in yellow") must
  inject ANSI into `LogLine` text themselves.

## More Information

- **Reference implementation:** `clown:cmd/clown/tent_loader.go` —
  the hand-rolled spinner + tail this FDR generalizes. Migrating
  clown to `operation_viewport.Run` is a follow-up, not a blocker.
- **Driving caller (TAP integration):** `amarbel-llc/tap` issue #21
  ("TTY viewport for Output Block tail") — separate FDR there owns
  the TAP-aware controller built on top of this primitive.
- **Upstream deps:**
  [`charmbracelet/bubbletea`][bt-v1] (v1),
  [`charmbracelet/bubbles`][bubbles] (spinner, progress, viewport),
  [`charmbracelet/lipgloss`][lipgloss],
  [`charmbracelet/x/xpty`][xpty],
  [`charmbracelet/x/vt`][vt] (v1 pane mode only).
- **Existing dewey precedent:** `pkgs/primordial/is_tty.go` for TTY
  autodetect; `pkgs/ui` for the prior generation of dewey UI
  primitives.

[bt-v1]: https://github.com/charmbracelet/bubbletea
[bubbles]: https://github.com/charmbracelet/bubbles
[lipgloss]: https://github.com/charmbracelet/lipgloss
