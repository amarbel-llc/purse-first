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
{ lib, ... }:
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
  # programs.shfmt contributes `-i 2 -s` (indent_size defaults to 2, simplify
  # defaults to true); it does NOT expose `-ci`, so append it via raw settings
  # (mkAfter so `-ci` lands after the program's `-i 2 -s`). A first-class
  # programs.shfmt.caseIndent option is a tracked conformist followup; swap to
  # it once available and drop this augmentation.
  programs.shfmt.enable = true;
  settings.formatter.shfmt.options = lib.mkAfter [ "-ci" ];

  # Linter: shellcheck over the justfile recipes' shell and any *.sh/*.bash/*.bats.
  linters.shellcheck.enable = true;

  # justfile-default: use purse-first's CORRECT implementation (native
  # `just --dump` plumbing) and force OFF presets.eng's buggy one, which
  # awk-sniffs raw text and false-positives on backslash-continued aggregates.
  # mkForce overrides the preset's `enable = true`. The native module is imported
  # in flake.nix's evalModule. Reclaim the canonical name once conformist drops
  # its version (purse-first task #8 / conformist drop request).
  linters.justfile-default.enable = lib.mkForce false;
  linters.justfile-default-native.enable = true;

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
