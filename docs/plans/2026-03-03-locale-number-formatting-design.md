# Locale Number Formatting for tap-dancer

## Context

TAP-14 amendment (`locale-formatting-amendment.md` in the tap spec repo) allows
locale-specific grouping separators in test point IDs and plan counts via
`pragma +locale-formatting:<bcp47-tag>`. This makes large test suites more
readable: `ok 1,234` (en-US), `ok 1.234` (de-DE), `1..10 000` (fr-FR).

tap-dancer has Go and Rust writer libraries plus a Go reader/parser. All three
need locale support.

## Approach

Use proper locale-aware formatting libraries rather than hardcoding separator
tables. The spec lists 5 example locales but we support any valid BCP 47 tag.

- **Go**: `golang.org/x/text/message` + `golang.org/x/text/language`
- **Rust**: `icu_decimal` + `icu_locale_core` (ICU4X)

## Writer API (Go)

Add locale support to `Writer`:

```go
func NewLocaleWriter(w io.Writer, locale language.Tag) *Writer
func (tw *Writer) SetLocale(tag language.Tag)
```

When locale is set:
1. Emit `pragma +locale-formatting:<tag>` after `TAP version 14`
2. Format all test point IDs and plan counts via
   `message.NewPrinter(tag).Sprintf("%d", n)`

Subtests inherit the parent's locale and auto-emit their own pragma line
(spec requires each subtest to declare its own pragma).

## Writer API (Rust)

Same pattern using `icu_decimal::DecimalFormatter`:

```rust
pub fn with_locale(w: &'a mut dyn Write, locale: icu_locale_core::Locale) -> io::Result<Self>
pub fn set_locale(&mut self, locale: icu_locale_core::Locale)
```

Free functions gain locale-aware variants or an `Option<&DecimalFormatter>`
parameter.

Subtests inherit locale and auto-emit pragma, same as Go.

## Reader/Parser (Go only)

1. `parsePragma` stores locale when key is `locale-formatting:<tag>`
2. `planRegexp` widened to accept digits plus grouping characters
3. `parseTestPoint` digit-scanning loop extended to consume separator chars
4. Before `strconv.Atoi`, strip grouping separators for the active locale
5. Locale stored per-frame in `Reader` state (subtest-scoped)

## Dependencies

- **Go**: `golang.org/x/text` (may already be vendored as indirect dep)
- **Rust**: `icu_decimal`, `icu_locale_core` (new to workspace)
- After adding: `just vendor && just vendor-hash` (Go), cargo hash update (Rust)

## Testing

Writer tests (both languages):
- Format numbers with en-US, de-DE, fr-FR, hi-IN (lakh), ja-JP
- Pragma emitted after version line
- Subtest inherits locale, emits own pragma
- Plan lines use formatted numbers

Reader tests (Go):
- Parse locale-formatted plan counts and test point IDs
- Stripping produces correct integers
- Locale scoping: subtest pragma independent of parent
- No pragma = plain integer parsing (backwards compatible)

## Rollback

Purely additive. Writers default to no locale (plain integers). Reader is
backwards-compatible without pragma. Rollback = don't call `SetLocale`.
