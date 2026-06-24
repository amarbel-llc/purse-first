# dewey reposition drift as a conformist whole-tree linter (purse-first#160).
#
# dewey's NATO-level layout (internal/<level>/<pkg>) must match each package's
# actual dependency depth; `dagnabit reposition` recomputes levels from the live
# import graph and moves packages. Nothing gated this drift — a mis-positioned
# package (e.g. dagnabit_rust sitting in echo when its deps put it at bravo) sat
# latent until someone ran `dagnabit reposition -n` by hand. This linter
# mechanizes the check: the read-only `command` runs `dagnabit reposition -n`
# (dry-run) over <deweyDir>/internal and fails if any package WOULD move; the
# `repair-command` applies the moves.
#
# REUSABLE: published from purse-first's flake as
# `lib.conformistLinters.dewey-reposition` (purse-first#163 Step 2) so other
# dewey-layout repos can import it. Parameterized by:
#   - deweyDir        — the internal/ root ("libs/dewey" for purse-first, "go"
#                       for madder)
#   - dagnabitPackage — null ⇒ ambient PATH dagnabit (purse-first self-test); a
#                       pinned package ⇒ hermetic, PATH-independent invocation
#
# IMPURE: `dagnabit reposition` shells out to `go list` to resolve the module
# graph, which needs the real go.work / go.mod + the Go module cache (network,
# not a read-only /nix/store copy). It therefore lives in the impure self-check
# lane (e.g. nix/conformist-impure.nix) and runs via `just lint-worktree`, NOT
# the sandboxed checks.formatting — the same constraint that puts conformist's
# own gomod2nix linter in the impure lane.
#
# writeShellScriptBin (not writeShellApplication) so the script inherits the
# caller's PATH, where an ambient `dagnabit`/`go` resolve when dagnabitPackage is
# null (mirrors conformist's sweatfile linter, which needs an ambient `spinclass`).
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.linters.dewey-reposition;

  # The dewey-layout module dir, relative to the tree root conformist runs from.
  deweyDir = cfg.deweyDir;

  # The dagnabit binary the scripts invoke: a pinned store path when
  # dagnabitPackage is set, else the bare name resolved from the caller's PATH.
  dagnabitBin =
    if cfg.dagnabitPackage != null then "${cfg.dagnabitPackage}/bin/dagnabit" else "dagnabit";

  # A PATH-existence guard, emitted only in the ambient case (dagnabitPackage ==
  # null); a baked store path always exists so no guard is needed there.
  ambientGuard =
    failMsg:
    lib.optionalString (cfg.dagnabitPackage == null) ''
      if ! command -v dagnabit >/dev/null 2>&1; then
        echo "dewey-reposition: ${failMsg}" >&2
        exit 1
      fi
    '';

  check = pkgs.writeShellScriptBin "conformist-dewey-reposition" ''
    set -eu
    # cwd is the tree root; this whole-tree check takes no file arguments.
    [ -d "${deweyDir}/internal" ] || {
      echo "dewey-reposition: no ${deweyDir}/internal — nothing to check"
      exit 0
    }

    ${ambientGuard "dagnabit not on PATH; run inside the dev shell (build dagnabit) or set dagnabitPackage"}
    # reposition is dagnabit's DEFAULT subcommand (no explicit verb word);
    # `-n` is the dry-run. It prints an NDJSON "would-move" event per
    # mis-positioned package and is silent when the layout is correct. Any
    # output = drift.
    cd "${deweyDir}"
    moves=$(${dagnabitBin} -n internal)
    if [ -n "$moves" ]; then
      echo "dewey-reposition: ${deweyDir}/internal is out of position; reposition (\`dagnabit internal\` / your repair recipe) and commit the moves:" >&2
      printf '%s\n' "$moves" >&2
      exit 1
    fi

    echo "dewey-reposition: ${deweyDir}/internal NATO levels are well-positioned"
  '';

  repair = pkgs.writeShellScriptBin "conformist-dewey-reposition-repair" ''
    set -eu
    [ -d "${deweyDir}/internal" ] || exit 0 # nothing to reposition

    ${ambientGuard "dagnabit not on PATH; cannot repair"}
    cd "${deweyDir}"
    # Apply the reposition (real moves, import-path rewrites, facade updates).
    # reposition is dagnabit's default subcommand (no explicit verb word).
    ${dagnabitBin} internal
    echo "dewey-reposition: repositioned ${deweyDir}/internal"
  '';
in
{
  options.linters.dewey-reposition = {
    enable = lib.mkEnableOption "the dewey NATO-level reposition-drift whole-tree check (dagnabit reposition -n; repair applies the moves)";

    deweyDir = lib.mkOption {
      type = lib.types.str;
      default = "libs/dewey";
      description = ''
        Path, relative to the conformist tree root, of the dewey-layout module
        root that holds internal/. purse-first: "libs/dewey"; madder: "go". The
        trigger-gate includes glob is "<deweyDir>/**/*.go".
      '';
    };

    dagnabitPackage = lib.mkOption {
      type = lib.types.nullOr lib.types.package;
      default = null;
      description = ''
        The dagnabit package whose binary the scripts invoke. null ⇒ resolve
        `dagnabit` from the ambient PATH (purse-first self-tests its working-tree
        build, placed on PATH by `just lint-worktree`). Set to a pinned package
        (e.g. purse-first.packages.<sys>.dagnabit) for a hermetic, PATH-independent
        invocation — the downstream consumer case.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    settings.linter.dewey-reposition = {
      command = lib.getExe check;
      "repair-command" = lib.getExe repair;
      # Trigger gate: Go sources under deweyDir drive the import graph the
      # reposition is computed from. A whole-tree check (passes-files=false) is
      # exempt from the global excludes — the matched files are only a trigger,
      # the script reads the real module graph via `go list` itself.
      includes = [ "${deweyDir}/**/*.go" ];
      passes-files = false;
    };
  };
}
