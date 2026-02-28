# bob:fdr Skill

**Date:** 2026-02-28

## Problem

We created a Feature Design Record (FDR) format as a sibling to ADRs. The ADR
format has a bob skill that guides agents through creating records. FDRs need
the same.

## Design

Mirror the bob:adr skill structure, adapted for the FDR format.

### Deliverables

1. `skills/fdr/SKILL.md` — Skill document
2. `skills/fdr/references/fdr-template-bare.md` — Single bare template
3. `.claude-plugin/plugin.json` — Register skill

### SKILL.md Sections

Parallels the ADR skill: intro, When to Use, FDR Structure table, Section
Guidance, File/Directory Conventions, Metadata, Status Lifecycle, Writing Tips,
Reference Files, Related Skills.

### Key Differences from ADR Skill

- Simpler metadata (status + date only, no RACI fields)
- Fewer record sections (Motivation, Interface, Examples, Limitations, More Info)
- Single bare template (no 4-variant selection)
- Writing tips focus on describing behavior and intent, not trade-off analysis
- Trigger criteria focus on feature documentation, not architectural decisions
