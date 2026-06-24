# dewey pkgs/ facade-export drift as a conformist whole-tree linter (purse-first#163).
#
# `dagnabit export --library` regenerates libs/dewey/pkgs/ public facades from
# the internal/ packages. Editing a public symbol in internal/ without
# regenerating produces no local fmt/lint signal — the drift only surfaces late
# at the merge gate. This linter mechanizes the check: the read-only `command`
# runs `dagnabit export --check --library` (no writes; nonzero + names the
# out-of-sync packages on drift); the `repair-command` runs `dagnabit export
# --library` to regenerate the facades in place (the half that did not exist as
# a conformist capability before — it makes `conformist` repair / `just
# codemod-fmt` resync facades). Mirrors nix/linters/dewey-reposition.nix.
#
# IMPURE: `dagnabit export` shells out to `go`/`go list` for the package graph
# and to `conformist` for facade formatting, so it cannot run in the sandboxed
# checks.formatting (read-only /nix/store copy, no module cache). It lives in
# the impure self-check lane (nix/conformist-impure.nix), run via `just
# lint-worktree` — the same constraint that puts dewey-reposition and
# conformist's own gomod2nix linter in the impure lane.
#
# CONFIG THREADING: unlike reposition, export's facade-format step invokes a
# formatter, so it needs a config. purse-first has NO conformist.toml on disk
# (Nix-generated config), so both scripts point dagnabit at the generated config
# via DAGNABIT_CONFORMIST_CONFIG — short-circuiting dagnabit's upward
# conformist.toml search so it does not escalate to a stray ancestor
# ~/eng/conformist.toml (purse-first#159). The store path is baked in (set from
# flake.nix via the `conformistConfig` option) rather than relying on ambient
# env, which is the whole point of #163. DAGNABIT_CEILING_DIRECTORIES is a
# belt-and-suspenders bound, set at runtime to the tree root conformist runs
# from (captured before the `cd` into libs/dewey).
#
# writeShellScriptBin (not writeShellApplication) so the script inherits the
# caller's PATH, where the working-tree `dagnabit` and `go` resolve (mirrors
# dewey-reposition, which needs an ambient `dagnabit`/`go`).
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.linters.dewey-facade-export;

  # The dewey module dir, relative to the tree root conformist runs from.
  deweyDir = "libs/dewey";

  check = pkgs.writeShellScriptBin "conformist-dewey-facade-export" ''
    set -eu
    # cwd is the tree root; this whole-tree check takes no file arguments.
    [ -d "${deweyDir}/internal" ] || {
      echo "dewey-facade-export: no ${deweyDir}/internal — nothing to check"
      exit 0
    }

    if ! command -v dagnabit >/dev/null 2>&1; then
      echo "dewey-facade-export: dagnabit not on PATH; run inside the dev shell (just build-dagnabit)" >&2
      exit 1
    fi

    # Capture the tree root before descending; it bounds dagnabit's upward
    # config walk (belt-and-suspenders alongside the explicit config below).
    root="$PWD"
    cd "${deweyDir}"

    # `export --check --library` re-exports every facade and compares it to the
    # committed pkgs/ without writing; it exits nonzero and names the out-of-sync
    # packages on drift. DAGNABIT_CONFORMIST_CONFIG points the facade-format pass
    # at purse-first's real (Nix-generated) config (purse-first#159).
    if ! DAGNABIT_CONFORMIST_CONFIG="${cfg.conformistConfig}" \
         DAGNABIT_CEILING_DIRECTORIES="$root" \
         dagnabit export --check --library; then
      echo "dewey-facade-export: ${deweyDir}/pkgs/ is out of sync with internal/; run \`just codemod-fmt\` (or dagnabit export --library) and commit the regenerated facades" >&2
      exit 1
    fi

    echo "dewey-facade-export: ${deweyDir}/pkgs/ facades are in sync with internal/"
  '';

  repair = pkgs.writeShellScriptBin "conformist-dewey-facade-export-repair" ''
    set -eu
    [ -d "${deweyDir}/internal" ] || exit 0 # nothing to export

    if ! command -v dagnabit >/dev/null 2>&1; then
      echo "dewey-facade-export: dagnabit not on PATH; cannot repair" >&2
      exit 1
    fi

    root="$PWD"
    cd "${deweyDir}"

    # Regenerate the facades in place (no --check). Same config threading as the
    # check half so the regenerated facades are formatted identically.
    DAGNABIT_CONFORMIST_CONFIG="${cfg.conformistConfig}" \
      DAGNABIT_CEILING_DIRECTORIES="$root" \
      dagnabit export --library
    echo "dewey-facade-export: regenerated ${deweyDir}/pkgs/ facades"
  '';
in
{
  options.linters.dewey-facade-export = {
    enable = lib.mkEnableOption "the dewey pkgs/ facade-export drift check (dagnabit export --check --library; repair regenerates the facades)";

    conformistConfig = lib.mkOption {
      # path (not str) so the `.#conformist-config` derivation is accepted and
      # coerced to its store path; a bare str rejects the derivation value.
      type = lib.types.path;
      description = ''
        Store path to the Nix-generated conformist config that dagnabit's
        facade-format pass runs via DAGNABIT_CONFORMIST_CONFIG, so it formats
        facades with purse-first's real config instead of walking up to a stray
        ancestor conformist.toml (purse-first#159). Set from flake.nix to the
        `.#conformist-config` output (conformistEval.config.build.configFile) —
        the PURE formatter config (goimports/gofumpt), not the impure
        self-check config.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    settings.linter.dewey-facade-export = {
      command = lib.getExe check;
      "repair-command" = lib.getExe repair;
      # Trigger gate: ALL Go sources under libs/dewey — both internal/ (the
      # facade source) AND pkgs/ (the generated facades themselves). conformist
      # only runs a passes-files=false whole-tree linter when a file matching
      # `includes` is in scope, so this MUST cover pkgs/ too: otherwise a stale
      # or hand-edited facade (a pkgs/ change with internal/ untouched) would not
      # trip the lane. Matches the sibling dewey-reposition's broad include. The
      # matched files are only a trigger — the script reads the real internal/ →
      # pkgs/ relationship itself (and is exempt from the global excludes).
      includes = [ "${deweyDir}/**/*.go" ];
      passes-files = false;
    };
  };
}
