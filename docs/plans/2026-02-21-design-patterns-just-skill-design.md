# Design: `design_patterns-just` Skill

## Summary

A bob skill that codifies justfile conventions for consistent task runner
configuration across all projects. Triggers when agents create or modify
justfiles.

## Location

- `skills/design_patterns-just/SKILL.md`
- Registered in `.claude-plugin/plugin.json` as `./skills/design_patterns-just`

## Trigger Conditions

Activates when users ask to create a justfile, add recipes, update a justfile,
set up build tasks, or are modifying a justfile in any project.

## Content

### Global Settings

- `set output-format := "tap"` (requires `amarbel-llc/just-us` fork)

### Naming Convention

`verb-noun` pattern. Verbs can nest for sub-categories (e.g. `codemod-fmt-go`).

### Verb Categories

| Verb | Purpose |
|------|---------|
| `build` | Compile, generate, or produce artifacts |
| `test` | Run test suites |
| `run` | Execute the built artifact |
| `clean` | Remove generated artifacts |
| `update` | Refresh dependencies or inputs |
| `codemod` | Automated code modifications (sub-verbs: `fmt`, `lint`, `fix`) |

### Task Hierarchy

- Bare-verb recipes (`build`, `test`, `codemod-fmt`) are aggregates that compose
  `verb-noun` leaf recipes as dependencies
- `default: build test` chains aggregates into a CI-equivalent target

### Dependency Ordering

- Positional dependencies: prerequisites (run before body)
- `&&` dependencies: post-steps (run after body)

### Anti-patterns

- Don't mix verb categories in a single recipe name
- Don't use generic names like `all`, `dev`, `check`
- Don't duplicate logic in aggregate recipes
- Don't skip the `default` recipe
- Don't comment aggregate-only recipes

## Implementation

Single self-contained `SKILL.md` file. No `references/` or `examples/`
directories needed — the content is concise enough to inline.
