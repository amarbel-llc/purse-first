# purse-first's IMPURE conformist config: whole-tree checks that need the live
# working tree + host tools (the Go module cache / `go list`, an ambient
# `dagnabit`) and therefore CANNOT run in the sandboxed checks.formatting (which
# sees only a read-only /nix/store copy of tracked files, no network/module
# cache). Consumed by `just lint-worktree`, which runs `conformist check`
# against the working tree. `package` is injected by flake.nix
# (conformistImpureEval). Mirrors conformist's own nix/conformist-impure.nix.
{ ... }:
{
  projectRootFile = "flake.nix";

  # dewey NATO-level reposition drift (purse-first#160): `dagnabit reposition -n`
  # shells out to `go list`, so it's impure. The linter module is imported in
  # flake.nix's conformistImpureEval. The standard eng-impure git-state roster
  # (git-remotes / sweatfile / agents-md) is intentionally NOT pulled in here —
  # this lane exists for the dewey reposition check; add presets.eng-impure
  # later if purse-first wants those too.
  linters.dewey-reposition.enable = true;

  # dewey pkgs/ facade-export drift (purse-first#163): `dagnabit export --check
  # --library` shells out to `go list` + `conformist`, so it's impure too. Its
  # `conformistConfig` option (the store path to the PURE formatter config) is
  # set from flake.nix, where conformistEval is in scope. Subsumes the standalone
  # read-only facade-drift recipe (now the debug-dewey-pkgs-drift escape hatch)
  # at the merge gate.
  linters.dewey-facade-export.enable = true;
}
