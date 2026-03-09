# Sweatfile Create-Session Hook & Snob-Case Rename

## Problem

Repos with non-trivial setup (PHP include paths, composer dependencies, custom
envrc directives) can't express their worktree bootstrap requirements in the
sweatfile. Users must maintain external scripts and remember to run them after
`spinclass new`. Additionally, TOML field names inconsistently mix snake_case
and snob-case.

## Changes

### 1. TOML Field Rename (snake_case to snob-case)

| Old TOML key | New TOML key |
|---|---|
| `git_excludes` | `git-excludes` |
| `claude_allow` | `claude-allow` |
| `stop_hook` | removed (moves to `[hooks]` table) |

Fields already in snob-case are unchanged: `system-prompt`,
`system-prompt-append`, `branch-name-command`, `boundary-notify`.

No backward compatibility. The `CheckUnknownFields` validator flags old names
as unknown fields, serving as migration guidance.

### 2. `[hooks]` Table

```toml
[hooks]
create = "composer install --no-scripts --working-dir=\"$WORKTREE\""
stop = "just build test"
```

Replaces the top-level `stop_hook` field. Groups all lifecycle hooks.

**Struct:**
```go
type Hooks struct {
    Create *string `toml:"create"`
    Stop   *string `toml:"stop"`
}
```

**Merge semantics:** Same as current `stop_hook` --- scalar override per field.
`nil` = inherit, empty string = clear, non-empty = override. `create` and
`stop` merge independently.

**Create hook execution:**
- Runs after `applyWorktreeConfig()` (after `.envrc`, Claude settings, local
  bin are written)
- Receives `$WORKTREE` as env var (absolute path), no positional arguments
- Working directory is the worktree
- Failure aborts worktree creation (worktree is removed on hook failure)

### 3. `envrc-directives`

```toml
envrc-directives = ["source_up", "dotenv_if_exists"]
```

**Struct:**
```go
EnvrcDirectives []string `toml:"envrc-directives"`
```

**Merge semantics:** Same as `git-excludes` and `claude-allow` --- `nil` =
inherit, `[]` = clear, non-empty = append.

**Default behavior:** When `nil` after merging, falls back to current behavior:
`source_up` + `use flake` (if `flake.nix` exists). When explicitly set, the
list is used verbatim --- no `flake.nix` auto-detection.

`PATH_add ".git/spinclass/bin"` is always appended regardless of directives.

### 4. `[env]` Table

```toml
[env]
PHP_INI_SCAN_DIR = ":$WORKTREE/.php-ini"
SOME_PATH = "${HOME}/lib:${WORKTREE}/vendor"
```

**Struct:**
```go
Env map[string]string `toml:"env"`
```

**Merge semantics:** Map merge --- repo keys override base keys with the same
name. `nil` = inherit. Individual keys cleared by setting to empty string.

**Variable interpolation:** Values expanded via `os.Expand` (supports `$VAR`
and `${VAR}`). The mapping function provides `WORKTREE` (absolute worktree
path) plus the current process environment as fallback.

**Rendering:** Written as `.spinclass.env` in the worktree root (key=value, one
per line, sorted by key). Only written when non-nil and non-empty after merging.

**Auto-directive:** When `env` is non-empty after merging, `dotenv .spinclass.env`
is automatically appended to the envrc directives. Hooks that want their own
`.env` can write one and add `dotenv_if_exists` to `envrc-directives`
independently.

## Complete Struct

```go
type Hooks struct {
    Create *string `toml:"create"`
    Stop   *string `toml:"stop"`
}

type Experimental struct {
    BoundaryNotify *bool `toml:"boundary-notify"`
}

type Sweatfile struct {
    SystemPrompt       *string           `toml:"system-prompt"`
    SystemPromptAppend *string           `toml:"system-prompt-append"`
    BranchNameCommand  string            `toml:"branch-name-command"`
    GitSkipIndex       []string          `toml:"git-excludes"`
    ClaudeAllow        []string          `toml:"claude-allow"`
    EnvrcDirectives    []string          `toml:"envrc-directives"`
    Env                map[string]string `toml:"env"`
    Hooks              *Hooks            `toml:"hooks"`
    Experimental       *Experimental     `toml:"experimental"`
}
```

## Example Sweatfile

```toml
# vim: ft=toml

system-prompt = ""
branch-name-command = "snob-case"

git-excludes = [".php-ini/", ".spinclass.env"]
claude-allow = ["Bash(composer:*)"]

envrc-directives = ["source_up", "dotenv_if_exists"]

[env]
PHP_INI_SCAN_DIR = ":$WORKTREE/.php-ini"

[hooks]
create = """
mkdir -p "$WORKTREE/.php-ini"
cat > "$WORKTREE/.php-ini/worktree.ini" <<EOF
include_path=$WORKTREE/htdocs/../phplib:.
EOF
composer install --no-scripts --working-dir="$WORKTREE"
"""

[experimental]
boundary-notify = true
```

## Files Affected

- `packages/spinclass/internal/sweatfile/sweatfile.go` --- struct changes, rename TOML tags
- `packages/spinclass/internal/sweatfile/hierarchy.go` --- merge logic for new fields
- `packages/spinclass/internal/sweatfile/apply.go` --- envrc generation from directives, .spinclass.env writing, create hook execution
- `packages/spinclass/internal/sweatfile/sweatfile_test.go` --- update all TOML strings, add tests for new fields
- `packages/spinclass/internal/sweatfile/apply_test.go` --- tests for envrc directives, env rendering
- `packages/spinclass/internal/validate/validate.go` --- update field name references
- `packages/spinclass/internal/hooks/hooks.go` --- read `Hooks.Stop` instead of `StopHook`
- `sweatfile` --- update to new key names
- `rcm/config/spinclass/sweatfile` --- update to new key names (in eng repo)
