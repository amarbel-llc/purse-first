# TAP-14 Validator Design

## Overview

Add a TAP-14 parser/validator to tap-dancer as both a reusable Go library and a CLI+MCP tool built with bob (`command.App` from `go-mcp`). This is the reading counterpart to tap-dancer's existing writer.

## Architecture

**Line-oriented state machine with nesting stack.** Single-pass streaming parser that processes input line-by-line. States: `Start`, `ExpectingPlan`, `Body`, `YAMLBlock`, `Done`. Subtest nesting is handled by a stack of context frames (one per depth level), each tracking its own plan, test counter, and expected count.

### Why state machine over alternatives

- TAP is inherently line-oriented and streaming
- Subtests are signaled by indentation (4-space multiples), handled with a stack rather than recursion
- Single pass, memory proportional to nesting depth, not input size
- Natural fit for piped stdin from test runners

## Project Structure

All new code lives in the existing `tap-dancer/go/` module:

```
go/
├── tap.go              # Existing writer
├── tap_test.go         # Existing writer tests
├── reader.go           # Parser/validator library
├── reader_test.go      # Parser tests
├── diagnostic.go       # Diagnostic types (errors, warnings)
├── cmd/
│   └── tap-dancer/
│       └── main.go     # bob command.App binary
├── go.mod              # Updated (add go-mcp dep)
├── go.sum              # Updated
└── gomod2nix.toml      # Updated
```

Module path stays `github.com/amarbel-llc/tap-dancer/go`. The `flake.nix` and `justfile` are updated to build the new binary.

## Library API

### Core Types

```go
type EventType int
const (
    EventVersion EventType = iota
    EventPlan
    EventTestPoint
    EventYAMLDiagnostic
    EventComment
    EventBailOut
    EventPragma
    EventSubtestStart
    EventSubtestEnd
    EventUnknown
)

type Event struct {
    Type        EventType
    Line        int
    Depth       int
    Raw         string
    TestPoint   *TestPoint
    Plan        *Plan
    BailOut     *BailOut
    YAML        map[string]string
    Comment     string
    Pragma      *Pragma
}

type TestPoint struct {
    Number      int
    Description string
    OK          bool
    Directive   Directive
    Reason      string
}

type Plan struct {
    Count  int
    Reason string
}

type Severity int
const (
    SeverityError   Severity = iota
    SeverityWarning
)

type Diagnostic struct {
    Line     int
    Severity Severity
    Rule     string
    Message  string
}

type Summary struct {
    Version    int
    TotalTests int
    Passed     int
    Failed     int
    Skipped    int
    Todo       int
    BailedOut  bool
    PlanCount  int
    Valid      bool
}
```

### Reader Interface

```go
type Reader struct { /* state machine internals */ }

func NewReader(r io.Reader) *Reader

// Streaming event iteration
func (r *Reader) Next() (Event, error)

// Validation results
func (r *Reader) Diagnostics() []Diagnostic
func (r *Reader) Summary() Summary

// io interfaces for pipeline composition
func (r *Reader) ReadFrom(src io.Reader) (int64, error)
func (r *Reader) WriteTo(dst io.Writer) (int64, error)
```

`ReadFrom` consumes an entire stream, parsing and validating. `WriteTo` writes the validation report to the destination.

## CLI / MCP Interface

Binary: `tap-dancer`, built with bob `command.App`.

### `validate` command

Reads TAP-14 from stdin (CLI) or `input` param (MCP), validates against full spec, reports diagnostics.

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `input` | String | stdin | TAP-14 text to validate |
| `strict` | Bool | false | Fail on first error |
| `format` | String | `text` | Output: `text`, `json`, or `tap` |

**Output formats:**
- **text** — Human-readable diagnostics with line numbers
- **json** — `Summary` + `[]Diagnostic` as JSON
- **tap** — Validation results as TAP-14 (each rule is a test point)

**Exit codes:** `0` = valid, `1` = errors found, `2` = usage error.

## Validation Rules

### Errors (MUST violations)

| Rule ID | Check |
|---------|-------|
| `version-required` | First line must be `TAP version 14` |
| `version-format` | Exact text, no extra whitespace |
| `plan-required` | Exactly one plan line per document |
| `plan-duplicate` | No second plan line |
| `plan-format` | Must match `1..N` or `1..N # reason` |
| `plan-count-mismatch` | Plan count must equal test point count |
| `test-status-required` | Must start with `ok` or `not ok` |
| `yaml-indent` | YAML must be `(depth*4)+2` spaces |
| `yaml-unclosed` | `---` without matching `...` |
| `subtest-indent` | Subtests indented by multiples of 4 |
| `subtest-unterminated` | Subtest without parent test point |
| `bailout-format` | Must be `Bail out!` (exact case with `!`) |
| `escape-invalid` | Only `\#` and `\\` are valid escapes |

### Warnings (SHOULD violations)

| Rule ID | Check |
|---------|-------|
| `test-number-missing` | No explicit test number |
| `test-number-sequence` | Numbers should be sequential |
| `test-number-duplicate` | Same number used twice |
| `description-separator` | Should use ` - ` separator |
| `directive-whitespace` | Space required before/after `#` |
| `directive-nonconformant` | `#` without proper spacing |
| `plan-skip-nonzero` | `1..0` should have a skip reason |
| `subtest-version` | Subtests should omit version line |

## State Machine

### States

- **Start** — Expecting `TAP version 14`
- **Header** — Version seen, expecting plan or first test
- **Body** — Processing test points, comments, pragmas, subtests
- **YAMLBlock** — Inside a YAML diagnostic block
- **Done** — EOF reached, final validation

### Nesting Stack

Each frame tracks:
- `planSeen bool`
- `planCount int`
- `testCount int`
- `lastTestNumber int`
- `depth int`

Indent detection: `leadingSpaces / 4 = depth`. Push frame on depth increase, pop on decrease. Validate completed subtest on pop (plan count matches test count).

### YAML Block Handling

YAML blocks use `(depth * 4) + 2` spaces of indentation. The `---` marker opens a block, `...` closes it. Lines between are accumulated as YAML content. The `inYAML` flag is tracked per stack frame.

## Nix Integration

Update `flake.nix` to add a `tap-dancer-cli` package using `buildGoApplication` from the go devenv overlay:

```nix
tap-dancer-cli = pkgs.buildGoApplication {
  pname = "tap-dancer";
  version = "0.1.0";
  src = ./go;
  modules = ./go/gomod2nix.toml;
  subPackages = [ "cmd/tap-dancer" ];
};
```

The `default` package (`symlinkJoin`) includes the new binary alongside existing packages.

Update `justfile` with targets: `build-cli`, `test-go` (expanded), `deps` (updated for new dependency).

## Dependencies

New Go dependency: `github.com/amarbel-llc/purse-first/libs/go-mcp` (bob framework). No other external dependencies — the parser/validator is pure stdlib.
