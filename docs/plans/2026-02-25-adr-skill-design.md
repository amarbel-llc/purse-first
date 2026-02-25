# ADR Skill Design

## Summary

Create a general-purpose bob skill (`bob:adr`) that teaches Claude how to create
and manage Architecture Decision Records using the MADR 4.0.0 format. The skill
is project-agnostic — usable in any repo, not just purse-first.

## Skill Structure

```
skills/adr/
├── SKILL.md
└── references/
    ├── adr-template.md              # Full annotated template
    ├── adr-template-minimal.md      # Mandatory sections only, annotated
    ├── adr-template-bare.md         # All sections, no guidance
    └── adr-template-bare-minimal.md # Mandatory only, no guidance
```

## SKILL.md Content

### Frontmatter

- **name:** `adr`
- **description:** Trigger phrases: "create an ADR", "add a decision record",
  "document architecture decision", "MADR", "decision log". Also when working in
  a `docs/decisions/` directory.

### Sections

1. **Overview** — What ADRs are. One architectural decision per record. MADR
   4.0.0 format.

2. **When to Use** — Technology choices, pattern changes, trade-off evaluations,
   significant design decisions. When NOT: trivial implementation details,
   one-off bug fixes.

3. **MADR 4.0.0 Structure** — Quick reference table of sections:
   - Mandatory: Title, Context and Problem Statement, Considered Options,
     Decision Outcome, Consequences
   - Optional: Decision Drivers, Confirmation, Pros and Cons of the Options,
     More Information

4. **File & Directory Conventions** — `NNNN-title-with-dashes.md` in
   `docs/decisions/`. Sequential numbering. Lowercase, dashes for spaces.

5. **Template Selection** — Guide for choosing between the four variants:
   - Full: new teams learning MADR, complex decisions needing guidance
   - Minimal: straightforward decisions, experienced teams
   - Bare: all sections but no handholding text
   - Bare-minimal: quick decisions, lightweight recording

6. **Metadata** — YAML front matter: status, date, decision-makers, consulted,
   informed. All optional.

7. **Status Lifecycle** — proposed → accepted → deprecated | superseded by
   ADR-NNNN. Document supersession with cross-references.

8. **Writing Tips** — Keep context to 2-3 sentences. List real options (not
   strawmen). Be honest about negative consequences. Link to issues/PRs.

9. **Reference Files** — Points to the four templates in `references/`.

## Reference Files

All four MADR 4.0.0 template variants from
https://github.com/adr/madr/tree/main/template, stripped of Jekyll front matter
(not relevant outside the MADR repo). Licensed under MIT/CC0.

## Decisions

- **General-purpose scope:** Not tied to purse-first conventions. Any project
  can use this skill.
- **Default directory:** `docs/decisions/` per MADR recommendation. adr-tools
  uses `doc/adr/` and log4brains uses `docs/adr/`, but we align with MADR since
  we use MADR templates.
- **All four template variants:** Included in references/ for maximum
  flexibility. Claude picks the appropriate one based on context.
- **No adr-tools dependency:** The skill teaches the format directly rather than
  wrapping CLI tooling. Claude creates the files itself.
