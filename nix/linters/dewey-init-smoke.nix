# dewey per-arch init-smoke drift as a conformist whole-tree linter
# (purse-first#180 / FDR 0014).
#
# `dagnabit init-smoke` regenerates the per-arch blank-import tests in
# <deweyDir>/initsmoke/ from the live package graph: for each arch declared in
# <deweyDir>/dagnabit.toml it blank-imports every package that BUILDS for that
# arch, so instantiating the test binary runs every package's init() and a
# per-arch init hazard (purse-first#177) surfaces at load time. Adding or
# removing a package — or changing what builds for an arch — silently staleness
# the committed import list unless it is regenerated. This linter mechanizes the
# check: the read-only `command` runs `dagnabit init-smoke --check` (no writes;
# nonzero + names the out-of-sync arch files on drift); the `repair-command`
# runs `dagnabit init-smoke` to regenerate them in place. Mirrors
# nix/linters/dewey-facade-export.nix.
#
# REUSABLE: published from purse-first's flake as
# `lib.conformistLinters.dewey-init-smoke` so other dewey-layout repos can
# import it. Parameterized by:
#   - deweyDir        — the module root holding dagnabit.toml + initsmoke/
#                       ("libs/dewey" for purse-first, "go" for madder)
#   - dagnabitPackage — null ⇒ ambient PATH dagnabit (purse-first self-test); a
#                       pinned package ⇒ hermetic, PATH-independent invocation
#   - conformistConfig — the PURE formatter config dagnabit's init-smoke format
#                        pass runs (generated files are conformist-formatted so
#                        both the pure lint gate and this drift check stay green)
#
# IMPURE: `dagnabit init-smoke` type-checks the package graph under each arch
# (golang.org/x/tools/go/packages, needing the Go module cache) and shells to
# `conformist` for formatting, so it cannot run in the sandboxed
# checks.formatting. It lives in the impure self-check lane
# (nix/conformist-impure.nix), run via `just lint-worktree` — the same
# constraint that puts dewey-reposition and dewey-facade-export there.
#
# CONFIG THREADING: identical to dewey-facade-export. A repo with NO
# conformist.toml on disk (Nix-generated config) points dagnabit at the
# generated config via DAGNABIT_CONFORMIST_CONFIG so its init-smoke format pass
# does not walk up to a stray ancestor conformist.toml (purse-first#159); any
# `working-dir` line is stripped first (sanitizeConfigForNestedPass), because
# that pass's tree root is already deweyDir (both scripts `cd` there first) so a
# working-dir matching deweyDir would double the descent.
#
# writeShellScriptBin (not writeShellApplication) so the script inherits the
# caller's PATH, where an ambient `dagnabit`/`go` resolve when dagnabitPackage is
# null (mirrors dewey-facade-export).
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.linters.dewey-init-smoke;

  deweyDir = cfg.deweyDir;

  dagnabitBin =
    if cfg.dagnabitPackage != null then "${cfg.dagnabitPackage}/bin/dagnabit" else "dagnabit";

  ambientGuard =
    failMsg:
    lib.optionalString (cfg.dagnabitPackage == null) ''
      if ! command -v dagnabit >/dev/null 2>&1; then
        echo "dewey-init-smoke: ${failMsg}" >&2
        exit 1
      fi
    '';

  # See dewey-facade-export.nix for the full rationale: dagnabit's format pass
  # runs with its tree root ALREADY at deweyDir, so a working-dir key scoping a
  # formatter from the repo-root case would descend twice. Strip it.
  sanitizeConfigForNestedPass = ''
    initSmokeConfig="$(mktemp)"
    trap 'rm -f "$initSmokeConfig"' EXIT
    sed '/^working-dir = /d' "${cfg.conformistConfig}" >"$initSmokeConfig"
  '';

  check = pkgs.writeShellScriptBin "conformist-dewey-init-smoke" ''
    set -eu
    # cwd is the tree root; this whole-tree check takes no file arguments.
    [ -f "${deweyDir}/dagnabit.toml" ] || {
      echo "dewey-init-smoke: no ${deweyDir}/dagnabit.toml — nothing to check"
      exit 0
    }

    ${ambientGuard "dagnabit not on PATH; run inside the dev shell (build dagnabit) or set dagnabitPackage"}
    root="$PWD"
    ${sanitizeConfigForNestedPass}
    cd "${deweyDir}"

    if ! DAGNABIT_CONFORMIST_CONFIG="$initSmokeConfig" \
         DAGNABIT_CEILING_DIRECTORIES="$root" \
         ${dagnabitBin} init-smoke --check; then
      echo "dewey-init-smoke: ${deweyDir}/initsmoke/ is out of sync with the package graph; regenerate (\`dagnabit init-smoke\` / your init-smoke-repair recipe) and commit" >&2
      exit 1
    fi

    echo "dewey-init-smoke: ${deweyDir}/initsmoke/ is in sync with the package graph"
  '';

  repair = pkgs.writeShellScriptBin "conformist-dewey-init-smoke-repair" ''
    set -eu
    [ -f "${deweyDir}/dagnabit.toml" ] || exit 0 # nothing to generate

    ${ambientGuard "dagnabit not on PATH; cannot repair"}
    root="$PWD"
    ${sanitizeConfigForNestedPass}
    cd "${deweyDir}"

    DAGNABIT_CONFORMIST_CONFIG="$initSmokeConfig" \
      DAGNABIT_CEILING_DIRECTORIES="$root" \
      ${dagnabitBin} init-smoke
    echo "dewey-init-smoke: regenerated ${deweyDir}/initsmoke/ per-arch tests"
  '';
in
{
  options.linters.dewey-init-smoke = {
    enable = lib.mkEnableOption "the dewey per-arch init-smoke drift check (dagnabit init-smoke --check; repair regenerates the per-arch tests)";

    deweyDir = lib.mkOption {
      type = lib.types.str;
      default = "libs/dewey";
      description = ''
        Path, relative to the conformist tree root, of the dewey-layout module
        root that holds dagnabit.toml (the init-smoke arch config) and initsmoke/
        (the generated per-arch tests). purse-first: "libs/dewey"; madder: "go".
        dagnabit runs from here and the trigger-gate includes glob is
        "<deweyDir>/**/*.go" plus "<deweyDir>/dagnabit.toml".
      '';
    };

    dagnabitPackage = lib.mkOption {
      type = lib.types.nullOr lib.types.package;
      default = null;
      description = ''
        The dagnabit package whose binary the scripts invoke. null ⇒ resolve
        `dagnabit` from the ambient PATH (purse-first self-tests its working-tree
        build, placed on PATH by `just lint-worktree`). Set to a pinned package
        for a hermetic, PATH-independent invocation — the downstream consumer
        case (madder).
      '';
    };

    conformistConfig = lib.mkOption {
      type = lib.types.path;
      description = ''
        Store path to the Nix-generated PURE conformist config that dagnabit's
        init-smoke format pass runs via DAGNABIT_CONFORMIST_CONFIG, so it formats
        the generated tests with the repo's real config instead of walking up to
        a stray ancestor conformist.toml (purse-first#159). Set to the consumer's
        `.#conformist-config` output. Any `working-dir` line is stripped before
        feeding it to dagnabit's nested pass, since that pass's tree root is
        already deweyDir.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    settings.linter.dewey-init-smoke = {
      command = lib.getExe check;
      "repair-command" = lib.getExe repair;
      # Trigger gate: ALL Go sources under deweyDir (a change to any package can
      # change what builds for an arch, i.e. the generated import set) PLUS the
      # arch config itself. conformist only runs a passes-files=false whole-tree
      # linter when a file matching `includes` is in scope, so both must be
      # covered. The script reads the real graph → initsmoke/ relationship
      # itself; the matched files are only a trigger.
      includes = [
        "${deweyDir}/**/*.go"
        "${deweyDir}/dagnabit.toml"
      ];
      passes-files = false;
    };
  };
}
