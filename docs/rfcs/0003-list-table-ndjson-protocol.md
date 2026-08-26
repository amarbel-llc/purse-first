---
status: accepted
date: 2026-08-26
---

# List-Table NDJSON Protocol

## Abstract

This specification defines an NDJSON stream that describes a tabular listing —
its columns, styled rows, legend, and empty-state — independently of any
rendering. A producer (in any language) emits the stream; a conformant renderer
(the mesa engine, reachable in-process in Go or as the `mesa` CLI)
consumes it and produces a styled table on a TTY or plain tab-separated text on a
pipe. Styling is carried semantically (a fixed severity vocabulary the renderer
colors) rather than as terminal escapes, so layout, width negotiation, and color
live in exactly one place across every consuming tool.

## Introduction

Several `list`-shaped commands across the fleet each hand-roll table rendering in
two languages with divergent conventions and two different `--json` framings (see
[FDR 0015]). This specification is the shared contract that lets them converge: a
producer serializes *what* the rows are, and the renderer owns *how* they look.

Scope: this document specifies the NDJSON record schema, the severity vocabulary,
the optional producer-side markup sugar, and the normative rendering behavior a
conformant renderer MUST exhibit. It does not specify the Go builder API surface
(that is a library concern) nor any command's domain data model.

## Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

## Specification

### 1. Stream framing

A stream is a sequence of newline-delimited JSON records (NDJSON): each line is
one complete JSON object followed by a single U+000A. Producers MUST NOT emit a
JSON array wrapping the records, and MUST NOT split one record across lines.

The **first** record MUST be a *header* record (§2). Every **subsequent** record
MUST be a *row* record (§4). A stream MUST contain exactly one header record. A
stream with a header and zero row records is valid and denotes an empty table.

### 2. Header record

The header record MUST contain a `columns` field and MAY contain the others:

```json
{
  "v": 1,
  "columns": [ /* Column, §3 */ ],
  "legend":  [ /* Legend entry, §6 */ ],
  "empty":   "No sessions.",
  "palette": { "special": "#c678dd" }
}
```

- `v` (integer, OPTIONAL, default `1`) — protocol version. A renderer encountering
  a `v` it does not support MUST fail with a protocol error (§8).
- `columns` (array, REQUIRED) — MUST be non-empty. Defines column count and order.
- `legend` (array, OPTIONAL) — status-key entries rendered as a footer (§6).
- `empty` (string, OPTIONAL) — text rendered when the table has zero rows (§7.4).
- `palette` (object, OPTIONAL) — per-severity color override (§5).

The header record is distinguished from a row record by the presence of
`columns`. A first record lacking `columns` is a protocol error (§8).

### 3. Column object

```json
{ "name": "STATUS", "role": "pin", "align": "left", "shrink": 0, "min": 8 }
```

- `name` (string, REQUIRED) — MUST be non-empty; the column header label.
- `role` (string, REQUIRED) — MUST be `"pin"` or `"flex"`. `pin` columns are sized
  to their content; `flex` columns absorb and shrink to fit the terminal (§7.2).
- `align` (string, OPTIONAL, default `"left"`) — MUST be `"left"` or `"right"`.
- `shrink` (integer ≥ 0, OPTIONAL, default `0`) — shrink priority among `flex`
  columns; lower values shrink first. Ignored for `pin` columns.
- `min` (integer ≥ 0, OPTIONAL) — minimum width a `flex` column shrinks to before
  ellipsizing. When omitted the renderer's default floor applies. Ignored for
  `pin` columns.

### 4. Row record

```json
{ "cells": [ "api", {"spans":[{"text":"●","sev":"ok","b":true},{"text":" attached"}]}, "2m" ] }
```

- `cells` (array, REQUIRED) — its length MUST equal the header `columns` length.
  A row whose length differs is a protocol error (§8).

Each element of `cells` is a **Cell**, which MUST be one of:

- a JSON string — shorthand for a single span of that text at severity `neutral`
  with no attributes; or
- an object `{ "spans": [ /* Span, §4.1 */ ] }` — `spans` MAY be empty (a blank
  cell).

#### 4.1 Span object

```json
{ "text": "●", "sev": "ok", "b": true }
```

- `text` (string, REQUIRED) — the literal display text. Rendered verbatim subject
  to sanitization (§Security); the renderer MUST NOT interpret its content as
  markup or as terminal escapes.
- `sev` (string, OPTIONAL, default `"neutral"`) — a severity name (§5).
- `b` (boolean, OPTIONAL, default `false`) — bold.
- `i`, `u` (boolean, OPTIONAL, default `false`) — italic, underline. Reserved; a
  renderer MAY apply them and MUST NOT error on their presence.

### 5. Severity vocabulary and palette

`sev` MUST be one of the following names. The default color is normative only as a
*role*; the exact rendered color is renderer-defined and MAY adapt to terminal
background:

| Severity  | Role / default |
|-----------|----------------|
| `neutral` | default foreground |
| `muted`   | dim / secondary (markers, hints, borders) |
| `ok`      | success / healthy (green family) |
| `accent`  | informational / secondary-active (cyan family) |
| `warn`    | degraded (yellow family) |
| `error`   | failed / unhealthy (red family) |
| `special` | distinguished (magenta family) — e.g. remote rows |

A renderer encountering an unknown `sev` SHOULD render the span as `neutral` and
MAY emit a diagnostic to stderr; it MUST NOT abort. This lets future severities
degrade gracefully.

The header `palette` object MAY override the color of any severity for the
current stream. Each key MUST be a severity name; each value MUST be either a
`#RRGGBB` hex string or a base-10 string in `"0"`–`"255"` (ANSI 256 index). A
malformed palette value MUST be ignored (the built-in color is used) and MAY be
reported to stderr.

### 6. Legend entry

```json
{ "sev": "ok", "glyph": "●", "label": "attached" }
```

- `sev` (string, REQUIRED) — severity whose color the glyph is drawn in (§5).
- `glyph` (string, REQUIRED) — the marker shown in the legend.
- `label` (string, REQUIRED) — human meaning of the glyph.

### 7. Rendering behavior

#### 7.1 Output mode selection

A renderer MUST select between **styled** and **plain** output by whether its
output stream is a terminal: styled on a TTY, plain otherwise. A renderer MUST
provide a means to force plain output and a means to force styled output,
overriding detection.

#### 7.2 Styled output

On a TTY the renderer MUST:

- draw a border (default: rounded) around the grid;
- render column headers in bold;
- color each span per its severity (§5), applying any `palette` override;
- apply bold to spans with `b: true`;
- render the legend (§6) as a footer beneath the grid when `legend` is present.

Width negotiation SHOULD proceed as: `pin` columns sized to the wcwidth of their
widest cell (header included); remaining terminal width distributed to `flex`
columns; when width is insufficient, `flex` columns shrink in ascending `shrink`
order down to `min`, and content exceeding a column's width is truncated with a
trailing U+2026 (`…`). All width measurement MUST be wcwidth-aware (East-Asian
width and zero-width combining marks) so that composed glyphs and CJK text stay
column-aligned.

#### 7.3 Plain output

On a non-TTY the renderer MUST emit, one line per record, the header row followed
by each data row, where each line is the row's cell texts (the concatenation of
each cell's span `text` values) joined by a single U+0009 (TAB). Plain output MUST
NOT contain ANSI escape sequences, a border, the legend, or width-based wrapping
or truncation. A renderer MAY offer an additional aligned plain mode, but the
TAB-separated form MUST be the default plain output.

#### 7.4 Empty table

When zero row records follow the header, the renderer MUST render the `empty`
string if one is present (styled `muted` on a TTY; verbatim on a pipe) and render
nothing beyond it. An empty table MUST NOT be treated as an error: the process
exit status MUST be `0`.

### 8. Error handling

A **protocol error** is any of: malformed JSON on a line; a first record lacking
`columns`; an empty `columns`; a `v` the renderer does not support; a row whose
`cells` length differs from the column count; a `role` that is not `pin`/`flex`.
On a protocol error the renderer MUST write a diagnostic identifying the offending
record to stderr and MUST exit with a non-zero status. A renderer MUST NOT emit a
partial or misaligned table in response to a protocol error.

### 9. Producer-side markup sugar (informative)

Producers MAY offer a convenience syntax that compiles to the span model, so that
call sites need not spell out span objects. A reference grammar:

```
<SEV [ATTR ...]>text</SEV>      SEV ∈ severity names, ATTR ∈ { b, i, u }
```

Flat (non-nesting); unmarked text compiles to a `neutral` span; a literal `<` is
written `\<` and a literal `\` is written `\\`. For example
`<ok b>●</ok> attached <muted>(current)</muted>` compiles to three spans.

Such sugar is a producer-side authoring aid only. Compiled spans, not raw markup,
MUST be transmitted on the wire (§4.1). Producers MUST resolve malformed markup at
authoring time and MUST NOT transmit it. Producers SHOULD NOT pass untrusted
row data through the markup compiler (tags occurring in the data would be
misinterpreted); untrusted text SHOULD be placed in a span's `text` directly.

## Security Considerations

**Terminal escape injection is the primary risk.** Row content frequently derives
from untrusted or semi-trusted sources (process output, session descriptions,
smart-card CNs, job sources). A renderer MUST treat span `text` as literal display
content and MUST NOT pass embedded control characters through to a terminal:
before styled output, a renderer MUST neutralize C0/C1 control characters and
escape sequences in span text (e.g. by stripping or visibly escaping them) so that
a crafted cell cannot reposition the cursor, alter the terminal state persistently,
or inject its own escape sequences. Plain output likewise MUST NOT emit control
characters from span text other than the structural TAB and newline this protocol
introduces.

**Resource bounds.** Width measurement and rendering of an arbitrarily large
single field could consume unbounded memory or time; a renderer MAY impose a
per-cell length bound and truncate beyond it.

**Palette values** are colors, not code; a renderer MUST validate their format
(§5) and there is no execution risk. **Markup compilation** (§9) runs on
producer-authored strings; the guidance there against feeding untrusted data
through the compiler exists precisely so untrusted text cannot smuggle styling.

## Conformance Testing

Conformance tests for this specification live in
`zz-tests_bats/mesa_table.bats` and exercise the `mesa` CLI, which reads a stream
on stdin and renders to stdout. Run them with `just test-mesa` (part of the `just
test` gate).

The binary under test is injected via the `MESA_BIN` environment variable,
falling back to `build/mesa` (`just build-mesa-cli`), so a re-implementation can
point `MESA_BIN` at its own binary and run the identical suite.

### Covered Requirements

| Requirement | Description |
|-------------|-------------|
| §8, header MUST be first | a first record lacking `columns` exits non-zero |
| §8, columns MUST be non-empty | an empty `columns` list exits non-zero |
| §3, `role` MUST be pin/flex | an invalid role is a protocol error |
| §4, `cells` length MUST equal columns | a row-length mismatch exits non-zero |
| §7.1/§7.3, mode selection + plain form | piped output is TAB-separated with no border and no ANSI |
| §7.1/§7.2, forced style | `--force-style` emits ANSI and a border to a pipe |
| §7.4, empty exit 0 | a header-only stream prints `empty` and exits 0 |
| §5, unknown severity degrades | an unknown `sev` renders without aborting |
| Security, escape sanitization | an embedded ESC in cell text is stripped from output |

## Compatibility

This protocol is the single machine-output framing for the consuming commands,
replacing the divergent `--json` shapes surveyed in [FDR 0015]:

- `clown list` and `piggy list` already emit NDJSON and converge with minimal
  change (aligning field names and adding the header record).
- `ringmaster ls --json` currently emits a single JSON array; migrating it to this
  protocol is a breaking change to that command's `--json` output. Consumers of
  the old array form MUST be updated; the change SHOULD be released with a note.

Future revisions bump the header `v` field (§2). A renderer MUST reject a `v` it
does not implement rather than mis-render (§8), so a version bump fails safely.

## References

### Normative

- [RFC 2119] Key words for use in RFCs to Indicate Requirement Levels.
- [NDJSON] Newline Delimited JSON — https://ndjson.org
- [UAX 11] Unicode Standard Annex #11, East Asian Width (wcwidth semantics).

### Informative

- [FDR 0015] List-table renderer + NDJSON row protocol (dewey) —
  `docs/features/0015-dewey-list-table-renderer.md`.
- [FDR 0010] Operation Viewport — `docs/features/0010-operation-viewport.md`.
