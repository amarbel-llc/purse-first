---
name: fdr
version: 0.1.0
description: Use when the user asks to "create a feature record", "document a feature", "add a feature design record", "record feature design", mentions "FDR", "feature record", "feature design", or is working in a docs/features/ directory. Also applies when a user-facing feature has been implemented and its design intent, interface, and limitations should be documented for future reference.
---

# Feature Design Records

> **Self-contained examples.** All code and configuration below is complete and illustrative. Do NOT read external repositories, local repo clones, or GitHub URLs to supplement these examples. Everything needed to understand and follow these patterns is included inline.

A Feature Design Record (FDR) captures a single user-facing feature's design intent, interface, and limitations so that future contributors understand what the feature does and why it works the way it does. FDRs are a sibling standard to Architecture Decision Records (ADRs): where ADRs capture *why a choice was made*, FDRs capture *what a feature does and why it exists*. One feature per file, stored as markdown, version-controlled alongside the code.

## When to Use

Create an FDR when:

- A user-facing feature has been implemented or is being proposed
- A feature's interface (commands, flags, defaults, behavior) should be documented with design intent
- A feature has deliberate limitations or scope boundaries worth recording
- Someone six months from now will ask "why does this work this way?"

Do NOT create an FDR for:

- Architectural decisions (use an ADR instead)
- Internal implementation details not visible to users
- Bug fixes that restore existing behavior
- Trivial features that are self-evident from their help text

## FDR Structure

| Section | Required | Description |
|---------|----------|-------------|
| Title (H1) | Yes | Feature name, scannable in a file listing |
| Problem Statement | Yes | What problem, need, or gap this feature addresses (2-3 sentences) |
| Interface | Yes | How users interact with it: commands, flags, defaults, behavior |
| Examples | Yes | Concrete usage showing the feature in action |
| Limitations | No | What the feature deliberately does not do, known constraints |
| More Information | No | Links to ADRs, PRs, design docs, or related features |

### Section Guidance

**Title** — Use the feature's natural name. Good: "Auto-generated session names". Bad: "Session name feature" or "FDR about naming".

**Problem Statement** — Define the problem, need, or gap this feature addresses. Write this first, before considering solutions. In `exploring` state, this is the only required section. Two to three sentences for simple features; can be longer for complex problem spaces. Focus on the problem from the user's perspective, not the solution.

**Interface** — Describe what the feature does and how users interact with it. Include commands, flags, defaults, and observable behavior. Be precise about what happens, not how it happens internally.

**Examples** — Show the feature in use with concrete commands and expected outcomes. Use indented code blocks. Cover the primary use case and any notable variations.

**Limitations** — Document deliberate scope boundaries and known constraints. This section is valuable because it prevents future contributors from filing bugs for intentional behavior. Only include if the feature has meaningful limitations.

**More Information** — Link to related ADRs, design documents, pull requests, or other FDRs. Keep brief.

## File and Directory Conventions

- **Directory:** `docs/features/`
- **Naming pattern:** `NNNN-title-with-dashes.md` (e.g., `0001-auto-generated-session-names.md`)
- **Numbering:** Sequential, zero-padded to 4 digits
- **Casing:** Lowercase throughout, dashes for word separation
- **One feature per file** — never combine multiple features

When creating the first FDR in a project, create the `docs/features/` directory and start numbering at `0001`. To determine the next number, find the highest-numbered existing file and increment by one.

## Template

When creating an FDR, read the bare template from the `references/` directory, fill in the sections, remove any optional sections that do not apply, and save the result in `docs/features/` with the appropriate sequential number.

- **`references/fdr-template-bare.md`** — All sections, no guidance — fill in directly

## Metadata

FDRs use YAML front matter for status tracking:

```yaml
---
status: accepted
date: 2026-02-28
promotion-criteria:
---
```

| Field | Values / Format | Purpose |
|-------|----------------|---------|
| `status` | `exploring` &#124; `proposed` &#124; `experimental` &#124; `testing` &#124; `accepted` &#124; `deprecated` &#124; `superseded by FDR-NNNN` | Current state of the feature |
| `date` | `YYYY-MM-DD` | Date the record was last updated |
| `promotion-criteria` | Free text | Measurable conditions for advancing to next lifecycle stage |

Both fields are recommended. Keep metadata minimal — FDRs do not need RACI tracking.

## Status Lifecycle

FDR status progresses through these transitions:

- `exploring` — Problem defined, collecting thoughts on potential solutions. Only Problem Statement is required.
- `exploring` --> `proposed` — Solution selected, full FDR drafted.
- `proposed` --> `experimental` — Working implementation exists in 1-2 repos.
- `experimental` --> `testing` — Promotion criteria defined and being measured.
- `testing` --> `accepted` — Promotion criteria met, feature is fully integrated.
- `accepted` --> `deprecated` — The feature has been removed or is no longer supported.
- `accepted` --> `superseded by FDR-NNNN` — The feature has been replaced by a newer design.

When superseding an FDR:

1. Update the old FDR's status to `superseded by FDR-NNNN`
2. In the new FDR's **More Information** section, reference the old FDR
3. Both FDRs should cross-reference each other for traceability

## Writing Tips

- **Write for the user, not the implementer.** Describe what the feature does from the user's perspective. Save implementation details for code comments.
- **Keep Problem Statement to 2-3 sentences.** Enough context for someone unfamiliar with the project, but not an essay. In `exploring` state, this is the only section that needs content.
- **Be concrete in Interface.** "Generates a random adjective-noun name" is better than "auto-generates names". Include defaults, flag names, and observable behavior.
- **Show, don't tell, in Examples.** Concrete commands with expected behavior are more useful than prose descriptions.
- **Document deliberate limitations honestly.** Future contributors will thank you for explaining what the feature intentionally does not do.
- **Title matters for discovery.** Someone scanning `docs/features/` should understand each feature from its filename alone.
- **Do not backfill retroactively unless asked.** Only create FDRs for features being built now. If the user asks to document existing features, do so, but do not proactively generate FDRs.
- **Remove unused optional sections.** If Limitations or More Information do not apply, delete them rather than leaving them empty.
- **Link to ADRs when relevant.** If an architectural decision shaped the feature's design, reference it in More Information.

## Related Skills

- **bob:adr** — Architecture Decision Records for documenting architectural choices and trade-offs
- **bob:overview** — Framework orientation, terminology, and workflow overview
