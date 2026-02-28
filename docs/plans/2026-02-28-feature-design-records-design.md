# Feature Design Records (FDR)

**Date:** 2026-02-28

## Problem

ADRs capture architectural decisions and their rationale. Manpages capture
usage. Neither captures feature-level design intent: what a feature does, why it
exists, and what it deliberately excludes. A lightweight format for recording
feature design and interface (not implementation) would fill this gap.

## Design

Feature Design Records are a sibling standard to MADR-based ADRs. They live in
`docs/features/` and follow the same file conventions: `NNNN-title-with-dashes.md`,
sequential numbering, one feature per file.

### YAML Front Matter

```yaml
---
status: accepted
date: 2026-02-28
---
```

Status lifecycle mirrors ADRs: `proposed` -> `accepted` -> `deprecated` /
`superseded by FDR-NNNN`.

### Sections

| Section       | Required | Purpose                                              |
|---------------|----------|------------------------------------------------------|
| Title (H1)    | Yes      | Feature name, scannable in file listing              |
| Motivation    | Yes      | What user need this addresses (2-3 sentences)        |
| Interface     | Yes      | Commands, flags, defaults, behavior                  |
| Examples      | Yes      | Concrete usage showing the feature in action         |
| Limitations   | No       | What the feature deliberately does not do            |
| More Info     | No       | Links to ADRs, PRs, design docs                     |

### Differentiation

**vs ADRs:** No Considered Options, no Pros/Cons. Features have Motivation and
Limitations instead of Decision Drivers and Consequences. Interface replaces
Decision Outcome.

**vs manpages:** Captures design intent (Motivation, Limitations), not just
usage. Answers "why does this work this way?" alongside "how do I use this?"

## Deliverables

1. Bare template at `docs/features/fdr-template-bare.md`
2. First record: auto-generated session names (`0001-auto-generated-session-names.md`)
3. Future: bob skill for creating FDRs (tracked as TODO)
