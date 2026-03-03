# Locale Number Formatting Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add locale-aware number formatting to tap-dancer's Go and Rust writer libraries and the Go reader/parser, per the TAP-14 locale-formatting amendment.

**Architecture:** Writers gain an optional locale setting. When set, they emit `pragma +locale-formatting:<tag>` and format test point IDs / plan counts using locale-specific grouping separators (`golang.org/x/text/message` for Go, `icu_decimal` for Rust). The Go reader recognizes the pragma and strips grouping separators before numeric comparison. Child writers (subtests) inherit the parent's locale and auto-emit their own pragma.

**Tech Stack:** Go (`golang.org/x/text/message`, `golang.org/x/text/language`), Rust (`icu_decimal`, `icu_locale_core`, `fixed_decimal`)

**Rollback:** N/A — purely additive. Writers default to no locale (current behavior). Reader is backwards-compatible without pragma.

---

### Task 1: Add `golang.org/x/text` dependency to tap-dancer Go module

**Promotion criteria:** N/A

**Files:**
- Modify: `packages/tap-dancer/go/go.mod`

**Step 1: Add the dependency**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/snug-spruce && nix develop --command bash -c "cd packages/tap-dancer/go && go get golang.org/x/text/message golang.org/x/text/language"`

**Step 2: Sync workspace and vendor**

Run: `just deps`

**Step 3: Verify it compiles**

Run: `nix develop --command go build ./packages/tap-dancer/...`

**Step 4: Commit**

```bash
git add packages/tap-dancer/go/go.mod packages/tap-dancer/go/go.sum go.work.sum vendor/
git commit -m "feat(tap-dancer): add golang.org/x/text dependency for locale formatting"
```

---

### Task 2: Go writer — locale-formatted number output

**Promotion criteria:** N/A

**Files:**
- Modify: `packages/tap-dancer/go/tap.go`
- Test: `packages/tap-dancer/go/tap_test.go`

**Step 1: Write failing tests**

Add to `packages/tap-dancer/go/tap_test.go`:

```go
func TestLocaleWriterEmitsPragma(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	tw.Ok("first")
	tw.Plan()
	out := buf.String()
	if !strings.Contains(out, "pragma +locale-formatting:en-US\n") {
		t.Errorf("expected locale pragma, got:\n%s", out)
	}
}

func TestLocaleWriterFormatsTestPointNumber(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	for i := 0; i < 1234; i++ {
		tw.Ok("test")
	}
	out := buf.String()
	if !strings.Contains(out, "ok 1,234 - test\n") {
		t.Errorf("expected locale-formatted number ok 1,234, got last lines:\n%s",
			out[max(0, len(out)-200):])
	}
}

func TestLocaleWriterFormatsPlanCount(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	tw.PlanAhead(10000)
	out := buf.String()
	if !strings.Contains(out, "1..10,000\n") {
		t.Errorf("expected locale-formatted plan 1..10,000, got:\n%s", out)
	}
}

func TestLocaleWriterGermanSeparator(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("de-DE"))
	tw.PlanAhead(10000)
	out := buf.String()
	if !strings.Contains(out, "1..10.000\n") {
		t.Errorf("expected German-formatted plan 1..10.000, got:\n%s", out)
	}
}

func TestLocaleWriterSmallNumbersUnformatted(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	tw.Ok("test")
	out := buf.String()
	// Numbers < 1000 should not have separators
	if !strings.Contains(out, "ok 1 - test\n") {
		t.Errorf("expected plain number for small values, got:\n%s", out)
	}
}

func TestLocaleWriterSubtestInheritsLocale(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	sub := tw.Subtest("nested")
	sub.PlanAhead(10000)
	sub.Plan()
	tw.Ok("nested")
	tw.Plan()
	out := buf.String()
	if !strings.Contains(out, "    pragma +locale-formatting:en-US\n") {
		t.Errorf("expected subtest to inherit and emit locale pragma, got:\n%s", out)
	}
	if !strings.Contains(out, "    1..10,000\n") {
		t.Errorf("expected subtest to use locale formatting, got:\n%s", out)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test -run 'TestLocaleWriter' -v ./packages/tap-dancer/go/...`
Expected: FAIL — `NewLocaleWriter` undefined

**Step 3: Implement locale support in Writer**

Modify `packages/tap-dancer/go/tap.go`:

1. Add imports: `"golang.org/x/text/language"` and `"golang.org/x/text/message"`

2. Add `locale` and `printer` fields to `Writer`:
```go
type Writer struct {
	w           io.Writer
	n           int
	depth       int
	planEmitted bool
	failed      bool
	color       bool
	locale      language.Tag
	printer     *message.Printer
}
```

3. Add `NewLocaleWriter` constructor:
```go
func NewLocaleWriter(w io.Writer, locale language.Tag) *Writer {
	fmt.Fprintln(w, "TAP version 14")
	p := message.NewPrinter(locale)
	fmt.Fprintf(w, "pragma +locale-formatting:%s\n", locale)
	return &Writer{w: w, locale: locale, printer: p}
}
```

4. Add `formatNumber` helper method:
```go
func (tw *Writer) formatNumber(n int) string {
	if tw.printer == nil {
		return fmt.Sprintf("%d", n)
	}
	return tw.printer.Sprintf("%d", n)
}
```

5. Replace all `%d` formatting of `tw.n` and plan counts to use `tw.formatNumber()`:
   - `Ok`: `fmt.Fprintf(tw.w, "%s %s - %s\n", tw.colorOk(), tw.formatNumber(tw.n), description)`
   - `OkDiag`: same pattern
   - `NotOk`: same pattern
   - `Skip`: same pattern
   - `SkipDiag`: same pattern
   - `Todo`: same pattern
   - `PlanAhead`: `fmt.Fprintf(tw.w, "1..%s\n", tw.formatNumber(n))`
   - `Plan`: `fmt.Fprintf(tw.w, "1..%s\n", tw.formatNumber(tw.n))`
   - `WriteAll` ok/not-ok branches: same pattern

6. Update `Subtest` to inherit locale:
```go
func (tw *Writer) Subtest(name string) *Writer {
	prefix := "    "
	fmt.Fprintf(tw.w, "%s# Subtest: %s\n", prefix, name)
	iw := &indentWriter{w: tw.w, prefix: prefix}
	child := &Writer{w: iw, depth: tw.depth + 1, color: tw.color, locale: tw.locale, printer: tw.printer}
	if tw.printer != nil {
		fmt.Fprintf(iw, "pragma +locale-formatting:%s\n", tw.locale)
	}
	return child
}
```

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test -run 'TestLocaleWriter' -v ./packages/tap-dancer/go/...`
Expected: PASS

**Step 5: Run all existing tests to verify no regressions**

Run: `nix develop --command go test -v ./packages/tap-dancer/go/...`
Expected: All PASS

**Step 6: Commit**

```bash
git add packages/tap-dancer/go/tap.go packages/tap-dancer/go/tap_test.go
git commit -m "feat(tap-dancer): add locale-formatted number output to Go writer"
```

---

### Task 3: Go reader — locale-aware parsing

**Promotion criteria:** N/A

**Files:**
- Modify: `packages/tap-dancer/go/reader.go`
- Modify: `packages/tap-dancer/go/classify.go`
- Modify: `packages/tap-dancer/go/parse.go`
- Modify: `packages/tap-dancer/go/diagnostic.go`
- Test: `packages/tap-dancer/go/reader_test.go`

**Step 1: Write failing tests**

Add to `packages/tap-dancer/go/reader_test.go`:

```go
func TestReaderLocaleFormattedPlan(t *testing.T) {
	input := "TAP version 14\npragma +locale-formatting:en-US\n1..1,200\nok 1 - first\nok 2 - second\n"
	_, diags, summary := collectEvents(input)

	// Plan should parse as 1200 despite comma
	if summary.PlanCount != 1200 {
		t.Errorf("expected plan count 1200, got %d", summary.PlanCount)
	}
	// Plan-count-mismatch expected since we only have 2 tests
	_ = diags
}

func TestReaderLocaleFormattedTestPoint(t *testing.T) {
	input := "TAP version 14\npragma +locale-formatting:en-US\n1..2\nok 1 - first\nok 1,234 - big\n"
	events, _, _ := collectEvents(input)

	for _, ev := range events {
		if ev.Type == EventTestPoint && ev.TestPoint.Number == 1234 {
			return // found it
		}
	}
	t.Error("expected test point with number 1234 parsed from '1,234'")
}

func TestReaderLocaleGermanPlan(t *testing.T) {
	input := "TAP version 14\npragma +locale-formatting:de-DE\n1..1.200\nok 1 - test\n"
	_, _, summary := collectEvents(input)

	if summary.PlanCount != 1200 {
		t.Errorf("expected plan count 1200 from German format, got %d", summary.PlanCount)
	}
}

func TestReaderLocaleFormattingSubtestScoping(t *testing.T) {
	input := "TAP version 14\npragma +locale-formatting:en-US\n1..1\n" +
		"    # Subtest: child\n    1..1\n    ok 1 - inner\n" +
		"ok 1 - child\n"
	_, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true")
	}
}

func TestReaderNoLocaleRejectsFormattedNumbers(t *testing.T) {
	// Without locale pragma, comma in plan should fail to parse
	input := "TAP version 14\n1..1,200\nok 1 - test\n"
	_, _, summary := collectEvents(input)

	// Without locale, "1,200" should not parse as a valid plan count
	if summary.PlanCount == 1200 {
		t.Error("expected plan NOT to parse as 1200 without locale pragma")
	}
}

func TestReaderLocaleRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	for i := 0; i < 1234; i++ {
		tw.Ok(fmt.Sprintf("test %d", i+1))
	}
	tw.Plan()

	reader := NewReader(strings.NewReader(buf.String()))
	summary := reader.Summary()
	if !summary.Valid {
		diags := reader.Diagnostics()
		for _, d := range diags {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("locale-formatted writer output did not validate")
	}
	if summary.TotalTests != 1234 {
		t.Errorf("expected 1234 tests, got %d", summary.TotalTests)
	}
	if summary.PlanCount != 1234 {
		t.Errorf("expected plan count 1234, got %d", summary.PlanCount)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test -run 'TestReaderLocale|TestReaderNoLocale' -v ./packages/tap-dancer/go/...`
Expected: FAIL — locale pragma not handled, formatted numbers not parsed

**Step 3: Implement locale-aware parsing**

3a. Add `locale` field to `frame` in `packages/tap-dancer/go/reader.go:21-29`:
```go
type frame struct {
	depth          int
	planSeen       bool
	planCount      int
	planLine       int
	testCount      int
	lastTestNumber int
	streamedOutput bool
	localeSep      string // grouping separator for active locale, empty = no locale
}
```

3b. Add import `"golang.org/x/text/language"` to `reader.go` and a helper to resolve the separator:
```go
func localeGroupingSeparator(tag language.Tag) string {
	p := message.NewPrinter(tag)
	// Format a known number and extract the separator
	formatted := p.Sprintf("%d", 1234)
	// "1,234" for en-US, "1.234" for de-DE, "1 234" for fr-FR
	if len(formatted) >= 2 {
		return string(formatted[1])
	}
	return ""
}
```

3c. Handle locale-formatting pragma in `reader.go` `Next()` method, in the `linePragma` case (around line 224-235):
```go
case linePragma:
	p := parsePragma(trimmed)
	if p.Key == "streamed-output" {
		// ... existing code ...
	}
	if strings.HasPrefix(p.Key, "locale-formatting:") {
		tag := strings.TrimPrefix(p.Key, "locale-formatting:")
		langTag, err := language.Parse(tag)
		if err == nil {
			r.currentFrame().localeSep = localeGroupingSeparator(langTag)
		}
	}
	r.lastWasTestPoint = false
	return Event{Type: EventPragma, Line: r.lineNum, Depth: depth, Raw: raw, Pragma: &p}, nil
```

3d. Modify `planRegexp` in `classify.go` to also accept non-digit chars after `1..`:
```go
planRegexp = regexp.MustCompile(`^1\.\.([\d,.\x{00a0}\x{202f} ]+)(\s+#\s+(.*))?$`)
```
Note: `\x{00a0}` is non-breaking space, `\x{202f}` is narrow no-break space (used by fr-FR in x/text).

3e. Update `parsePlan` in `parse.go` to accept a separator parameter:
```go
func parsePlan(line string) (PlanResult, error) {
	return parsePlanWithSep(line, "")
}

func parsePlanWithSep(line, sep string) (PlanResult, error) {
	m := planRegexp.FindStringSubmatch(line)
	if m == nil {
		return PlanResult{}, fmt.Errorf("invalid plan line: %q", line)
	}

	countStr := m[1]
	if sep != "" {
		countStr = strings.ReplaceAll(countStr, sep, "")
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return PlanResult{}, fmt.Errorf("invalid plan count: %v", err)
	}

	return PlanResult{
		Count:  count,
		Reason: strings.TrimSpace(m[3]),
	}, nil
}
```

3f. Update `parseTestPoint` in `parse.go` to accept a separator and consume separator chars in the digit-scanning loop:
```go
func parseTestPoint(line string) (TestPointResult, []Diagnostic) {
	return parseTestPointWithSep(line, "")
}

func parseTestPointWithSep(line, sep string) (TestPointResult, []Diagnostic) {
	var tp TestPointResult
	var diags []Diagnostic

	rest := line
	if strings.HasPrefix(rest, "not ok") {
		tp.OK = false
		rest = rest[6:]
	} else if strings.HasPrefix(rest, "ok") {
		tp.OK = true
		rest = rest[2:]
	}

	rest = strings.TrimLeft(rest, " ")

	// Parse optional test number (may contain locale separator)
	if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
		numEnd := 0
		for numEnd < len(rest) {
			c := rest[numEnd]
			if c >= '0' && c <= '9' {
				numEnd++
			} else if sep != "" && numEnd > 0 && string(c) == sep {
				numEnd++
			} else {
				break
			}
		}
		numStr := rest[:numEnd]
		if sep != "" {
			numStr = strings.ReplaceAll(numStr, sep, "")
		}
		tp.Number, _ = strconv.Atoi(numStr)
		rest = rest[numEnd:]
	}

	// Parse optional description separator " - " or "- "
	if strings.HasPrefix(rest, " - ") {
		rest = rest[3:]
	} else if strings.HasPrefix(rest, "- ") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, " ") {
		rest = rest[1:]
	}

	desc, directive, reason := splitDirective(rest)
	tp.Description = unescapeDescription(strings.TrimSpace(desc))
	tp.Directive = directive
	tp.Reason = reason

	return tp, diags
}
```

3g. Update the `linePlan` and `lineTestPoint` cases in `reader.go` `Next()` to pass the separator:
```go
case linePlan:
	f := r.currentFrame()
	// ... duplicate check ...
	plan, _ := parsePlanWithSep(trimmed, f.localeSep)
	// ... rest unchanged ...

case lineTestPoint:
	// ... version check ...
	f := r.currentFrame()
	tp, tpDiags := parseTestPointWithSep(trimmed, f.localeSep)
	// ... rest unchanged ...
```

3h. Propagate locale to child frames — when a new subtest frame is pushed (depth > currentFrame.depth), it should NOT inherit `localeSep` (per spec scoping). The child gets an empty `localeSep` and must receive its own pragma.

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test -run 'TestReaderLocale|TestReaderNoLocale' -v ./packages/tap-dancer/go/...`
Expected: PASS

**Step 5: Run all existing tests to verify no regressions**

Run: `nix develop --command go test -v ./packages/tap-dancer/go/...`
Expected: All PASS

**Step 6: Commit**

```bash
git add packages/tap-dancer/go/reader.go packages/tap-dancer/go/classify.go packages/tap-dancer/go/parse.go packages/tap-dancer/go/reader_test.go
git commit -m "feat(tap-dancer): add locale-aware number parsing to Go reader"
```

---

### Task 4: Rust writer — add `icu_decimal` dependency and locale support

**Promotion criteria:** N/A

**Files:**
- Modify: `packages/tap-dancer/rust/Cargo.toml`
- Modify: `packages/tap-dancer/rust/src/lib.rs`

**Step 1: Add dependencies to Cargo.toml**

Modify `packages/tap-dancer/rust/Cargo.toml`:
```toml
[package]
name = "tap-dancer"
version = "0.1.0"
edition = "2021"
description = "TAP-14 writer library"

[dependencies]
icu_decimal = "2"
icu_locale_core = "2"
fixed_decimal = "0.7"
```

**Step 2: Verify it compiles**

Run: `cd /home/sasha/eng/repos/purse-first/.worktrees/snug-spruce && nix develop --command cargo build -p tap-dancer`

**Step 3: Write failing tests**

Add to the `#[cfg(test)] mod tests` block in `packages/tap-dancer/rust/src/lib.rs`:

```rust
#[test]
fn writer_locale_emits_pragma() {
    let mut buf = Vec::new();
    let locale: icu_locale_core::Locale = "en-US".parse().unwrap();
    let mut tw = TapWriter::with_locale(&mut buf, locale).unwrap();
    tw.ok("first").unwrap();
    tw.plan().unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(out.contains("pragma +locale-formatting:en-US\n"));
}

#[test]
fn writer_locale_formats_large_test_number() {
    let mut buf = Vec::new();
    let locale: icu_locale_core::Locale = "en-US".parse().unwrap();
    let mut tw = TapWriter::with_locale(&mut buf, locale).unwrap();
    for _ in 0..1234 {
        tw.ok("test").unwrap();
    }
    let out = String::from_utf8(buf).unwrap();
    assert!(out.contains("ok 1,234 - test\n"), "expected 'ok 1,234', got last 200 chars: {}", &out[out.len().saturating_sub(200)..]);
}

#[test]
fn writer_locale_formats_plan_count() {
    let mut buf = Vec::new();
    let locale: icu_locale_core::Locale = "en-US".parse().unwrap();
    let mut tw = TapWriter::with_locale(&mut buf, locale).unwrap();
    tw.plan_ahead(10000).unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(out.contains("1..10,000\n"), "expected '1..10,000', got: {out}");
}

#[test]
fn writer_locale_german_separator() {
    let mut buf = Vec::new();
    let locale: icu_locale_core::Locale = "de-DE".parse().unwrap();
    let mut tw = TapWriter::with_locale(&mut buf, locale).unwrap();
    tw.plan_ahead(10000).unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(out.contains("1..10.000\n"), "expected '1..10.000', got: {out}");
}

#[test]
fn writer_locale_small_numbers_no_separator() {
    let mut buf = Vec::new();
    let locale: icu_locale_core::Locale = "en-US".parse().unwrap();
    let mut tw = TapWriter::with_locale(&mut buf, locale).unwrap();
    tw.ok("test").unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(out.contains("ok 1 - test\n"));
}

#[test]
fn writer_locale_subtest_inherits() {
    let mut buf = Vec::new();
    let locale: icu_locale_core::Locale = "en-US".parse().unwrap();
    let mut tw = TapWriter::with_locale(&mut buf, locale).unwrap();
    tw.subtest("nested", |sub| {
        sub.plan_ahead(10000)?;
        sub.plan()
    }).unwrap();
    tw.ok("nested").unwrap();
    tw.plan().unwrap();
    let out = String::from_utf8(buf).unwrap();
    assert!(out.contains("    pragma +locale-formatting:en-US\n"), "expected subtest locale pragma, got:\n{out}");
    assert!(out.contains("    1..10,000\n"), "expected subtest formatted plan, got:\n{out}");
}

#[test]
fn write_plan_locale_free_fn() {
    let mut buf = Vec::new();
    let locale: icu_locale_core::Locale = "en-US".parse().unwrap();
    let formatter = icu_decimal::DecimalFormatter::try_new(
        locale.into(),
        Default::default(),
    ).unwrap();
    write_plan_locale(&mut buf, 10000, &formatter).unwrap();
    assert_eq!(String::from_utf8(buf).unwrap(), "1..10,000\n");
}
```

**Step 4: Run tests to verify they fail**

Run: `nix develop --command cargo test -p tap-dancer -- writer_locale write_plan_locale`
Expected: FAIL — `with_locale`, `write_plan_locale` undefined

**Step 5: Implement locale support in Rust writer**

Add to `packages/tap-dancer/rust/src/lib.rs`:

1. Add imports at top:
```rust
use icu_decimal::DecimalFormatter;
use icu_locale_core::Locale;
use fixed_decimal::Decimal;
```

2. Add `formatter` and `locale` fields to `TapWriter`:
```rust
pub struct TapWriter<'a> {
    w: &'a mut dyn Write,
    counter: usize,
    failed: bool,
    plan_emitted: bool,
    locale: Option<Locale>,
    formatter: Option<DecimalFormatter>,
}
```

3. Update `new` and `child` to initialize the new fields to `None`.

4. Add `with_locale` constructor:
```rust
pub fn with_locale(w: &'a mut dyn Write, locale: Locale) -> io::Result<Self> {
    writeln!(w, "TAP version 14")?;
    let formatter = DecimalFormatter::try_new(locale.into(), Default::default())
        .map_err(|e| io::Error::new(io::ErrorKind::Other, e.to_string()))?;
    writeln!(w, "pragma +locale-formatting:{locale}")?;
    Ok(Self {
        w,
        counter: 0,
        failed: false,
        plan_emitted: false,
        locale: Some(locale),
        formatter: Some(formatter),
    })
}
```

5. Add `format_number` helper:
```rust
fn format_number(&self, n: usize) -> String {
    match &self.formatter {
        Some(fmt) => {
            let decimal = Decimal::from(n as i64);
            fmt.format(&decimal).to_string()
        }
        None => n.to_string(),
    }
}
```

6. Replace all `self.counter` and `n` formatting in `ok`, `not_ok`, `not_ok_diag`, `skip`, `todo`, `plan`, `plan_ahead` to use `self.format_number()`.

7. Update `subtest` to pass locale to child:
```rust
pub fn subtest(
    &mut self,
    name: &str,
    f: impl FnOnce(&mut TapWriter) -> io::Result<()>,
) -> io::Result<()> {
    writeln!(self.w, "    # Subtest: {}", name)?;
    let mut indent = IndentWriter { w: &mut *self.w };
    let mut child = TapWriter::child(&mut indent);
    if let Some(ref locale) = self.locale {
        child.locale = Some(locale.clone());
        child.formatter = Some(
            DecimalFormatter::try_new(locale.clone().into(), Default::default())
                .map_err(|e| io::Error::new(io::ErrorKind::Other, e.to_string()))?,
        );
        writeln!(child.w, "pragma +locale-formatting:{locale}")?;
    }
    f(&mut child)
}
```

8. Add locale-aware free function:
```rust
pub fn write_plan_locale(w: &mut impl Write, count: usize, fmt: &DecimalFormatter) -> io::Result<()> {
    let decimal = Decimal::from(count as i64);
    writeln!(w, "1..{}", fmt.format(&decimal))
}
```

**Step 6: Run tests to verify they pass**

Run: `nix develop --command cargo test -p tap-dancer -- writer_locale write_plan_locale`
Expected: PASS

**Step 7: Run all existing tests to verify no regressions**

Run: `nix develop --command cargo test -p tap-dancer`
Expected: All PASS

**Step 8: Commit**

```bash
git add packages/tap-dancer/rust/Cargo.toml packages/tap-dancer/rust/src/lib.rs
git commit -m "feat(tap-dancer): add locale-formatted number output to Rust writer"
```

---

### Task 5: Update Nix build hashes

**Promotion criteria:** N/A

**Files:**
- Modify: `flake.nix` (goVendorHash if `golang.org/x/text` wasn't already vendored)
- Modify: Cargo.lock / Rust cargo hash

**Step 1: Rebuild Go vendor hash**

Run: `just vendor-hash`

If the hash changed, update `flake.nix` with the new value.

**Step 2: Build the full marketplace to verify**

Run: `just build`
Expected: Build succeeds

**Step 3: Run the full test suite**

Run: `just test`
Expected: All tests pass

**Step 4: Commit**

```bash
git add flake.nix Cargo.lock vendor/
git commit -m "chore(tap-dancer): update build hashes for locale deps"
```

---

### Task 6: Integration validation — round-trip test

**Promotion criteria:** N/A

**Files:**
- Test: `packages/tap-dancer/go/tap_test.go` (the round-trip test was added in Task 3)

**Step 1: Run the full round-trip test**

Run: `nix develop --command go test -run 'TestReaderLocaleRoundTrip' -v ./packages/tap-dancer/go/...`
Expected: PASS — locale-formatted writer output validates correctly through the reader

**Step 2: Run all tests one final time**

Run: `just test`
Expected: All PASS

**Step 3: Final build verification**

Run: `just build`
Expected: Build succeeds
