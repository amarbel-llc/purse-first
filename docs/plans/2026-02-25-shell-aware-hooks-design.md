# Shell-Aware Hook Matching

**Date**: 2026-02-25
**Status**: Approved

## Problem

PreToolUse hooks match Bash commands using `strings.HasPrefix(command, prefix)`.
Agents frequently prefix commands with `cd <dir> &&`, which causes the hook to
see `cd` instead of the actual command. Example:

```
cd /home/user/repo && git log --oneline master..HEAD 2>/dev/null || echo "(no commits ahead)"
```

Registered prefix `"git log"` fails because the command starts with `"cd"`.

## Solution

Parse the command string into a bash AST using `mvdan.cc/sh/v3/syntax`, extract
all simple commands, and run prefix matching against each one. If any matches,
deny the entire Bash invocation.

## Scope

All changes in `libs/go-mcp/command/`. No package-level changes. Packages
continue declaring `CommandPrefixes` as static strings.

## New file: `shellparse.go`

Single function:

```go
func extractSimpleCommands(command string) []string
```

- Parses with `syntax.NewParser(syntax.Variant(syntax.LangBash))`
- Walks the AST collecting `*syntax.CallExpr` nodes
- Reconstructs each as a space-joined string of word literals
- On parse failure, returns `[]string{command}` (fallback to raw prefix match)

## Change to `HandleHook`

After extracting the command string (hook.go:58), when the tool is `"Bash"` and
command is non-empty, call `extractSimpleCommands(command)`. Loop over each
extracted command and try `FindToolMatch`. First match wins — deny the whole
invocation.

`FindToolMatch` stays unchanged.

## Edge cases

| Input                              | Extracted                                    | Match? |
|------------------------------------|----------------------------------------------|--------|
| `git status`                       | `["git status"]`                             | Yes    |
| `cd /foo && git log ...`           | `["cd /foo", "git log ..."]`                 | Yes    |
| `git status && echo done`          | `["git status", "echo done"]`                | Yes    |
| `git log \| head`                  | `["git log", "head"]`                        | Yes    |
| `(cd /foo && git log)`             | `["cd /foo", "git log"]`                     | Yes    |
| `git log 2>/dev/null`              | `["git log"]`                                | Yes    |
| `echo "git status"`               | `["echo git status"]`                        | No     |
| Unparseable input                  | `["<raw command>"]`                           | Fallback |

## Testing

New `shellparse_test.go` with table-driven tests for all edge cases above.

Update `hook_test.go` with integration test: `HandleHook` with
`cd /foo && git log` input matches the `git log` mapping.

## Dependency

Add `mvdan.cc/sh/v3` to `libs/go-mcp/go.mod`. Requires `go mod tidy` and
vendor hash update in top-level `flake.nix`.

## Future work

Pivot hook matching to delegation: let packages provide custom matchers instead
of the framework owning all matching logic via static `CommandPrefixes`/
`Extensions`. Tracked in TODO.md.
