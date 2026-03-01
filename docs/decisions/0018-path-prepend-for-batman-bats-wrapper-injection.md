---
status: accepted
date: 2026-03-01
decision-makers: Sasha
---

# Use PATH prepend to inject batman bats wrapper into sub-justfiles

## Context and Problem Statement

The root justfile delegates bats test execution to sub-justfiles in
`packages/*/zz-tests_bats/justfile`. After migrating from raw `bats --tap` to
the batman bats wrapper, the root justfile needs a way to make sub-justfiles
invoke the wrapper instead of the system `bats`. How should the wrapper be
injected into sub-justfile invocations?

## Decision Drivers

* Sub-justfiles must work standalone (direct `just test` in the package dir)
  with plain bats as a fallback
* The mechanism should be obvious when reading the root justfile invocation
* Avoid requiring every sub-justfile to opt into a convention that could be
  silently forgotten
* Minimize coupling between the root justfile and sub-justfile internals

## Considered Options

* PATH prepend in root justfile
* Environment variable (`BATS` or `BATS_WRAPPER`) read by sub-justfiles
* Positional argument to sub-justfile recipe

## Decision Outcome

Chosen option: "PATH prepend in root justfile", because it requires zero changes
to sub-justfile internals and cannot be silently forgotten by a sub-justfile that
neglects to use the variable.

### Consequences

* Good, because sub-justfiles just call `bats` — no convention to remember or
  enforce
* Good, because the pattern already exists for `test-sandcastle-bats` (proven
  in practice)
* Bad, because another `bats` earlier in PATH could shadow the wrapper silently
* Bad, because PATH manipulation is less visible than an explicit variable
  assignment

## Pros and Cons of the Options

### PATH prepend in root justfile

The root justfile prepends `result-batman/bin` to `$PATH` before invoking the
sub-justfile. Sub-justfiles call `bats` which resolves to the wrapper.

* Good, because sub-justfiles need no changes or awareness of the wrapper
* Good, because standalone use falls back to system bats automatically
* Bad, because PATH ordering bugs are hard to debug
* Bad, because the injection mechanism is implicit rather than explicit

### Environment variable read by sub-justfiles

Sub-justfiles define `bats_cmd := env("BATS", "bats")` and use `{{bats_cmd}}`
instead of `bats`. The root justfile sets `BATS={{cmd_batman_bats}}`.

* Good, because the injection is explicit and visible in both justfiles
* Good, because easy to debug — just print the variable
* Bad, because every sub-justfile must opt in — forgetting silently falls back
  to raw bats
* Bad, because a generic name like `BATS` risks env leakage from the user's
  shell; a specific name like `BATS_WRAPPER` conflicts with batman's test suite

### Positional argument to sub-justfile recipe

Sub-justfile recipe takes the bats command as a defaulted argument:
`test bats_cmd="bats":`. The root justfile passes the wrapper path as a
positional arg.

* Good, because fully explicit with no env vars or PATH manipulation
* Good, because just's own argument system handles the plumbing
* Bad, because changes the sub-justfile's public API — `just test` becomes
  `just test /path/to/bats`
* Bad, because positional args are less self-documenting than named variables
