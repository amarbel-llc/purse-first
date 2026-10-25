# dagnabit × Go toolchain compatibility (the Go 1.27.3 facade-export break).
#
# dagnabit loads packages through golang.org/x/tools/go/packages, which reads
# the export data the RUNTIME `go` writes (`go list -export`). That data's
# format moves with each Go release, and an x/tools older than the runtime Go
# cannot decode it: under go1.27.3, dagnabit on x/tools v0.44.0 died with
# `packages.go:1218: internal error: package "context" without types was
# imported from …/internal/0/jobwake`, halting ringmaster's facade restamp.
#
# The binary's Go/x/tools and the consumer's devShell Go are pinned separately,
# so every igloo Go bump can reopen this. One check per toolchain runs the
# PUBLISHED dagnabit (packages.dagnabit, the artifact consumers pin) on a
# fixture whose internal package imports `context`, twice:
#   1. `dagnabit export --library` — the facade generation path;
#   2. the dewey-facade-export conformist linter's own check script (the
#      lib.conformistLinters module consumers wire, dagnabitPackage pinned) —
#      the drift gate path that runs `dagnabit export --check --library`.
# `newest` tracks igloo's goToolchain.go, so a purse-first lock bump that brings
# in a Go minor this x/tools cannot read turns red HERE, before any consumer;
# `build` is the Go dagnabit itself is compiled with (the older supported Go).
{
  pkgs,
  lib,
  conformist,
  conformistBin,
  dagnabit,
  facadeConfig,
  facadeExportLinter,
  goToolchains,
}:
let
  # Instantiate the published linter module the way a consumer does, rooted at
  # the fixture module ("."), so the check exercises its real script.
  linterEval = conformist.lib.evalModule pkgs {
    imports = [ facadeExportLinter ];
    package = conformistBin;
    linters.dewey-facade-export = {
      enable = true;
      deweyDir = ".";
      library = true;
      dagnabitPackage = dagnabit;
      conformistConfig = facadeConfig;
    };
  };

  linterCheck = linterEval.config.settings.linter.dewey-facade-export.command;

  mkCheck =
    name: go:
    pkgs.runCommand "dagnabit-go-toolchain-${name}"
      {
        nativeBuildInputs = [
          go
          conformistBin
        ];
      }
      ''
        export HOME=$TMPDIR GOCACHE=$TMPDIR/gocache GOPATH=$TMPDIR/gopath \
          GOWORK=off GOTOOLCHAIN=local GOFLAGS=-mod=mod
        go version
        mkdir -p fixture/internal/0/jobwake && cd fixture
        printf 'module example.com/fixture\n\ngo 1.22\n' >go.mod
        cat >internal/0/jobwake/jobwake.go <<'EOF'
        package jobwake

        import "context"

        // Wait blocks until ctx is done.
        func Wait(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
        EOF

        DAGNABIT_CONFORMIST_CONFIG=${facadeConfig} DAGNABIT_CEILING_DIRECTORIES=$PWD \
          ${lib.getExe' dagnabit "dagnabit"} export --library
        grep -q 'var Wait = internal.Wait' pkgs/jobwake/main.go

        ${linterCheck}

        touch $out
      '';
in
lib.mapAttrs mkCheck goToolchains
