# Bash Library Design

## Problem

The TAP-14 bash writer library lives in batman (`lib/tap-writer/`) but is used by standalone scripts (e.g. git aliases) that don't have `BATS_LIB_PATH` set. It logically belongs in tap-dancer alongside the Go and Rust implementations.

## Design

### File layout

```
bash/
  load.bash          # entry point: sources src/*.bash, prints version line, sets up trailing plan trap
  src/
    plan.bash        # tap_plan <count>
    run.bash         # tap_run [--no-bail] <desc> <cmd...>
    skip.bash        # tap_skip <desc> <reason>
    comment.bash     # tap_comment <text>
    bail_out.bash    # tap_bail_out <reason>
```

Files are moved from batman's `lib/tap-writer/` as-is.

### Nix packaging

New `tap-dancer-bash` derivation in `flake.nix`:

- Installs to `$out/share/tap-dancer/lib/`
- Exports `TAP_DANCER_LIB` via `nix-support/setup-hook`
- Exposed as `packages.bash-lib`
- Added to the default `symlinkJoin`

### Consumer usage

Standalone scripts:

```bash
source "${TAP_DANCER_LIB}/load.bash"
```

### Batman follow-up (separate PR)

- Remove `lib/tap-writer/` directory
- Add tap-dancer as a flake input
- Update `bats-libs` to include tap-dancer's bash-lib package and alias for BATS discovery
