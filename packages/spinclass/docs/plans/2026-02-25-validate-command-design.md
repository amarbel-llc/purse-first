# Design: `spinclass validate` Command

## Purpose

Validate the sweatfile hierarchy starting from the current working directory.
Reports structural, semantic, and application-level issues using TAP-14 output
with subtests.

## Architecture

New `internal/validate/` package with validation logic. Wired as
`spinclass validate` in `main.go`. The `internal/tap/` writer is extended with
subtest support.

### Components

- `internal/validate/validate.go` — `Run()` orchestrates hierarchy loading,
  per-file checks, merged-result checks, and dry-run apply checks
- `internal/validate/checks.go` — individual check functions returning `[]Issue`
- `internal/tap/tap.go` — extended with `Subtest(name) *Writer`

### Data Flow

```
PWD → DetectRepo → LoadHierarchy → per-file checks → merge checks → apply checks → TAP-14 output
```

## Validation Checks

### Per-File Structural

1. **Valid TOML** — file parses without error
2. **No unknown fields** — only `git_excludes` and `claude_allow` are valid keys

### Per-File Semantic — `claude_allow`

3. **Rule syntax** — must be `ToolName` or `ToolName(pattern)`, reusing
   `parseRule` from `internal/perms/match.go`
4. **Known tools** — warn on unrecognized tool names. Known set: `Bash`, `Read`,
   `Write`, `Edit`, `Glob`, `Grep`, `WebFetch`, `WebSearch`, `NotebookEdit`,
   `Task`, `Skill`, `LSP`

### Per-File Semantic — `git_excludes`

5. **Non-empty strings** — no empty exclude patterns
6. **No absolute paths** — excludes should be relative patterns

### Merged Result

7. **Duplicate detection** — warn on duplicate entries across files

### Applied (Dry-Run)

8. **Git excludes structure** — merged excludes are valid
9. **Claude settings structure** — merged rules produce valid settings JSON

## TAP-14 Output Format

Top-level test points: one per hierarchy file + "merged result" + "apply
(dry-run)". Each has subtests for individual checks. Files not found are skipped
with `# SKIP not found`.

```
TAP version 14
1..4
    # Subtest: ~/.config/spinclass/sweatfile
    1..2
    ok 1 - valid TOML
    ok 2 - claude_allow syntax valid
ok 1 - ~/.config/spinclass/sweatfile
    # Subtest: /Users/me/eng/repos/foo/sweatfile
    1..3
    ok 1 - valid TOML
    ok 2 - claude_allow syntax valid
    ok 3 - git_excludes valid
ok 2 - /Users/me/eng/repos/foo/sweatfile
    # Subtest: merged result
    1..1
    ok 1 - no duplicate entries
ok 3 - merged result
    # Subtest: apply (dry-run)
    1..2
    ok 1 - git excludes structure valid
    ok 2 - claude settings structure valid
ok 4 - apply (dry-run)
```

Failures include YAML diagnostics:

```
    not ok 2 - claude_allow syntax valid
      ---
      message: unknown tool name
      severity: warning
      rule: FooBar(something)
      ...
```

## Exit Code

- 0 — all checks pass (warnings don't cause failure)
- 1 — any check fails

## Tap Writer Extension

Add `Subtest(name string) *Writer` to `internal/tap/`. A subtest emits an
indented TAP stream preceded by `# Subtest: <name>`. The parent test point is
emitted after the subtest completes, with pass/fail determined by whether the
subtest had any failures.

## Known Tools List

```
Bash, Read, Write, Edit, Glob, Grep, WebFetch, WebSearch, NotebookEdit, Task, Skill, LSP
```

This list is defined as a package-level variable in validate so it can be
extended.
