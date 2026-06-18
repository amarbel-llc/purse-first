# purse-first's conformist config, consumed by conformist.lib.evalModule in
# flake.nix (the eng-convention preset is imported alongside this module there).
# This replaces the former hand-written conformist.toml: purse-first now consumes
# conformist's Nix module library instead of a standalone TOML config.
#
# Drives `nix fmt` (repair, via build.wrapper → `just codemod-fmt-conformist`)
# and `nix build .#checks.<sys>.formatting` (read-only `conformist check`, via
# build.check → `just lint-conformist`). `package` is injected by flake.nix.
#
# See conformist's own nix/conformist.nix for the reference shape and the
# `nix flake init -t github:amarbel-llc/conformist#eng` template.
{ ... }:
{
  projectRootFile = "flake.nix";

  # Go formatter chain: goimports (priority 1) MUST run before gofumpt
  # (priority 2) so the import-grouped output is re-canonicalized by gofumpt —
  # the grouping dagnabit's `export` relies on for stable facades. conformist
  # batches a file's formatters by ascending priority, so this reproduces the
  # ordered chain the old conformist.toml encoded.
  programs.goimports.enable = true;
  programs.goimports.priority = 1;
  programs.gofumpt.enable = true;
  programs.gofumpt.priority = 2;

  programs.nixfmt.enable = true;

  # Project shell style: 2-space indent, simplify, switch-case indent.
  # programs.shfmt emits `-i 2 -s -ci` by default (indent_size 2, simplify true,
  # caseIndent true as of conformist 1b8e32d — the option that retired the old
  # `settings.formatter.shfmt.options = lib.mkAfter ["-ci"]` augmentation).
  programs.shfmt.enable = true;

  # Linter: shellcheck over the justfile recipes' shell and any *.sh/*.bash/*.bats.
  linters.shellcheck.enable = true;

  # justfile-default comes from presets.eng — conformist fixed its
  # backslash-continued-aggregate false positive upstream (587cabc, then 1b8e32d)
  # by switching to `just --dump --dump-format json` plumbing, so purse-first no
  # longer needs a local replacement.

  # NOTE: do NOT enable linters.golangci-dewey. purse-first's lint-go is
  # `go vet ./...` and there is no root .golangci.{yml,yaml}; that linter is
  # whole-tree tree-root and would hit its "repo does not gate on golangci-lint;
  # nothing to check" branch — a guaranteed exit-0 no-op. purse-first runs the
  # dewey custom binary only inside libs/dewey (lint-dewey-self, via
  # libs/dewey/.golangci.yml found by walk-up), not via a root golangci gate.
  # Enabling a guaranteed no-op linter would misrepresent intent.

  # Per-file formatter excludes: helper_test.go uses a deliberate `stmt; stmt`
  # one-liner that goimports/gofumpt would normalise to newlines, breaking a
  # line-number invariant the operation tests depend on (ported from the old
  # conformist.toml per-formatter excludes).
  settings.formatter.goimports.excludes = [ "libs/go-mcp/operation/helper_test.go" ];
  settings.formatter.gofumpt.excludes = [ "libs/go-mcp/operation/helper_test.go" ];

  # Generated / locked / prose — not formatted (ported from conformist.toml).
  # The eng-convention whole-tree linters from presets.eng (eng-versioning,
  # flake-outputs, flake-lock, justfile-*) are exempt from these excludes by
  # design (their includes are a trigger gate, not an input set), so version.env
  # / flake.lock are still checked.
  settings.excludes = [
    "flake.lock"
    "go.sum"
    "go.work.sum"
    "gomod2nix.toml"
    "version.env"
    "LICENSE"
    "*.md"
    "result"
    "result-*"
    ".tmp/**"
    # Templates ship as scaffolding for downstream `nix flake init`; formatting
    # them would invalidate nix-instantiate --parse test snapshots.
    "templates/**"
  ];
}
