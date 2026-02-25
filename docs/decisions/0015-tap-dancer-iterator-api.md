---
status: accepted
date: 2026-02-25
---

# Add iterator-based API to tap-dancer Go library

## Context and Problem Statement

The tap-dancer Go library required callers to manually call individual `Ok`, `NotOk`, `Skip`, and `Todo` methods with string arguments. This was error-prone and didn't enforce structural correctness -- callers had to remember version headers, plan line placement, and YAML diagnostic formatting. There was no way to express a test suite as structured data.

## Considered Options

* Keep string-based imperative API only
* Add structured `TestPoint` type with `WriteAll` iterator consumer
* Add a builder/fluent API

## Decision Outcome

Chosen option: "Add structured TestPoint type with WriteAll iterator consumer", because it provides a composable, type-safe API that handles TAP-14 formatting (plan placement, YAML diagnostics, subtest nesting) automatically while remaining complementary to the existing imperative API, accepting the additional API surface area.

### Consequences

* Good, because `TestPoint` with `Diagnostics` struct enforces correct TAP-14 structure at the type level (skip directives, todo directives, YAML diagnostic blocks).
* Good, because `WriteAll` auto-emits trailing plan lines, eliminating a common source of malformed TAP output.
* Good, because subtests compose naturally -- `TestPoint.Subtests` receives a child `*Writer`, allowing mixed iterator and imperative styles.
* Bad, because two API styles (imperative and iterator) increase the surface area callers need to understand.
* Neutral, because all existing Writer methods and tests remain unchanged -- this is purely additive.

## Pros and Cons of the Options

### Keep string-based imperative API only

* Good, because no new types or methods to maintain.
* Bad, because callers must manually handle plan placement, diagnostic YAML formatting, and subtest indentation.
* Bad, because structural errors (missing plan lines, malformed YAML) are only caught at runtime by TAP consumers.

### Add structured TestPoint type with WriteAll iterator consumer

* Good, because test suites can be expressed as data (`[]TestPoint`) and iterated.
* Good, because `Diagnostics` struct provides named fields for common TAP-14 YAML keys plus an `Extras` map for arbitrary keys.
* Good, because plan-first and plan-trailing modes are handled automatically via `planEmitted` tracking.
* Bad, because `iter.Seq[TestPoint]` requires Go 1.23+ iterator support.

### Add a builder/fluent API

* Good, because familiar chaining pattern for some Go developers.
* Bad, because fluent APIs in Go tend to be verbose and do not compose well with standard iteration patterns.
* Bad, because builder state management adds complexity without clear benefit over the iterator approach.

## More Information

* Design: `docs/plans/2026-02-22-tap-dancer-iterator-api-design.md`
* Key types: `TestPoint`, `Diagnostics`, `Writer.WriteAll(iter.Seq[TestPoint])`
