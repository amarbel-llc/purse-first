---
status: accepted
date: 2026-02-25
---

# Use MADR for Architecture Decision Records

## Context and Problem Statement

The purse-first project has accumulated 25+ design documents in `docs/plans/` containing architectural decisions embedded in prose. These documents are not standardized and there is no way to browse decisions without reading full documents. We need a lightweight, structured format for recording architectural decisions that supports tradeoff analysis.

## Considered Options

* Nygard format
* MADR 4.0.0
* No formal ADR process (status quo)

## Decision Outcome

Chosen option: "MADR 4.0.0", because it provides structured tradeoff analysis with Considered Options and Consequences sections that the simpler Nygard format lacks, accepting the overhead of maintaining a separate `docs/decisions/` directory alongside existing design docs.

ADRs are stored in `docs/decisions/` using `NNNN-title-with-dashes.md` naming with sequential numbering.

### Consequences

* Good, because decisions are discoverable by browsing filenames without reading full documents.
* Good, because the structured format forces explicit tradeoff documentation with Considered Options and Consequences.
* Good, because MADR is the most widely adopted modern ADR format with tooling support and community familiarity.
* Bad, because architectural context is now split between design docs in `docs/plans/` and ADRs in `docs/decisions/`.

## More Information

Design docs in `docs/plans/` remain the primary vehicle for detailed technical design. ADRs capture the decision and rationale; design docs capture the how.
