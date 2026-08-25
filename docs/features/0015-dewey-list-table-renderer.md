---
status: proposed
date: 2026-08-25
promotion-criteria: one Go consumer (e.g. `clown list` or `ringmaster ls`) and one Rust consumer (e.g. `posh list` or `piggy list`) both render through the dewey table renderer over the shared row protocol, replacing their in-tree table code; NDJSON is the single `--json` framing across the Go and Rust consumers; `sc list`'s rich surface (composite status cell, one flex column, legend footer) is reproduced by the renderer with no `sc`-local table code remaining.
---

# List-table renderer + NDJSON row protocol (dewey)

## Problem Statement

At least six `list`-shaped commands across the fleet each hand-roll table
rendering, in two languages, with divergent conventions: `clown list` and
`ringmaster ls` are monochrome `text/tabwriter`; `sc list` is a rich Charmbracelet
`lipgloss/table` with colored status glyphs, a legend footer, and a bubbletea
`--watch` mode; `posh list`, `piggy list`, and the piggy-ids enumerators are
hand-rolled ANSI + box-drawing in Rust. They disagree even on machine output —
`clown list --json` emits NDJSON while `ringmaster ls --json` emits a single JSON
array, despite `clown list` rendering a *ringmaster* type (`jobwake.Presence`).
We want one table renderer, in dewey, that Go consumers call in-process and Rust
consumers feed out-of-process over a simple NDJSON stream, so column layout,
width negotiation, styling, and empty-state handling live in exactly one place.

## Background / survey

This is a consolidation of renderers that already exist, not a greenfield design.
As surveyed 2026-08-25:

| Command | Lang | Current renderer | Cols | Styling | TTY switch | Machine output |
|---|---|---|---|---|---|---|
| `clown list` | Go | `text/tabwriter` | 4 | none | no | `--json` → NDJSON (`jobwake.Presence`) |
| `ringmaster ls` | Go | `text/tabwriter` | 5–6 | none | no | `--json` → JSON array (`[]JobSummary`) |
| `sc list` | Go | `lipgloss/table` + bubbletea | 4 | full ANSI, legend, `--watch` alt-screen | yes (`go-isatty`) | `--format json` → array of `session.ListRow` |
| `posh list` | Rust | hand-rolled ANSI + box-drawing, wcwidth-aware, responsive shrink | 7 | full ANSI, legend | yes (`libc::isatty`) | `--json` (hand-built), `--short` |
| `piggy list` | Rust | hand-rolled `format!` (comment-style lines, not gridded) | n/a | none | yes (`is_terminal()`) | `--format human\|ndjson\|ssh` |
| `piggy pass list` | Rust | external `tree(1)` / custom tree renderer | — | tree | n/a | none |

Two consumers already do the right thing on the wire (`clown list`, `piggy list`
emit NDJSON) and can converge with minimal churn; the rest each re-invent layout.
`piggy pass list` is out of scope — it is an alias for `piggy pass show` and
renders a *filesystem tree*, not a row table (see Limitations).

Three near-duplicates would be worse than one; five is the current reality. This
FDR scopes a dewey-side row-table primitive, a direct sibling of the
[Operation Viewport](0010-operation-viewport.md) progress primitive: the same
"promote the shared Charmbracelet layer into dewey, give existing callers a
migration target" move, applied to tabular listing instead of progress.

## Interface

### Two entry points, one renderer

- **Go consumers** call the renderer in-process: build a `Table` value
  (column specs + rows carrying semantic cells) and hand it to a render function
  that returns a styled string for a TTY and a plain form for a pipe.
- **Rust consumers** stay style-free: they emit **NDJSON** on stdout — one header
  record describing the columns, then one record per row — and an out-of-process
  dewey formatter reads that stream and renders the table. Width negotiation and
  styling move entirely out of the Rust codebase; the Rust side owns only *what*
  the rows are, never *how* they look.

### The row protocol carries semantics, not ANSI

The wire/data contract must carry enough for the renderer to reproduce every
surveyed table without the emitter styling anything:

- **Column specs** — name, alignment, and a role flag: `pin-to-content` vs
  `flex`, plus a shrink-priority ordering among flex columns (posh shrinks
  ACTIVITY → ECHO → STARTED IN).
- **Semantic cell styling** — a *state token* (e.g. `active` / `detached` /
  `stale`), never an ANSI code. The renderer owns the token → color mapping, so
  Rust emitters and plain-tabwriter Go emitters stay colorless.
- **Composite status cell** — glyph + count + optional marker (`(current)`,
  `main`, `tombstone`). `sc list` and `posh list` both fold multiple signals into
  one status column, so a cell must be more than a plain string.
- **Footer / legend** — an optional status-key legend row (both `sc` and `posh`
  render one).
- **Empty-state string** — supplied per table; the surveyed commands all differ
  ("No sessions." / "no jobs" / "no sessions found in {dir}" / silent).

### Width and glyph correctness

The renderer measures widths **wcwidth-aware** (posh already depends on this;
naive byte or rune length misaligns CJK/emoji), pins narrow columns to content
width, and lets flex column(s) absorb remaining terminal width, shrinking by
declared priority down to a per-column minimum.

### TTY switching and machine output

The renderer detects a TTY and renders the styled table for a terminal, a plain
tab-separated form for a pipe. **NDJSON is the single machine-output framing**
for `--json` across every consumer — replacing `ringmaster ls`'s array framing
and matching `clown list` / `piggy list`, so downstream tools parse one shape.

## Examples

Go consumer, in-process:

    tbl := deweytable.New().
        Col("ID", deweytable.Flex).
        Col("STATUS", deweytable.PinContent).
        Col("AGE", deweytable.PinContent).
        Col("DESCRIPTION", deweytable.Flex).
        Empty("No sessions.")
    for _, s := range sessions {
        tbl.Row(deweytable.Cells{
            "ID":     deweytable.Text(s.Key),
            "STATUS": deweytable.Status(s.StateToken, s.ClownCount),
            "AGE":    deweytable.Text(s.Age),
        })
    }
    fmt.Fprint(os.Stdout, tbl.Render())   // styled on TTY, plain on pipe

Rust consumer, out-of-process — emit NDJSON, pipe to the dewey formatter:

    # header record, then one row record per session
    posh list --format ndjson | dewey table

    {"cols":[{"name":"NAME","role":"flex"},{"name":"STATUS","role":"pin"}]}
    {"NAME":"api","STATUS":{"state":"attached","marker":"(current)"}}
    {"NAME":"web","STATUS":{"state":"stale"}}

The same NDJSON stream is what `posh list --json` returns directly to a caller
that wants data rather than a table.

## Limitations

- **`piggy pass list` is excluded.** It is an alias for `piggy pass show` and
  renders a filesystem tree of the password store (via `tree(1)` or a custom
  Rust tree renderer), not a row/column table. Tree rendering is a separate
  concern; forcing it through a tabular row protocol would be a mis-fit.
- **`--watch` ownership is unresolved.** `sc list`'s bubbletea alt-screen loop is
  the one surveyed behavior that is not a stateless NDJSON → table transform. If
  the renderer stays one-shot, `sc` keeps its own watch harness and calls the
  renderer per frame; if the renderer owns watch, the NDJSON source must become
  repeatable or streaming. This decision changes whether "Rust emits NDJSON once
  to a formatter" is sufficient or the formatter must be long-lived, so it is
  called out rather than assumed.
- **Rust row structs are not yet serializable.** `posh`'s row data and piggy-ids'
  `Classification` build their JSON by hand today; the NDJSON path presumes real
  `serde::Serialize` row types feeding the stream, which is prerequisite work on
  each Rust consumer.

## Tuning Levers

| Lever | Current | Rationale | Change signal |
|---|---|---|---|
| min flex-column width | 8 chars (posh's existing floor) | keeps a shrunk column legible rather than a stub | a consumer's flex content is routinely unreadable at 8 |
| non-TTY default output | plain tab-separated | pipe-friendly, greppable, matches tabwriter consumers today | consumers overwhelmingly pass `--json` when piping, making NDJSON the better default |
| watch refresh interval | 2s (`sc list`'s current default) | responsive without hammering the data source | watch feels laggy or too busy in practice |

## More Information

- [FDR 0010 — Operation Viewport](0010-operation-viewport.md): the sibling
  dewey-side Charmbracelet primitive; same consolidation pattern for progress
  UX. This FDR is the tabular-listing counterpart.
- [FDR 0012 — Declarative command framework (futility → dewey)](0012-declarative-command-framework.md):
  once a command's `Run` signature is the authoritative declaration, column
  specs for its list output are a candidate to derive from the same source
  rather than hand-declaring them a second time.
