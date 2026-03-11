# tap-dancer Rust Builder Pattern Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace TapWriter's constructor proliferation with a builder pattern that composes color and locale amendments independently.

**Architecture:** Extract amendment state into a cloneable `TapConfig` struct. Add `TapWriterBuilder` with `new()` (blank) and `auto()` (env-detected) constructors, chainable setters, and `build()`/`build_without_printing()` terminals. Remove old constructors.

**Tech Stack:** Rust, icu_decimal 2.x, icu_locale_core 2.x, fixed_decimal 0.7

**Spec:** `docs/superpowers/specs/2026-03-11-tap-dancer-rust-builder-pattern-design.md`

---

## Chunk 1: TapConfig struct

### Task 1: Add TapConfig struct and format_number method

**Files:**
- Modify: `packages/tap-dancer/rust/src/lib.rs` (insert before `TestResult` struct at line 31)

- [ ] **Step 1: Write failing tests for TapConfig**

Add to the test module:

```rust
#[test]
fn config_format_number_no_locale() {
    let config = TapConfig {
        color: false,
        locale: None,
        formatter: None,
    };
    assert_eq!(config.format_number(1234), "1234");
}

#[test]
fn config_format_number_with_locale() {
    let locale: Locale = "en-US".parse().unwrap();
    let formatter =
        DecimalFormatter::try_new(locale.clone().into(), Default::default()).unwrap();
    let config = TapConfig {
        color: true,
        locale: Some(locale),
        formatter: Some(formatter),
    };
    assert_eq!(config.format_number(1234), "1,234");
}

#[test]
fn config_color_accessor() {
    let config = TapConfig {
        color: true,
        locale: None,
        formatter: None,
    };
    assert!(config.color());
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `nix develop --command cargo test --package tap-dancer config_ 2>&1 | tail -5`
Expected: compilation error — `TapConfig` not defined

- [ ] **Step 3: Implement TapConfig**

Add after the `use` block (line 5), before `TestResult`:

```rust
#[derive(Clone)]
pub struct TapConfig {
    color: bool,
    locale: Option<Locale>,
    formatter: Option<DecimalFormatter>,
}

impl TapConfig {
    pub fn color(&self) -> bool {
        self.color
    }

    pub fn format_number(&self, n: usize) -> String {
        match &self.formatter {
            Some(fmt) => fmt.format(&Decimal::from(n as i64)).to_string(),
            None => n.to_string(),
        }
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `nix develop --command cargo test --package tap-dancer config_ 2>&1 | tail -10`
Expected: 3 tests pass

- [ ] **Step 5: Run `cargo fmt --package tap-dancer` then commit**

```
feat(tap-dancer): add TapConfig struct with format_number and color accessor
```

## Chunk 2: Builder + internal migration (atomic)

This chunk is atomic: the builder, TapWriter struct change, method migration,
and old-constructor delegation all happen in a single task to avoid compilation
gaps. The old constructors are kept as thin wrappers around the builder so that
existing tests continue to compile.

### Task 2: Add TapWriterBuilder and migrate TapWriter to use TapConfig

**Files:**
- Modify: `packages/tap-dancer/rust/src/lib.rs`
  - TapWriter struct (replace `color`/`locale`/`formatter` with `config`)
  - TapWriter impl (all methods: `self.color` → `self.config.color()`, etc.)
  - Add TapWriterBuilder struct + impl
  - Rewrite old constructors as builder wrappers

This is the largest task. All changes must compile together.

- [ ] **Step 1: Write failing tests for builder construction**

Add to the test module:

```rust
#[test]
fn builder_new_defaults() {
    let mut buf = Vec::new();
    let tw = TapWriterBuilder::new(&mut buf).build().unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(out.contains("TAP version 14\n"));
    assert!(!tw.config.color());
    assert_eq!(tw.count(), 0);
}

#[test]
fn builder_with_color() {
    let mut buf = Vec::new();
    let tw = TapWriterBuilder::new(&mut buf).color(true).build().unwrap();
    assert!(tw.config.color());
}

#[test]
fn builder_with_locale() {
    let mut buf = Vec::new();
    let locale: Locale = "en-US".parse().unwrap();
    let mut tw = TapWriterBuilder::new(&mut buf)
        .locale(locale)
        .build()
        .unwrap();
    tw.plan_ahead(10000).unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(out.contains("pragma +locale-formatting:en-US\n"));
    assert!(out.contains("1..10,000\n"));
}

#[test]
fn builder_with_color_and_locale() {
    let mut buf = Vec::new();
    let locale: Locale = "en-US".parse().unwrap();
    let mut tw = TapWriterBuilder::new(&mut buf)
        .color(true)
        .locale(locale)
        .build()
        .unwrap();
    tw.ok("test").unwrap();
    tw.plan_ahead(10000).unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(out.contains("pragma +locale-formatting:en-US\n"));
    assert!(out.contains("\x1b[32mok\x1b[0m 1 - test\n"));
    assert!(out.contains("1..10,000\n"));
}

#[test]
fn builder_build_without_printing() {
    let mut buf = Vec::new();
    let mut tw = TapWriterBuilder::new(&mut buf)
        .color(true)
        .build_without_printing()
        .unwrap();
    tw.ok("first").unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(!out.contains("TAP version"));
    assert!(out.contains("\x1b[32mok\x1b[0m 1 - first\n"));
}

#[test]
fn builder_build_without_printing_with_locale() {
    let mut buf = Vec::new();
    let locale: Locale = "en-US".parse().unwrap();
    let mut tw = TapWriterBuilder::new(&mut buf)
        .color(true)
        .locale(locale)
        .build_without_printing()
        .unwrap();
    tw.plan_ahead(10000).unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(!out.contains("TAP version"));
    assert!(!out.contains("pragma"));
    assert!(out.contains("1..10,000\n"));
}

#[test]
fn builder_no_locale_clears() {
    let mut buf = Vec::new();
    let locale: Locale = "en-US".parse().unwrap();
    let mut tw = TapWriterBuilder::new(&mut buf)
        .locale(locale)
        .no_locale()
        .build()
        .unwrap();
    tw.plan_ahead(10000).unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(!out.contains("pragma"));
    assert!(out.contains("1..10000\n"));
}
```

- [ ] **Step 2: Verify tests fail (TapWriterBuilder not defined)**

Run: `nix develop --command cargo test --package tap-dancer builder_ 2>&1 | tail -5`
Expected: compilation error

- [ ] **Step 3: Replace TapWriter struct fields with config**

Change the `TapWriter` struct from:

```rust
pub struct TapWriter<'a> {
    w: &'a mut dyn Write,
    counter: usize,
    failed: bool,
    plan_emitted: bool,
    color: bool,
    locale: Option<Locale>,
    formatter: Option<DecimalFormatter>,
}
```

To:

```rust
pub struct TapWriter<'a> {
    w: &'a mut dyn Write,
    counter: usize,
    failed: bool,
    plan_emitted: bool,
    pub(crate) config: TapConfig,
}
```

- [ ] **Step 4: Update all TapWriter method bodies to use config**

Apply these replacements throughout the `impl<'a> TapWriter<'a>` block:

- `self.color` → `self.config.color()` (in `ok`, `not_ok`, `not_ok_diag`, `skip`, `todo`, `bail_out`, `test_point`)
- `self.format_number(...)` → `self.config.format_number(...)` (in `ok`, `not_ok`, `not_ok_diag`, `skip`, `todo`, `plan`, `plan_ahead`)
- Delete `format_number` method from TapWriter (it's now on TapConfig)

Update `test_point()` to locale-format `result.number`. Replace the two
`writeln!` calls that use `result.number`:

```rust
let num = self.config.format_number(result.number);
if let Some(ref directive) = result.directive {
    writeln!(self.w, "{status} {num} - {} # {directive}", result.name)?;
} else {
    writeln!(self.w, "{status} {num} - {}", result.name)?;
}
```

Update `subtest()` to clone config instead of ad-hoc field copying:

```rust
pub fn subtest(
    &mut self,
    name: &str,
    f: impl FnOnce(&mut TapWriter) -> io::Result<()>,
) -> io::Result<()> {
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

- [ ] **Step 5: Rewrite old constructors as builder wrappers**

Replace the existing `new`, `new_color`, `with_locale`, `bare` bodies.
Remove `child()` entirely (subtest now constructs inline).

```rust
pub fn new(w: &'a mut dyn Write) -> io::Result<Self> {
    TapWriterBuilder::new(w).build()
}

pub fn new_color(w: &'a mut dyn Write, color: bool) -> io::Result<Self> {
    TapWriterBuilder::new(w).color(color).build()
}

pub fn with_locale(w: &'a mut dyn Write, locale: Locale) -> io::Result<Self> {
    TapWriterBuilder::new(w).locale(locale).build()
}

pub fn bare(w: &'a mut dyn Write, color: bool) -> Self {
    // Safe: no locale means no formatter creation, no I/O in build_without_printing
    TapWriterBuilder::new(w)
        .color(color)
        .build_without_printing()
        .unwrap()
}
```

- [ ] **Step 6: Add TapWriterBuilder struct and impl**

Insert after `TapConfig` impl, before `TestResult`:

```rust
pub struct TapWriterBuilder<'a> {
    w: &'a mut dyn Write,
    color: bool,
    locale: Option<Locale>,
}

impl<'a> TapWriterBuilder<'a> {
    pub fn new(w: &'a mut dyn Write) -> Self {
        Self {
            w,
            color: false,
            locale: None,
        }
    }

    pub fn auto(w: &'a mut dyn Write) -> Self {
        Self::new(w).default_color().default_locale()
    }

    pub fn color(mut self, color: bool) -> Self {
        self.color = color;
        self
    }

    pub fn locale(mut self, locale: Locale) -> Self {
        self.locale = Some(locale);
        self
    }

    pub fn no_locale(mut self) -> Self {
        self.locale = None;
        self
    }

    pub fn default_color(mut self) -> Self {
        self.color = std::env::var("NO_COLOR").is_err();
        self
    }

    pub fn default_locale(mut self) -> Self {
        let locale_str = std::env::var("LC_ALL")
            .or_else(|_| std::env::var("LC_NUMERIC"))
            .or_else(|_| std::env::var("LANG"))
            .ok();
        if let Some(s) = locale_str {
            // Strip .UTF-8 or other encoding suffixes for ICU parsing
            let base = s.split('.').next().unwrap_or(&s);
            if let Ok(locale) = base.parse::<Locale>() {
                self.locale = Some(locale);
            }
        }
        self
    }

    fn build_config(&self) -> io::Result<TapConfig> {
        let (locale, formatter) = match &self.locale {
            Some(locale) => {
                let formatter =
                    DecimalFormatter::try_new(locale.clone().into(), Default::default())
                        .map_err(|e| {
                            io::Error::new(io::ErrorKind::Other, e.to_string())
                        })?;
                (Some(locale.clone()), Some(formatter))
            }
            None => (None, None),
        };
        Ok(TapConfig {
            color: self.color,
            locale,
            formatter,
        })
    }

    pub fn build(self) -> io::Result<TapWriter<'a>> {
        // Create formatter before any I/O to avoid partial output on error
        let config = self.build_config()?;
        writeln!(self.w, "TAP version 14")?;
        if let Some(ref locale) = config.locale {
            writeln!(self.w, "pragma +locale-formatting:{locale}")?;
        }
        Ok(TapWriter {
            w: self.w,
            counter: 0,
            failed: false,
            plan_emitted: false,
            config,
        })
    }

    pub fn build_without_printing(self) -> io::Result<TapWriter<'a>> {
        let config = self.build_config()?;
        Ok(TapWriter {
            w: self.w,
            counter: 0,
            failed: false,
            plan_emitted: false,
            config,
        })
    }
}
```

- [ ] **Step 7: Run all tests**

Run: `nix develop --command cargo test --package tap-dancer 2>&1 | tail -15`
Expected: All existing tests pass + 7 new builder tests pass

- [ ] **Step 8: Run `cargo fmt --package tap-dancer` then commit**

```
feat(tap-dancer): add TapWriterBuilder and migrate TapWriter to TapConfig

Replace color/locale/formatter fields on TapWriter with a cloneable
TapConfig struct. Add TapWriterBuilder with new()/auto() constructors,
chainable setters, and build()/build_without_printing() terminals.
Old constructors kept as thin builder wrappers pending removal.
```

## Chunk 3: Remove old constructors + env detection tests

### Task 3: Remove old constructors and migrate all tests to builder

**Files:**
- Modify: `packages/tap-dancer/rust/src/lib.rs` (TapWriter impl + test module)

- [ ] **Step 1: Delete old constructors from TapWriter impl**

Remove these methods: `new()`, `new_color()`, `with_locale()`, `bare()`.

- [ ] **Step 2: Update all 37 test call sites to use builder**

Mechanical replacements:

| Old | New |
|-----|-----|
| `TapWriter::new(&mut buf).unwrap()` | `TapWriterBuilder::new(&mut buf).build().unwrap()` |
| `TapWriter::new_color(&mut buf, true).unwrap()` | `TapWriterBuilder::new(&mut buf).color(true).build().unwrap()` |
| `TapWriter::with_locale(&mut buf, locale).unwrap()` | `TapWriterBuilder::new(&mut buf).locale(locale).build().unwrap()` |
| `TapWriter::bare(&mut buf, false)` | `TapWriterBuilder::new(&mut buf).build_without_printing().unwrap()` |

Note: there are no existing `bare(_, true)` call sites, but the builder
supports it via `.color(true).build_without_printing()`.

- [ ] **Step 3: Run all tests**

Run: `nix develop --command cargo test --package tap-dancer 2>&1 | tail -15`
Expected: All tests pass

- [ ] **Step 4: Run `cargo fmt --package tap-dancer` then commit**

```
refactor(tap-dancer): remove old constructors, builder is sole API
```

### Task 4: Add auto() environment detection tests

**Files:**
- Modify: `packages/tap-dancer/rust/src/lib.rs` (test module)

- [ ] **Step 1: Add auto() tests**

These tests manipulate env vars and must run single-threaded.

```rust
#[test]
fn builder_auto_no_color_when_set() {
    let original = std::env::var("NO_COLOR").ok();
    std::env::set_var("NO_COLOR", "1");

    let mut buf = Vec::new();
    let tw = TapWriterBuilder::auto(&mut buf).build().unwrap();
    assert!(!tw.config.color());

    match original {
        Some(v) => std::env::set_var("NO_COLOR", v),
        None => std::env::remove_var("NO_COLOR"),
    }
}

#[test]
fn builder_auto_color_when_no_color_absent() {
    let original = std::env::var("NO_COLOR").ok();
    std::env::remove_var("NO_COLOR");

    let mut buf = Vec::new();
    let tw = TapWriterBuilder::auto(&mut buf).build().unwrap();
    assert!(tw.config.color());

    if let Some(v) = original {
        std::env::set_var("NO_COLOR", v);
    }
}

#[test]
fn builder_auto_override_color() {
    let original = std::env::var("NO_COLOR").ok();
    std::env::remove_var("NO_COLOR");

    let mut buf = Vec::new();
    let tw = TapWriterBuilder::auto(&mut buf).color(false).build().unwrap();
    assert!(!tw.config.color());

    if let Some(v) = original {
        std::env::set_var("NO_COLOR", v);
    }
}

#[test]
fn builder_default_locale_ignores_c_locale() {
    let orig_all = std::env::var("LC_ALL").ok();
    let orig_num = std::env::var("LC_NUMERIC").ok();
    let orig_lang = std::env::var("LANG").ok();
    std::env::set_var("LANG", "C");
    std::env::remove_var("LC_ALL");
    std::env::remove_var("LC_NUMERIC");

    let mut buf = Vec::new();
    let mut tw = TapWriterBuilder::new(&mut buf)
        .default_locale()
        .build()
        .unwrap();
    tw.plan_ahead(10000).unwrap();
    let out = String::from_utf8(buf).unwrap();
    // C locale should not parse as ICU locale, so no formatting
    assert!(!out.contains("pragma"));
    assert!(out.contains("1..10000\n"));

    // Restore
    match orig_all {
        Some(v) => std::env::set_var("LC_ALL", v),
        None => std::env::remove_var("LC_ALL"),
    }
    match orig_num {
        Some(v) => std::env::set_var("LC_NUMERIC", v),
        None => std::env::remove_var("LC_NUMERIC"),
    }
    match orig_lang {
        Some(v) => std::env::set_var("LANG", v),
        None => std::env::remove_var("LANG"),
    }
}
```

- [ ] **Step 2: Run auto tests single-threaded**

Run: `nix develop --command cargo test --package tap-dancer builder_auto builder_default_locale -- --test-threads=1 2>&1 | tail -10`
Expected: 4 tests pass

- [ ] **Step 3: Run `cargo fmt --package tap-dancer` then commit**

```
test(tap-dancer): add auto() environment detection and C locale tests
```

## Chunk 4: Combined amendment tests + verification

### Task 5: Add color + locale combined tests

**Files:**
- Modify: `packages/tap-dancer/rust/src/lib.rs` (test module)

- [ ] **Step 1: Write combined amendment tests**

```rust
#[test]
fn writer_color_and_locale_combined() {
    let mut buf = Vec::new();
    let locale: Locale = "en-US".parse().unwrap();
    let mut tw = TapWriterBuilder::new(&mut buf)
        .color(true)
        .locale(locale)
        .build()
        .unwrap();
    for _ in 0..1234 {
        tw.ok("test").unwrap();
    }
    tw.plan().unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(out.starts_with("TAP version 14\n"));
    assert!(out.contains("pragma +locale-formatting:en-US\n"));
    assert!(out.contains("\x1b[32mok\x1b[0m 1,234 - test\n"));
    assert!(out.contains("1..1,234\n"));
}

#[test]
fn writer_color_and_locale_subtest_inheritance() {
    let mut buf = Vec::new();
    let locale: Locale = "en-US".parse().unwrap();
    let mut tw = TapWriterBuilder::new(&mut buf)
        .color(true)
        .locale(locale)
        .build()
        .unwrap();
    tw.subtest("nested", |sub| {
        sub.plan_ahead(10000)?;
        sub.ok("inner")?;
        sub.plan()
    })
    .unwrap();
    tw.ok("nested").unwrap();
    tw.plan().unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(
        out.contains("    pragma +locale-formatting:en-US\n"),
        "expected subtest locale pragma, got:\n{out}"
    );
    assert!(
        out.contains("    \x1b[32mok\x1b[0m 1 - inner\n"),
        "expected subtest color, got:\n{out}"
    );
    assert!(
        out.contains("    1..10,000\n"),
        "expected subtest locale plan, got:\n{out}"
    );
    assert!(out.contains("\x1b[32mok\x1b[0m 1 - nested\n"));
}

#[test]
fn writer_test_point_formats_number_with_locale() {
    let mut buf = Vec::new();
    let locale: Locale = "en-US".parse().unwrap();
    let mut tw = TapWriterBuilder::new(&mut buf)
        .locale(locale)
        .build()
        .unwrap();
    let result = TestResult {
        number: 1234,
        name: "big number".into(),
        ok: true,
        directive: None,
        error_message: None,
        exit_code: None,
        output: None,
        suppress_yaml: false,
    };
    tw.test_point(&result).unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(
        out.contains("ok 1,234 - big number\n"),
        "expected locale-formatted number, got:\n{out}"
    );
}
```

- [ ] **Step 2: Run tests**

Run: `nix develop --command cargo test --package tap-dancer writer_color_and_locale writer_test_point_formats 2>&1 | tail -10`
Expected: 3 tests pass

- [ ] **Step 3: Run `cargo fmt --package tap-dancer` then commit**

```
test(tap-dancer): add color+locale combined and test_point locale tests
```

### Task 6: Final verification

**Files:**
- None (verification only)

- [ ] **Step 1: Run all Rust tests**

Run: `nix develop --command cargo test --package tap-dancer 2>&1`
Expected: All tests pass (72 original + ~14 new = ~86 total)

- [ ] **Step 2: Run clippy**

Run: `nix develop --command cargo clippy --package tap-dancer -- -D warnings 2>&1 | tail -10`
Expected: No warnings

- [ ] **Step 3: Run fmt check**

Run: `nix develop --command cargo fmt --package tap-dancer -- --check 2>&1`
Expected: No formatting issues

- [ ] **Step 4: Build the nix package**

Run: `nix build .#tap-dancer 2>&1 | tail -5`
Expected: Build succeeds

### Task 7: Update TODO.md

**Files:**
- Modify: `TODO.md`

- [ ] **Step 1: Mark completed items**

Check off:
- `- [x] update tap-dancer with latest tap amendments`
- `- [x] tap-dancer rust: add combined color+locale constructor`

- [ ] **Step 2: Commit**

```
docs(TODO): mark tap-dancer rust builder pattern items complete
```
