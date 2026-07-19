# dewey pkgs/ facade-export drift as a conformist whole-tree linter (purse-first#163).
#
# `dagnabit export` regenerates a dewey-layout module's pkgs/ public facades from
# its internal/ packages. Editing a public symbol in internal/ without
# regenerating produces no local fmt/lint signal — the drift only surfaces late
# at the merge gate. This linter mechanizes the check: the read-only `command`
# runs `dagnabit export --check [--library]` (no writes; nonzero + names the
# out-of-sync packages on drift); the `repair-command` runs `dagnabit export
# [--library]` to regenerate the facades in place (makes `conformist` repair /
# the consumer's codemod-fmt resync facades). Mirrors nix/linters/dewey-reposition.nix.
#
# REUSABLE: this module is published from purse-first's flake as
# `lib.conformistLinters.dewey-facade-export` (purse-first#163 Step 2) so other
# dewey-layout repos (madder) can import it. Parameterized by:
#   - deweyDir       — the internal/+pkgs/ root ("libs/dewey" for purse-first, "go"
#                      for madder)
#   - library        — pass `--library` (purse-first, no //go:generate directives)
#                      vs directive-scan export (madder)
#   - dagnabitPackage — null ⇒ ambient PATH dagnabit (purse-first self-test); a
#                      pinned package ⇒ hermetic, PATH-independent invocation (madder)
#   - conformistConfig — the PURE formatter config the facade-format pass runs
#
# IMPURE: `dagnabit export` shells out to `go`/`go list` for the package graph
# and to `conformist` for facade formatting, so it cannot run in the sandboxed
# checks.formatting (read-only /nix/store copy, no module cache). It lives in
# the impure self-check lane (e.g. nix/conformist-impure.nix), run via `just
# lint-worktree` — the same constraint that puts dewey-reposition and
# conformist's own gomod2nix linter in the impure lane.
#
# CONFIG THREADING: unlike reposition, export's facade-format step invokes a
# formatter, so it needs a config. A repo with NO conformist.toml on disk
# (Nix-generated config) points dagnabit at the generated config via
# DAGNABIT_CONFORMIST_CONFIG — short-circuiting dagnabit's upward conformist.toml
# search so it does not escalate to a stray ancestor ~/eng/conformist.toml
# (purse-first#159). The store path is baked in (set via the `conformistConfig`
# option) rather than relying on ambient env, which is the whole point of #163.
# DAGNABIT_CEILING_DIRECTORIES is a belt-and-suspenders bound, set at runtime to
# the tree root conformist runs from (captured before the `cd` into deweyDir).
#
# The config fed to dagnabit's nested pass has any `working-dir` line stripped
# first (sanitizeConfigForNestedPass): that pass's tree root is already
# deweyDir (both scripts `cd` there before invoking dagnabit), so a
# `working-dir` matching deweyDir — the common case when the SAME config also
# drives the consumer's own repo-root-scoped `nix fmt` / `lint-fmt` — would
# double the descent (deweyDir/deweyDir, a nonexistent chdir target). Found
# and fixed via madder + chrest, both of which fed their outer conformistEval
# straight through and hit `chdir .../go/go: no such file or directory`.
#
# writeShellScriptBin (not writeShellApplication) so the script inherits the
# caller's PATH, where an ambient `dagnabit`/`go` resolve when dagnabitPackage is
# null (mirrors dewey-reposition).
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.linters.dewey-facade-export;

  # The dewey-layout module dir, relative to the tree root conformist runs from.
  deweyDir = cfg.deweyDir;

  # The dagnabit binary the scripts invoke: a pinned store path when
  # dagnabitPackage is set, else the bare name resolved from the caller's PATH.
  dagnabitBin =
    if cfg.dagnabitPackage != null then "${cfg.dagnabitPackage}/bin/dagnabit" else "dagnabit";

  # `--library` (export every package under internal/) vs directive-scan export.
  libraryFlag = lib.optionalString cfg.library " --library";

  # A PATH-existence guard, emitted only in the ambient case (dagnabitPackage ==
  # null); a baked store path always exists so no guard is needed there.
  ambientGuard =
    failMsg:
    lib.optionalString (cfg.dagnabitPackage == null) ''
      if ! command -v dagnabit >/dev/null 2>&1; then
        echo "dewey-facade-export: ${failMsg}" >&2
        exit 1
      fi
    '';

  # dagnabit's facade-format pass runs with its tree root ALREADY at deweyDir
  # (both scripts `cd "${deweyDir}"` below before invoking it). A formatter in
  # `cfg.conformistConfig` may carry a `working-dir` key scoping it from the
  # OUTER, repo-root-relative case — e.g. a consumer's own `nix fmt` /
  # `lint-fmt`, which needs `working-dir = "<deweyDir>"` to descend from the
  # tree root into deweyDir (madder, chrest: both set
  # `programs.goimports.workingDir = "go"` to match `deweyDir = "go"`, and
  # both fed the SAME config here). Composed with a tree root that is already
  # deweyDir, that descends TWICE: deweyDir + working-dir(deweyDir) =
  # "<deweyDir>/<deweyDir>", a nonexistent chdir target (confirmed
  # `chdir .../go/go: no such file or directory` in both repos). Strip any
  # `working-dir` line before feeding the config to dagnabit — the scoping it
  # provided is already accounted for by the `cd` below, so its absence here
  # correctly defaults the nested pass's tree root to deweyDir (cwd).
  sanitizeConfigForNestedPass = ''
    facadeConfig="$(mktemp)"
    trap 'rm -f "$facadeConfig"' EXIT
    sed '/^working-dir = /d' "${cfg.conformistConfig}" >"$facadeConfig"
  '';

  check = pkgs.writeShellScriptBin "conformist-dewey-facade-export" ''
    set -eu
    # cwd is the tree root; this whole-tree check takes no file arguments.
    [ -d "${deweyDir}/internal" ] || {
      echo "dewey-facade-export: no ${deweyDir}/internal — nothing to check"
      exit 0
    }

    ${ambientGuard "dagnabit not on PATH; run inside the dev shell (build dagnabit) or set dagnabitPackage"}
    # Capture the tree root before descending; it bounds dagnabit's upward
    # config walk (belt-and-suspenders alongside the explicit config below).
    root="$PWD"
    ${sanitizeConfigForNestedPass}
    cd "${deweyDir}"

    # `export --check [--library]` re-exports every facade and compares it to the
    # committed pkgs/ without writing; it exits nonzero and names the out-of-sync
    # packages on drift. DAGNABIT_CONFORMIST_CONFIG points the facade-format pass
    # at the repo's real (Nix-generated) config (purse-first#159), sanitized
    # above so an outer-scoped working-dir does not double-apply.
    if ! DAGNABIT_CONFORMIST_CONFIG="$facadeConfig" \
         DAGNABIT_CEILING_DIRECTORIES="$root" \
         ${dagnabitBin} export --check${libraryFlag}; then
      echo "dewey-facade-export: ${deweyDir}/pkgs/ is out of sync with internal/; regenerate (\`dagnabit export${libraryFlag}\` / your facade-repair recipe) and commit" >&2
      exit 1
    fi

    echo "dewey-facade-export: ${deweyDir}/pkgs/ facades are in sync with internal/"
  '';

  repair = pkgs.writeShellScriptBin "conformist-dewey-facade-export-repair" ''
    set -eu
    [ -d "${deweyDir}/internal" ] || exit 0 # nothing to export

    ${ambientGuard "dagnabit not on PATH; cannot repair"}
    root="$PWD"
    ${sanitizeConfigForNestedPass}
    cd "${deweyDir}"

    # Regenerate the facades in place (no --check). Same config threading as the
    # check half (including the working-dir sanitization) so the regenerated
    # facades are formatted identically.
    DAGNABIT_CONFORMIST_CONFIG="$facadeConfig" \
      DAGNABIT_CEILING_DIRECTORIES="$root" \
      ${dagnabitBin} export${libraryFlag}
    echo "dewey-facade-export: regenerated ${deweyDir}/pkgs/ facades"
  '';
in
{
  options.linters.dewey-facade-export = {
    enable = lib.mkEnableOption "the dewey pkgs/ facade-export drift check (dagnabit export --check [--library]; repair regenerates the facades)";

    deweyDir = lib.mkOption {
      type = lib.types.str;
      default = "libs/dewey";
      description = ''
        Path, relative to the conformist tree root, of the dewey-layout module
        root that holds internal/ (facade source) and pkgs/ (generated facades).
        purse-first: "libs/dewey"; madder: "go". dagnabit runs from here and the
        trigger-gate includes glob is "<deweyDir>/**/*.go".
      '';
    };

    library = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = ''
        Pass `--library` to `dagnabit export` (export every package under
        internal/, failing if any //go:generate dagnabit export directive exists).
        true for purse-first (no directives); set false for a directive-scan repo
        like madder, where `dagnabit export [--check]` walks //go:generate markers.
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
        invocation — the downstream consumer case (madder).
      '';
    };

    conformistConfig = lib.mkOption {
      # path (not str) so the `.#conformist-config` derivation is accepted and
      # coerced to its store path; a bare str rejects the derivation value.
      type = lib.types.path;
      description = ''
        Store path to the Nix-generated conformist config that dagnabit's
        facade-format pass runs via DAGNABIT_CONFORMIST_CONFIG, so it formats
        facades with the repo's real config instead of walking up to a stray
        ancestor conformist.toml (purse-first#159). Set to the consumer's
        `.#conformist-config` output (conformistEval.config.build.configFile) —
        the PURE formatter config (goimports/gofumpt), not the impure
        self-check config. It is safe to pass that config even when its
        formatters carry a `working-dir` matching deweyDir (the common case
        when the SAME config also drives the consumer's own `nix fmt` /
        `lint-fmt`, which needs that working-dir to descend from the tree
        root): the check/repair scripts strip any `working-dir` line before
        feeding the config to dagnabit's nested pass, since that pass's tree
        root is already deweyDir (they `cd` there first) — a passed-through
        working-dir would otherwise double-apply the descent.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    settings.linter.dewey-facade-export = {
      command = lib.getExe check;
      "repair-command" = lib.getExe repair;
      # Trigger gate: ALL Go sources under deweyDir — both internal/ (the facade
      # source) AND pkgs/ (the generated facades themselves). conformist only runs
      # a passes-files=false whole-tree linter when a file matching `includes` is
      # in scope, so this MUST cover pkgs/ too: otherwise a stale or hand-edited
      # facade (a pkgs/ change with internal/ untouched) would not trip the lane.
      # Matches the sibling dewey-reposition's broad include. The matched files are
      # only a trigger — the script reads the real internal/ → pkgs/ relationship
      # itself (and is exempt from the global excludes).
      includes = [ "${deweyDir}/**/*.go" ];
      passes-files = false;
    };
  };
}
