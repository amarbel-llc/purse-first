# dewey reposition drift as a conformist whole-tree linter (purse-first#160).
#
# dewey's NATO-level layout (internal/<level>/<pkg>) must match each package's
# actual dependency depth; `dagnabit reposition` recomputes levels from the live
# import graph and moves packages. Nothing gated this drift — a mis-positioned
# package (e.g. dagnabit_rust sitting in echo when its deps put it at bravo) sat
# latent until someone ran `dagnabit reposition -n` by hand. This linter
# mechanizes the check: the read-only `command` runs `dagnabit reposition -n`
# (dry-run) over libs/dewey/internal and fails if any package WOULD move; the
# `repair-command` applies the moves.
#
# IMPURE: `dagnabit reposition` shells out to `go list` to resolve the module
# graph, which needs the real go.work / go.mod + the Go module cache (network,
# not a read-only /nix/store copy). It therefore lives in the impure self-check
# lane (nix/conformist-impure.nix) and runs via `just lint-worktree`, NOT the
# sandboxed checks.formatting — the same constraint that puts conformist's own
# gomod2nix linter in the impure lane.
#
# writeShellScriptBin (not writeShellApplication) so the script inherits the
# caller's PATH, where the working-tree `dagnabit` and `go` resolve (mirrors
# conformist's sweatfile linter, which needs an ambient `spinclass`).
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.linters.dewey-reposition;

  # The dewey module dir, relative to the tree root conformist runs from.
  deweyDir = "libs/dewey";

  check = pkgs.writeShellScriptBin "conformist-dewey-reposition" ''
    set -eu
    # cwd is the tree root; this whole-tree check takes no file arguments.
    [ -d "${deweyDir}/internal" ] || {
      echo "dewey-reposition: no ${deweyDir}/internal — nothing to check"
      exit 0
    }

    if ! command -v dagnabit >/dev/null 2>&1; then
      echo "dewey-reposition: dagnabit not on PATH; run inside the dev shell (just build-dagnabit)" >&2
      exit 1
    fi

    # reposition is dagnabit's DEFAULT subcommand (no explicit verb word);
    # `-n` is the dry-run. It prints an NDJSON "would-move" event per
    # mis-positioned package and is silent when the layout is correct. Any
    # output = drift.
    cd "${deweyDir}"
    moves=$(dagnabit -n internal)
    if [ -n "$moves" ]; then
      echo "dewey-reposition: ${deweyDir}/internal is out of position; run \`just debug-dewey-reposition-apply\` (or repair) and commit the moves:" >&2
      printf '%s\n' "$moves" >&2
      exit 1
    fi

    echo "dewey-reposition: ${deweyDir}/internal NATO levels are well-positioned"
  '';

  repair = pkgs.writeShellScriptBin "conformist-dewey-reposition-repair" ''
    set -eu
    [ -d "${deweyDir}/internal" ] || exit 0 # nothing to reposition

    if ! command -v dagnabit >/dev/null 2>&1; then
      echo "dewey-reposition: dagnabit not on PATH; cannot repair" >&2
      exit 1
    fi

    cd "${deweyDir}"
    # Apply the reposition (real moves, import-path rewrites, facade updates).
    # reposition is dagnabit's default subcommand (no explicit verb word).
    dagnabit internal
    echo "dewey-reposition: repositioned ${deweyDir}/internal"
  '';
in
{
  options.linters.dewey-reposition = {
    enable = lib.mkEnableOption "the dewey NATO-level reposition-drift whole-tree check (dagnabit reposition -n; repair applies the moves)";
  };

  config = lib.mkIf cfg.enable {
    settings.linter.dewey-reposition = {
      command = lib.getExe check;
      "repair-command" = lib.getExe repair;
      # Trigger gate: Go sources under libs/dewey drive the import graph the
      # reposition is computed from. A whole-tree check (passes-files=false) is
      # exempt from the global excludes — the matched files are only a trigger,
      # the script reads the real module graph via `go list` itself.
      includes = [ "${deweyDir}/**/*.go" ];
      passes-files = false;
    };
  };
}
