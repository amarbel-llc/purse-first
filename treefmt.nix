# treefmt-nix configuration. Run via `nix fmt`.
{ lib, ... }:
{
  projectRootFile = "flake.nix";

  # Go: goimports → gofumpt chain. Lower priority runs first; goimports must
  # run before gofumpt so the import-grouped output is then re-canonicalized
  # by gofumpt. gofumpt's grouping rule (consecutive same-kind decls fold
  # into a shared block) is what dagnabit export relies on for stable output.
  programs.goimports.enable = true;
  settings.formatter.goimports.priority = 1;
  programs.gofumpt.enable = true;
  settings.formatter.gofumpt.priority = 2;

  programs.nixfmt.enable = true;

  programs.shfmt.enable = true;
  settings.formatter.shfmt.includes = [
    "*.sh"
    "*.bash"
    "*.bats"
  ];
  # treefmt-nix's shfmt module exposes `indent_size` and `simplify` but
  # not `--case-indent` (-ci). Override the full options list to keep
  # those flags AND add -ci so `case` branches stay indented one level
  # past the `case` keyword (project style).
  settings.formatter.shfmt.options = lib.mkForce [
    "-i"
    "2"
    "-s"
    "-ci"
  ];

  settings.global.excludes = [
    "flake.lock"
    "go.sum"
    "gomod2nix.toml"
    "version.env"
    "LICENSE"
    "*.md"
    "result"
    "result-*"
    ".tmp/**"
    # Templates ship as scaffolding for downstream `nix flake init`;
    # formatting them would invalidate `nix-instantiate --parse` test
    # snapshots that expect specific layout.
    "templates/**"
  ];
}
