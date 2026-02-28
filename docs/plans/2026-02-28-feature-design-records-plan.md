# Feature Design Records Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create the FDR format with a bare template and validate it by writing the first feature record.

**Architecture:** Three files: a bare template in `docs/features/`, the first real record, and a design doc commit. No code — pure markdown.

**Tech Stack:** Markdown, YAML front matter.

---

### Task 1: Create the bare FDR template

**Files:**
- Create: `docs/features/fdr-template-bare.md`

**Step 1: Create the template**

```markdown
---
status:
date:
---

# <!-- feature name -->

## Motivation

## Interface

## Examples

## Limitations

## More Information
```

**Step 2: Commit**

```
git add docs/features/fdr-template-bare.md
git commit -m "docs: add Feature Design Record bare template"
```

---

### Task 2: Write the first feature record

**Files:**
- Create: `docs/features/0001-auto-generated-session-names.md`

**Step 1: Write the record**

Use the auto-generated session names feature we just built. Fill in all
mandatory sections (Motivation, Interface, Examples) and the optional
Limitations section. Reference the design doc and commits in More Information.

```markdown
---
status: accepted
date: 2026-02-28
---

# Auto-generated session names

## Motivation

`spinclass new` requires a branch name argument. For quick sessions where the
user doesn't care about the name, this adds friction. A random human-readable
name removes the need to think of one.

## Interface

When `spinclass new` is called with no target argument, a random
`adjective-tree` name is generated (e.g., `swift-cedar`, `calm-willow`). The
name is checked against existing worktrees to avoid collisions. If a target is
provided, behavior is unchanged.

## Examples

    # auto-named session
    spinclass new

    # explicit name (unchanged)
    spinclass new my-feature

## Limitations

- ~2,500 name combinations (50 adjectives x 50 tree nouns); sufficient for
  typical concurrent session counts but not designed for high-volume use.
- No user-configurable word lists.
- Name format is always `adjective-tree`; no other patterns supported.

## More Information

- Design doc: `docs/plans/2026-02-28-spinclass-auto-names-design.md`
```

**Step 2: Commit**

```
git add docs/features/0001-auto-generated-session-names.md
git commit -m "docs: add first Feature Design Record for auto-generated session names"
```

---

### Task 3: Commit the design docs

**Files:**
- Stage: `docs/plans/2026-02-28-feature-design-records-design.md`
- Stage: `docs/plans/2026-02-28-feature-design-records-plan.md`
- Stage: `docs/plans/2026-02-28-spinclass-auto-names-design.md`

**Step 1: Commit all design docs**

```
git add docs/plans/2026-02-28-feature-design-records-design.md \
       docs/plans/2026-02-28-feature-design-records-plan.md \
       docs/plans/2026-02-28-spinclass-auto-names-design.md
git commit -m "docs: add design docs for auto-names and feature design records"
```
