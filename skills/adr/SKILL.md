---
name: adr
version: 0.1.0
description: Use when the user asks to "create an ADR", "add a decision record", "document architecture decision", "record a design decision", mentions "MADR", "decision log", "architecture decision record", or is working in a docs/decisions/ directory. Also applies when making significant architectural choices, technology selections, or trade-off evaluations that should be documented for future reference.
---

# Architecture Decision Records

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

An Architecture Decision Record (ADR) captures a single architectural decision and its rationale so that future contributors understand why the codebase is shaped the way it is. This skill uses the MADR 4.0.0 (Markdown Architectural Decision Records) format: one decision per file, stored as markdown. ADRs are lightweight, version-controlled, and live alongside the code they describe.

## When to Use

Create an ADR when:

- Selecting a technology, framework, or library (e.g., choosing between SQLite and PostgreSQL)
- Adopting a significant design pattern (e.g., event sourcing, hexagonal architecture)
- Evaluating trade-offs with meaningful consequences for the system
- Changing an established convention (e.g., switching test frameworks, restructuring modules)
- Making infrastructure or deployment decisions (e.g., container orchestration, CI/CD strategy)

Do NOT create an ADR for:

- Trivial implementation details (variable naming, minor refactors)
- One-off bug fixes that do not change architecture
- Changes that do not affect system structure, interfaces, or constraints

## MADR 4.0.0 Structure

| Section | Required | Description |
|---------|----------|-------------|
| Title (H1) | Yes | Short title representing the problem and solution |
| Context and Problem Statement | Yes | 2-3 sentences describing the problem being addressed |
| Decision Drivers | No | Forces or concerns influencing the decision |
| Considered Options | Yes | List of alternatives that were evaluated |
| Decision Outcome | Yes | Chosen option with a brief justification |
| Consequences | Yes | Good and bad outcomes of the decision |
| Confirmation | No | How to verify the decision was implemented correctly |
| Pros and Cons of the Options | No | Detailed analysis per option |
| More Information | No | Links to issues, PRs, evidence, or team agreements |

### Section Guidance

**Title** — Use the format "Use X for Y" or "Adopt X" to make the decision scannable in a file listing. Good: "Use PostgreSQL for persistent storage". Bad: "Database decision".

**Context and Problem Statement** — Describe the situation that motivates the decision. State the problem as a question or as a need. Two to three sentences are sufficient; link to external design documents for deeper background.

**Decision Drivers** — List the forces that shape which option wins: performance requirements, team expertise, licensing constraints, deadline pressure, existing infrastructure. These are the criteria by which you evaluate the options.

**Considered Options** — A numbered or bulleted list of genuine alternatives. Each option gets a short label (used as a reference in the Pros and Cons section). Include "do nothing" or "status quo" only when it is a real possibility.

**Decision Outcome** — State the chosen option and why it was selected. Use the Y-statement pattern: "Chosen option: X, because it achieves Y, accepting Z." Follow with the Consequences subsections.

**Consequences** — Split into Good (benefits realized) and Bad (costs accepted). Neutral consequences are optional. Be concrete: "Reduces query latency by ~40%" is better than "Improves performance".

## File and Directory Conventions

- **Directory:** `docs/decisions/`
- **Naming pattern:** `NNNN-title-with-dashes.md` (e.g., `0001-use-madr-for-adrs.md`)
- **Numbering:** Sequential, zero-padded to 4 digits
- **Casing:** Lowercase throughout, dashes for word separation
- **One decision per file** — never combine multiple decisions

When creating the first ADR in a project, create the `docs/decisions/` directory and start numbering at `0001`. To determine the next number, find the highest-numbered existing file and increment by one.

## Template Selection

Four template variants are available. Choose based on the decision complexity and your familiarity with the format:

| Template | When to Use |
|----------|-------------|
| `adr-template.md` | Complex decisions needing full analysis with all optional sections |
| `adr-template-minimal.md` | Straightforward decisions — mandatory sections with guidance text |
| `adr-template-bare.md` | All sections available but no guidance — fill in directly |
| `adr-template-bare-minimal.md` | Quick decisions — just headings, fill in the blanks |

**Recommendation:** Start with `adr-template-bare.md` for most decisions. It includes every section as a ready-to-fill scaffold without cluttering the document with instructional text. Use `adr-template.md` when learning the format for the first time or documenting a particularly complex decision that benefits from section-by-section guidance.

When creating an ADR, read the selected template from the `references/` directory, fill in the sections, remove any sections that are not applicable, and save the result in `docs/decisions/` with the appropriate sequential number.

## Metadata

ADRs support optional YAML front matter for tracking status and stakeholders:

```yaml
---
status: accepted
date: 2026-02-25
decision-makers: Alice, Bob
consulted: Carol (security), Dave (infrastructure)
informed: Engineering team
---
```

| Field | Values / Format | Purpose |
|-------|----------------|---------|
| `status` | `exploring` &#124; `proposed` &#124; `experimental` &#124; `testing` &#124; `accepted` &#124; `rejected` &#124; `deprecated` &#124; `superseded by ADR-NNNN` | Current state of the decision |
| `date` | `YYYY-MM-DD` | Date the record was last updated |
| `decision-makers` | Comma-separated names | People who made or approved the decision |
| `consulted` | Names with optional role context | Subject-matter experts consulted (two-way communication) |
| `informed` | Names or team names | Stakeholders kept up-to-date (one-way communication) |

All fields are optional. At minimum, include `status` and `date` to make the record useful for future readers.

## Status Lifecycle

ADR status progresses through these transitions:

- `exploring` -- Problem defined, collecting thoughts on potential approaches.
- `exploring` --> `proposed` -- Approach selected, full ADR drafted.
- `proposed` --> `accepted` -- The decision is adopted and should be followed.
- `proposed` --> `rejected` -- The decision was considered but not adopted.
- `accepted` --> `experimental` -- Decision implemented in limited scope, not yet validated.
- `experimental` --> `testing` -- Promotion criteria defined and being measured.
- `testing` --> `accepted` -- Promotion criteria met, decision fully validated.
- `accepted` --> `deprecated` -- The decision is no longer relevant (e.g., the feature was removed).
- `accepted` --> `superseded by ADR-NNNN` -- The decision is replaced by a newer one.

When superseding an ADR:

1. Update the old ADR's status to `superseded by ADR-NNNN` (referencing the new ADR number)
2. In the new ADR's **More Information** section, reference the old ADR: "Supersedes ADR-NNNN"
3. Both ADRs should cross-reference each other for traceability

## Writing Tips

- **Keep Context and Problem Statement to 2-3 sentences.** Provide enough background for someone unfamiliar with the project, but do not write an essay. Link to detailed design docs if more context is needed.
- **List real options.** Do not include strawman alternatives just to pad the list. Every considered option should be something the team would genuinely consider.
- **Be honest about negative consequences.** Every decision has trade-offs. Documenting them builds trust and helps future decision-makers understand constraints.
- **Use the Y-statement format for Decision Outcome.** Write concise justifications: "Chosen option: X, because it achieves Y, accepting Z." This forces clarity about what you gain and what you give up.
- **Link to evidence.** Reference relevant issues, pull requests, benchmarks, or design documents in the More Information section.
- **Write for future readers.** The primary audience is someone encountering this decision six months from now and asking "why did we do it this way?"
- **Keep Considered Options brief.** A short paragraph or bullet list per option is sufficient. Save detailed analysis for the Pros and Cons section if needed.
- **Title matters for discovery.** Someone scanning a `docs/decisions/` directory should understand each decision from its filename and H1 title alone. Prefer "Use X for Y" or "Adopt X over Y" patterns.
- **Do not backfill retroactively unless asked.** Only create ADRs for decisions being made now. If the user asks to document past decisions, do so, but do not proactively generate ADRs for existing architecture.
- **Remove unused optional sections.** If you chose a template with optional sections (like `adr-template-bare.md`) but a section does not apply, delete it rather than leaving it empty. Empty sections add noise.

## Reference Files

Consult these MADR 4.0.0 templates when creating ADRs:

- **`references/adr-template.md`** — Full template with all sections and explanatory guidance
- **`references/adr-template-minimal.md`** — Mandatory sections only with guidance text
- **`references/adr-template-bare.md`** — All sections, no guidance — recommended starting point
- **`references/adr-template-bare-minimal.md`** — Mandatory sections only, no guidance — for quick decisions

## Related Skills

- **bob:overview** — Framework orientation, terminology, and workflow overview
