# ADR Skill Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a bob skill (`bob:adr`) that teaches Claude how to create and manage Architecture Decision Records using MADR 4.0.0.

**Architecture:** Skill-only package addition — a new `skills/adr/` directory with `SKILL.md` and four MADR template files in `references/`. No code changes, no Nix changes, no plugin.json changes needed (skill discovery is automatic via `skills/*/SKILL.md` globbing).

**Tech Stack:** Markdown (MADR 4.0.0 format)

---

### Task 1: Create reference templates

**Files:**
- Create: `skills/adr/references/adr-template.md`
- Create: `skills/adr/references/adr-template-minimal.md`
- Create: `skills/adr/references/adr-template-bare.md`
- Create: `skills/adr/references/adr-template-bare-minimal.md`

**Step 1: Create the references directory**

Run: `mkdir -p skills/adr/references`

**Step 2: Write the full annotated template**

Write `skills/adr/references/adr-template.md` with the MADR 4.0.0 full template
content from https://github.com/adr/madr/tree/main/template. Strip Jekyll front
matter (parent, nav_order, title fields). Keep all sections with explanatory
guidance text in curly braces.

**Step 3: Write the minimal annotated template**

Write `skills/adr/references/adr-template-minimal.md` with mandatory sections
only (Title, Context and Problem Statement, Considered Options, Decision Outcome,
Consequences) plus guidance text.

**Step 4: Write the bare template**

Write `skills/adr/references/adr-template-bare.md` with all sections present but
no guidance text — just empty sections ready to fill in.

**Step 5: Write the bare-minimal template**

Write `skills/adr/references/adr-template-bare-minimal.md` with mandatory
sections only, no guidance text.

**Step 6: Commit**

```
git add skills/adr/references/
git commit -m "feat(bob): add MADR 4.0.0 reference templates for ADR skill"
```

---

### Task 2: Write the SKILL.md

**Files:**
- Create: `skills/adr/SKILL.md`

**Step 1: Write SKILL.md**

Write `skills/adr/SKILL.md` with:

- YAML frontmatter: `name: adr`, description with trigger phrases
- Self-contained examples banner (matches existing bob skill convention)
- Overview: what ADRs are, MADR 4.0.0
- When to Use: architectural decisions, technology choices, trade-offs; when NOT
- MADR structure quick reference table (mandatory vs optional sections)
- File & directory conventions: `NNNN-title-with-dashes.md` in `docs/decisions/`
- Template selection guide: which variant for which situation
- Metadata: YAML front matter fields (status, date, decision-makers, consulted, informed)
- Status lifecycle: proposed → accepted → deprecated/superseded
- Writing tips
- Reference files section pointing to the four templates

Target: ~1500 words. Keep lean per writing-skills guidance.

**Step 2: Commit**

```
git add skills/adr/SKILL.md
git commit -m "feat(bob): add ADR skill for MADR 4.0.0 decision records"
```

---

### Task 3: Verify skill discovery

**Step 1: Verify file structure**

Run: `find skills/adr -type f | sort`

Expected:
```
skills/adr/SKILL.md
skills/adr/references/adr-template-bare-minimal.md
skills/adr/references/adr-template-bare.md
skills/adr/references/adr-template-minimal.md
skills/adr/references/adr-template.md
```

**Step 2: Verify SKILL.md frontmatter**

Read the first 10 lines of `skills/adr/SKILL.md` and confirm `name` and
`description` fields are present and valid.

**Step 3: Build and verify skill is included in package output**

Run: `nix build` in the repo root, then check:
`ls result/share/purse-first/bob/skills/adr/`

Expected: `SKILL.md` and `references/` directory present.
