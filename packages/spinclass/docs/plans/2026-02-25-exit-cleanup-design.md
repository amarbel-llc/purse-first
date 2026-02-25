# Shop Exit Cleanup Design

## Problem

When a user exits a spinclass session (detaches from zmx) and the worktree is
dirty, `closeShop` silently logs the status and exits. The worktree is left
behind with no guidance. Users must manually decide what to do.

## Solution

When the worktree is dirty AND stdin is an interactive terminal, prompt the user
with three choices:

1. **Discard changes** — reset all tracked changes and remove untracked files,
   then proceed to merge
2. **Reattach** — re-enter the zmx session to continue working, then re-run
   closeShop on exit
3. **Exit without integrating** — current behavior (log status, leave worktree)

When not interactive (piped, CI, etc.), keep current behavior unchanged.

## Architecture

### closeShop changes (`internal/shop/shop.go`)

Add `exec executor.Executor` parameter (already available from `New`) and
`interactive bool` to `closeShop`.

When dirty AND interactive:

- Call `promptDirtyAction()` returning one of three enum values
- **Discard**: run `git checkout .` then `git clean -fd` on the worktree path,
  then fall through to the merge path
- **Reattach**: call `exec.Attach()` again with the same parameters, then
  re-run `closeShop` (loop, not recursion — use a for loop)
- **Exit**: current behavior

### promptDirtyAction (`internal/shop/shop.go`)

Uses `charmbracelet/huh` Select (already a direct dependency) to present
three options. Returns a typed constant.

### TTY detection

In `New()`, check `isatty.IsTerminal(os.Stdin.Fd())` to determine
interactivity. `mattn/go-isatty` is already an indirect dependency — promote
to direct.

### Migrate gum → huh in merge (`internal/merge/merge.go`)

Replace `chooseWorktree`'s `exec.Command("gum", "choose")` with
`huh.NewSelect`. This removes the external `gum` dependency from merge and
makes the codebase consistent.

### Discard implementation

Use `git checkout .` + `git clean -fd` (all-at-once discard). No per-file
walkthrough.

## Scope

### In scope

- Interactive dirty-worktree prompt in `closeShop`
- Reattach loop (re-enter session, re-run close on exit)
- Discard-all shortcut
- Migrate `merge.chooseWorktree` from gum to huh
- Promote `mattn/go-isatty` to direct dependency
- Tests for `promptDirtyAction` and the discard path

### Out of scope

- Changing the `clean` command's interactive mode (already uses huh)
- Adding new CLI flags
- Modifying the `--no-merge` behavior
