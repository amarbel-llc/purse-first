# dagnabit's facade config: the conformist config dagnabit's own format pass
# runs over the files it generates (pkgs/ facades, initsmoke/ tests) via
# DAGNABIT_CONFORMIST_CONFIG.
#
# Generated files belong to the generator: a repo's own conformist config
# excludes them, and dagnabit formats them with THIS config instead, so the
# committed bytes are a function of the dagnabit version and this config alone.
# See dagnabit(1) ENVIRONMENT for the contract dagnabit enforces on it:
#   - formatters only — no [linter.*] (whole-tree linters such as codegen-repair
#     would run at dagnabit's tree root; purse-first#195),
#   - no excludes (they would hide the very files dagnabit is formatting),
#   - no working-dir (dagnabit's tree root is the module root; a repo-root
#     working-dir such as "go" would descend a second time).
#
# Published as `lib.conformistModules.dagnabit-facade`. A consumer evaluates it
# with its own conformist and nixpkgs pins:
#
#   facadeEval = conformist.lib.evalModule pkgs {
#     imports = [ purse-first.lib.conformistModules.dagnabit-facade ];
#     package = conformist.packages.${system}.default;
#   };
#   # DAGNABIT_CONFORMIST_CONFIG = facadeEval.config.build.configFile
{ ... }:
{
  # goimports (priority 1) before gofumpt (priority 2): gofumpt re-canonicalizes
  # the import-grouped output, matching the eng Go formatter chain.
  programs.goimports.enable = true;
  programs.goimports.priority = 1;
  programs.gofumpt.enable = true;
  programs.gofumpt.priority = 2;
}
