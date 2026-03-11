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
    color: bool,
    locale: Option<Locale>,
    formatter: Option<DecimalFormatter>,
}
```

All fields are private; values are set only through the builder.
`DecimalFormatter` implements `Clone` (ICU 2.x), so `TapConfig` can derive it.

- `color` defaults to `false`
- `locale` and `formatter` are always set together (or both `None`)
- `format_number(&self, n: usize) -> String` moves here from `TapWriter`
- `color(&self) -> bool` accessor for callers that need to read the value
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
- `TapWriterBuilder::auto(w)` — enables all latest amendments via
  environment detection:
  - Color: `true` when `NO_COLOR` env var is absent
  - Locale: reads `LC_ALL` > `LC_NUMERIC` > `LANG` (POSIX precedence),
    parses as ICU locale; `None` if unset or parse fails

Named `auto` rather than `default` to avoid confusion with the `Default`
trait convention.

**Explicit setters** (chainable, last-wins):

- `.color(bool) -> Self`
- `.locale(Locale) -> Self`
- `.no_locale() -> Self` — clears locale (useful after `auto()` to keep
  env-detected color but disable locale formatting)

**Environment-aware setters** (used by `auto()` internally, also public):

- `.default_color() -> Self` — sets color from `NO_COLOR` absence
- `.default_locale() -> Self` — sets locale from `LC_*`/`LANG`

**Terminal methods:**

Both terminal methods create the `DecimalFormatter` (if locale is set)
**before** writing any output. If formatter creation fails, no output has
been written and the error propagates cleanly. All `io::Error` sources
(formatter creation, version line write, pragma write) propagate through
the same `io::Result`.

- `build(self) -> io::Result<TapWriter<'a>>` — creates formatter (if
  locale set), writes `TAP version 14`, emits
  `pragma +locale-formatting:{locale}` if locale is set
- `build_without_printing(self) -> io::Result<TapWriter<'a>>` — creates
  formatter (if locale set), skips version line and pragma output
  (replaces `bare()`)

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
- Methods read `self.config.color()`, call `self.config.format_number(n)`
- `sanitize_yaml_value()` receives `self.config.color()`
- `test_point()` formats `result.number` through
  `self.config.format_number(result.number)` for locale consistency

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
| `TapWriter::bare(&mut buf, true)` | `TapWriterBuilder::new(&mut buf).color(true).build_without_printing()` |

**New tests:**

1. Color + locale combined — ANSI codes and locale-formatted numbers in same
   stream, pragma emitted
2. Color + locale subtest inheritance — child gets both amendments
3. `auto()` constructor — env var detection for color and locale
4. `auto()` with `NO_COLOR=1` — color disabled
5. Explicit overrides after `auto()` — e.g.,
   `TapWriterBuilder::auto(&mut w).color(false).build()`
6. `auto()` with `.no_locale()` — locale cleared after env detection
7. `test_point()` with locale — verify `result.number` is locale-formatted

## Migration

Breaking change. All callers update to builder pattern. No deprecation period.
