# tap-dancer Rust: Builder Pattern for Amendment Composition

**Date:** 2026-03-11
**Scope:** `packages/tap-dancer/rust/src/lib.rs`

## Problem

`TapWriter` has 4 constructors (`new`, `new_color`, `with_locale`, `bare`).
`with_locale` hardcodes `color: false`, making color and locale mutually
exclusive. Adding a combined constructor would be the 5th, and future
amendments would cause combinatorial explosion.

## Solution

Replace all constructors with a `TapWriterBuilder` + `TapConfig` struct.
Amendments compose independently via builder chaining.

## Design

### TapConfig

```rust
#[derive(Clone)]
pub struct TapConfig {
    pub color: bool,
    locale: Option<Locale>,
    formatter: Option<DecimalFormatter>,
}
```

- `color` defaults to `false`
- `locale` and `formatter` are always set together (or both `None`)
- `format_number(&self, n: usize) -> String` moves here from `TapWriter`
- Children clone the parent's config — replaces ad-hoc field copying in
  `subtest()`

### TapWriterBuilder

```rust
pub struct TapWriterBuilder<'a> {
    w: &'a mut dyn Write,
    color: bool,
    locale: Option<Locale>,
}
```

**Two constructors:**

- `TapWriterBuilder::new(w)` — blank slate, all amendments off
- `TapWriterBuilder::default(w)` — enables all latest amendments via
  environment detection:
  - Color: `true` when `NO_COLOR` env var is absent
  - Locale: reads `LC_ALL` > `LC_NUMERIC` > `LANG` (POSIX precedence),
    parses as ICU locale; `None` if unset or parse fails

**Explicit setters** (chainable, last-wins):

- `.color(bool) -> Self`
- `.locale(Locale) -> Self`

**Environment-aware setters** (used by `default()` internally, also public):

- `.default_color() -> Self` — sets color from `NO_COLOR` absence
- `.default_locale() -> Self` — sets locale from `LC_*`/`LANG`

**Terminal methods:**

- `build(self) -> io::Result<TapWriter<'a>>` — writes `TAP version 14`,
  emits `pragma +locale-formatting:{locale}` if locale is set, creates
  `DecimalFormatter` from locale (returns `Err` on failure)
- `build_without_printing(self) -> io::Result<TapWriter<'a>>` — same
  initialization, skips version line and pragma output (replaces `bare()`)

### TapWriter

```rust
pub struct TapWriter<'a> {
    w: &'a mut dyn Write,
    counter: usize,
    failed: bool,
    plan_emitted: bool,
    config: TapConfig,
}
```

- `color`, `locale`, `formatter` fields replaced by `config: TapConfig`
- Methods read `self.config.color`, call `self.config.format_number(n)`
- `sanitize_yaml_value()` receives `self.config.color`

**Removed:** `new()`, `new_color()`, `with_locale()`, `bare()`, `child()`

**Subtest** clones parent config and emits locale pragma if set:

```rust
pub fn subtest(&mut self, name: &str, f: impl FnOnce(&mut TapWriter) -> io::Result<()>) -> io::Result<()> {
    writeln!(self.w, "    # Subtest: {}", name)?;
    let mut indent = IndentWriter { w: &mut *self.w };
    let config = self.config.clone();
    let mut child = TapWriter {
        w: &mut indent,
        counter: 0,
        failed: false,
        plan_emitted: false,
        config,
    };
    if let Some(ref locale) = child.config.locale {
        writeln!(child.w, "pragma +locale-formatting:{locale}")?;
    }
    f(&mut child)
}
```

### Free functions

Unchanged — `write_version()`, `write_plan()`, etc. remain as-is.

## Test Strategy

**Existing tests (~88):** Mechanical update of constructor calls:

| Old | New |
|-----|-----|
| `TapWriter::new(&mut buf)` | `TapWriterBuilder::new(&mut buf).build()` |
| `TapWriter::new_color(&mut buf, true)` | `TapWriterBuilder::new(&mut buf).color(true).build()` |
| `TapWriter::with_locale(&mut buf, locale)` | `TapWriterBuilder::new(&mut buf).locale(locale).build()` |
| `TapWriter::bare(&mut buf, false)` | `TapWriterBuilder::new(&mut buf).build_without_printing()` |

**New tests:**

1. Color + locale combined — ANSI codes and locale-formatted numbers in same
   stream, pragma emitted
2. Color + locale subtest inheritance — child gets both amendments
3. `default()` constructor — env var detection for color and locale
4. `default()` with `NO_COLOR=1` — color disabled
5. Explicit overrides after `default()` — e.g.,
   `TapWriterBuilder::default(&mut w).color(false).build()`

## Migration

Breaking change. All callers update to builder pattern. No deprecation period.
