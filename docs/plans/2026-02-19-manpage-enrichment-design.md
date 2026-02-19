# Manpage Enrichment Design

## Problem

The current manpage generation produces one page per subcommand (matching Unix
convention), but the main app page is too thin — it has only NAME, DESCRIPTION,
and COMMANDS. It lacks SYNOPSIS, EXAMPLES, and SEE ALSO sections. There is also
no way for command authors to declare usage examples, which means manpages can't
show realistic invocations or workflows.

Beyond manpages, there is no structured source of "how to use this tool"
examples that could feed into skill generation. Examples authored for manpages
should become the canonical source that downstream tooling (skill documents, help
output) consumes.

## Approach

Add an `Example` struct and `Examples` fields to the command framework. Enrich
both the app-level and per-command manpages with new sections. Provide a
scaffold command to reduce cold-start friction for authors.

## Data Model

New type in `command.go`:

```go
type Example struct {
    Description string // what this example demonstrates
    Command     string // the shell invocation (may be multi-line)
    Output      string // optional expected output snippet
}
```

New fields on existing types:

```go
type Command struct {
    // ... existing fields ...
    Examples []Example
}

type App struct {
    // ... existing fields ...
    Examples []Example
}
```

### Template Variables

Examples support template variables (e.g., `{{repo_path}}`, `{{file}}`) so
they aren't hardcoded to specific paths. Different renderers resolve variables
appropriately:

- Manpage rendering substitutes sensible defaults
- Skill generation may substitute differently or preserve variables
- Exact variable set and resolution syntax TBD during implementation

## Enriched App Manpage

Current sections: NAME, DESCRIPTION, COMMANDS.

New structure:

```roff
.TH GRIT 1 "2026-02-19" "grit 0.1.0"

.SH NAME
grit \- MCP server exposing git operations

.SH SYNOPSIS
.B grit
.I command
.RI [ options ]

.SH DESCRIPTION
An MCP server exposing git operations as both CLI subcommands
and MCP tools.

.SH COMMANDS
.TP
.BR add (1)
Stage files for commit
.TP
.BR commit (1)
Create a new commit with staged changes

.SH EXAMPLES
.TP
Stage changes and commit:
.nf
$ grit add --repo_path=. --paths='["main.go"]'
$ grit commit --repo_path=. --message='fix: resolve nil pointer'
.fi

.SH SEE ALSO
.BR grit-add (1),
.BR grit-commit (1),
.BR grit-status (1)
```

Changes from current:

- **SYNOPSIS** shows general invocation pattern plus any app-level params
- **COMMANDS** entries cross-reference their subcommand manpage with `(1)`
- **EXAMPLES** rendered from `App.Examples` in `.nf`/`.fi` blocks
- **SEE ALSO** auto-generated from visible subcommand list

## Enriched Per-Command Manpage

Current sections: NAME, SYNOPSIS, DESCRIPTION, OPTIONS, ALIASES.

Additions:

- **EXAMPLES** section after OPTIONS, rendered from `Command.Examples`
- **SEE ALSO** back-referencing the main app page

## Scaffold Command

```
purse-first scaffold-examples --app=grit
```

Or equivalently via a bob skill.

The scaffold step:

1. Inspects the app's command registry (via plugin.json or binary introspection)
2. Generates stub `Example` entries for each command using required params with
   type-appropriate placeholders
3. Author curates the generated examples in Go source
4. Build (`GenerateAll`) renders curated examples into manpages

## Example Lifecycle

```
scaffold-examples
    -> author curates in Go source
    -> build (GenerateAll)
        -> manpages (for humans)
        -> skill generation (for agents, future)
        -> help output (future)
```

Manpage examples are the canonical source. Skill documents and help output
consume them downstream.

## Scope

**In scope:**

- `Example` struct in `command.go`
- `Examples` field on `Command` and `App`
- Enriched `writeAppManpage`: SYNOPSIS, EXAMPLES, SEE ALSO
- EXAMPLES and SEE ALSO on `writeCommandManpage`
- Template variable placeholder convention (defined, not resolved)
- `scaffold-examples` subcommand or bob skill
- Updated tests

**Not in scope (future work):**

- Skill generation consuming examples
- Help output (`--help`) consuming examples
- Template variable resolution engine
- Updating existing consumers (grit, lux, chix) with actual examples
