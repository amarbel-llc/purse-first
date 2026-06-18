# A correct reimplementation of conformist's justfile-default linter, authored
# in purse-first because conformist's version (nix/linters/justfile-default.nix
# at conformist a8471278) is buggy: it awk-sniffs raw justfile text and decides
# a `default` dependency is a "leaf with a body" by checking whether the next
# PHYSICAL line is indented. A backslash-continued aggregate —
#
#     test: \
#         test-go \
#         ...
#
# — has an indented continuation line, so the awk misclassifies the pure
# aggregate `test` as a leaf and false-positives. conformist's sibling
# justfile-task-hierarchy already does the body-vs-dependencies determination
# correctly via `just --dump --dump-format json`; this module uses the same
# native plumbing for that part.
#
# conformist-justfile(7) AGGREGATES AND LEAVES: `default` must be the FIRST
# recipe, and it must list only aggregate targets (recipes with no body) — never
# leaves directly. Whole-tree check (passes-files=false), takes no file
# arguments. BOTH halves use `just --dump --dump-format json`: the top-level
# `.first` field gives the first recipe by source order, and per-recipe `.body`
# distinguishes aggregates (empty body) from leaves — no raw-text heuristics at
# all, so the continuation bug cannot arise.
#
# Temporary name `justfile-default-native` avoids colliding with presets.eng's
# `options.linters.justfile-default` (the module system forbids two declarations
# of the same option). Once conformist drops its broken justfile-default, this
# reclaims the canonical `justfile-default` name. See purse-first task #8.
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.linters.justfile-default-native;

  check = pkgs.writeShellApplication {
    name = "conformist-justfile-default-native";
    runtimeInputs = with pkgs; [
      coreutils
      jq
      just
    ];
    text = ''
      [ -f justfile ] || {
        echo "justfile-default-native: justfile missing at tree root" >&2
        exit 1
      }

      fail=0

      # Both checks read just's NATIVE parsed model (`just --dump --dump-format
      # json`), which exposes both `.first` (the first recipe by source order)
      # and per-recipe `.body` — so neither half needs raw-text heuristics:
      #   - "default must be FIRST": compare top-level `.first` to "default".
      #   - "default lists only aggregates": each default dependency must be an
      #     aggregate (empty `.body`). just parses a backslash-continued
      #     dependency list as dependencies with an EMPTY body, so multi-line
      #     aggregates are correctly classified — the fix over conformist's
      #     raw-text indent-sniffing, which misreads the indented continuation
      #     line as a body.
      # $root/$dep/$r are jq bindings, not shell vars; keep them literal in the
      # single-quoted program.
      # shellcheck disable=SC2016
      filter='. as $root
        | (
            if $root.first != "default" then
              "the first recipe must be \"default\" (found: \"\($root.first // "none")\")"
            else empty end
          ),
          (
            ($root.recipes.default.dependencies // [])[]
            | .recipe as $dep
            | ($root.recipes[$dep]) as $r
            | select($r != null and (($r.body | length) > 0))
            | "\"default\" lists leaf recipe \"\($dep)\" (it has a body); default must contain only aggregate targets"
          )'

      # Capture (not process substitution) so a just/jq failure aborts loudly
      # rather than yielding an empty stream that reads as "no findings" — a
      # check must never pass vacuously on its own parse error.
      if ! offenders=$(just --dump --dump-format json | jq -r "$filter"); then
        echo "justfile-default-native: failed to read recipes via just/jq" >&2
        exit 2
      fi

      while read -r line; do
        [ -n "$line" ] || continue
        echo "justfile-default-native: $line (conformist-justfile(7) AGGREGATES AND LEAVES)" >&2
        fail=1
      done <<< "$offenders"

      [ "$fail" -eq 0 ] || exit 1
      echo "justfile-default-native: 'default' is the first recipe and lists only aggregates"
    '';
  };
in
{
  options.linters.justfile-default-native = {
    enable = lib.mkEnableOption "the 'default is first + aggregates-only' whole-tree check (native just --dump plumbing; conformist-justfile(7))";
  };

  config = lib.mkIf cfg.enable {
    settings.linter.justfile-default-native = {
      command = lib.getExe check;
      includes = [ "justfile" ];
      passes-files = false;
    };
  };
}
