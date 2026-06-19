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
}
