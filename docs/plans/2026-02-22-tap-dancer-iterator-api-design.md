# tap-dancer Go Iterator API Design

Date: 2026-02-22

## Summary

Add an iterator-based API to tap-dancer's Go library as a complementary
higher-level API on top of the existing imperative Writer. Callers pass an
`iter.Seq[TestPoint]` to `WriteAll`, and the library walks the iterator to
produce TAP-14 output including YAML diagnostics and recursive subtests.

## New Types

### Diagnostics

Structured YAML diagnostic data for a test point. Named fields cover common
TAP-14 diagnostic keys; `Extras` allows arbitrary additional keys.

```go
type Diagnostics struct {
    Message  string
    Severity string
    File     string
    Line     int
    Extras   map[string]any
}
```

Zero-value named fields are omitted from output. `Extras` keys are sorted
alphabetically and emitted after the named fields. Values that are maps or
slices are expanded as nested YAML.

### TestPoint

A single test result, optionally with subtests.

```go
type TestPoint struct {
    Description string
    Ok          bool
    Skip        string
    Todo        string
    Diagnostics *Diagnostics
    Subtests    func(*Writer)
}
```

- `Skip`: non-empty emits `# SKIP <reason>` directive
- `Todo`: non-empty emits `# TODO <reason>` directive
- `Diagnostics`: nil means no YAML block
- `Subtests`: nil means leaf test point; non-nil receives a child `*Writer`,
  allowing mixed iterator and imperative styles inside the callback

## Changes to Writer

### New field

```go
type Writer struct {
    w           io.Writer
    n           int
    depth       int
    planEmitted bool  // new
}
```

`PlanAhead()` and `Plan()` both set `planEmitted = true`.

### New method

```go
func (tw *Writer) WriteAll(tests iter.Seq[TestPoint])
```

## WriteAll Algorithm

```
for each TestPoint in iterator:
    if Subtests != nil:
        child := tw.Subtest(tp.Description)
        tp.Subtests(child)
        if !child.planEmitted: child.Plan()
        tw.Ok(tp.Description)
    else if Skip != "":
        tw.Skip(tp.Description, tp.Skip)
    else if Todo != "":
        tw.Todo(tp.Description, tp.Todo)
    else if tp.Ok:
        tw.Ok(tp.Description)
        emit diagnostics if non-nil
    else:
        tw.NotOk(tp.Description, flattened diagnostics)

if !tw.planEmitted:
    tw.Plan()
```

### Plan placement

- **Default (trailing)**: `WriteAll` emits `1..N` after exhausting the iterator.
- **Plan-first**: caller calls `tw.PlanAhead(n)` before `WriteAll`. `WriteAll`
  detects `planEmitted == true` and skips the trailing plan.
- **Subtests**: child writer's `WriteAll` auto-emits a trailing plan for the
  subtest unless the callback already emitted one.

### Diagnostics serialization

`Diagnostics` is flattened internally before YAML emission:

1. Named fields emitted in order: `message`, `severity`, `file`, `line`
2. Zero-value fields omitted (empty string, 0 for Line)
3. `Extras` keys sorted alphabetically, emitted after named fields
4. `any` values in Extras serialized: strings inline, multiline strings as
   block scalars, maps/slices as nested YAML

The existing Writer's YAML emission (currently `map[string]string` in `NotOk`)
needs an internal helper that handles `any` values. The public `NotOk` signature
stays unchanged.

### Subtest parent reporting

The parent always emits `ok` for a subtest group. TAP-14 convention is that the
subtest's own test points carry pass/fail detail.

## What stays unchanged

- All existing Writer methods: `Ok`, `NotOk`, `Skip`, `Todo`, `Subtest`,
  `Comment`, `BailOut`, `PlanAhead`, `Plan`
- Reader/parser API
- GoTest converter
- All existing tests

## Usage Examples

### Pure iterator

```go
tw := tap.NewWriter(os.Stdout)
tw.WriteAll(slices.Values([]tap.TestPoint{
    {Description: "connects to db", Ok: true},
    {Description: "inserts row", Ok: true, Diagnostics: &tap.Diagnostics{
        Message: "inserted id=42",
    }},
    {Description: "handles conflict", Ok: false, Diagnostics: &tap.Diagnostics{
        Message:  "unique violation",
        Severity: "fail",
        File:     "db_test.go",
        Line:     87,
    }},
}))
```

### Mixed iterator + imperative subtests

```go
tw := tap.NewWriter(os.Stdout)
tw.WriteAll(slices.Values([]tap.TestPoint{
    {Description: "unit tests", Subtests: func(sub *tap.Writer) {
        sub.WriteAll(unitTestIter())
    }},
    {Description: "integration", Subtests: func(sub *tap.Writer) {
        sub.Ok("setup")
        sub.WriteAll(integrationIter())
        sub.Ok("teardown")
    }},
}))
```

### Plan-first

```go
tw := tap.NewWriter(os.Stdout)
tw.PlanAhead(3)
tw.WriteAll(slices.Values([]tap.TestPoint{
    {Description: "test one", Ok: true},
    {Description: "test two", Ok: true},
    {Description: "test three", Ok: true},
}))
```
