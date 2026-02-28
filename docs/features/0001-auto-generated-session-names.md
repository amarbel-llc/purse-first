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
