# lib/mkGoWorkspaceModule.nix
#
# Builds a Go package from the workspace via gomod2nix's buildGoApplication.
# All Go packages in the monorepo share the same source tree and the single
# checked-in `gomod2nix.toml` lockfile at the workspace root.
#
# `pkgs` MUST have the gomod2nix overlay applied so that `pkgs.buildGoApplication`
# is available. `gomod.nix` handles this for the self-build path.
#
# `goWorkspaceSrc` is the workspace source derivation — by convention the
# RFC 0001 `go-pkgs-test` output from `pkgs.mkGoPkgs`, so each binary's
# `checkPhase` exercises the same artifact downstream consumers receive.
#
# Usage:
#   mkGoModule = import ./lib/mkGoWorkspaceModule.nix {
#     inherit pkgs;
#     goWorkspaceSrc = (pkgs.mkGoPkgs { src = self; }).go-pkgs-test;
#   };
#   purseFirstPkg = mkGoModule {
#     pname = "purse-first";
#     subPackages = [ "cmd/purse-first" ];
#   };
{
  pkgs,
  goWorkspaceSrc,
  go ? pkgs.go,
}:

attrs:

pkgs.buildGoApplication (
  {
    inherit go;
    version = "0.1.0";
    src = goWorkspaceSrc;
    # `pwd` tells the fork where to find go.work — required so it parses the
    # workspace and merges replace maps from each child go.mod. Without this,
    # the synthetic vendor/modules.txt drops the explicit markers for direct
    # deps and `go build` rejects the vendor tree.
    pwd = goWorkspaceSrc;
    modules = goWorkspaceSrc + "/gomod2nix.toml";
  }
  // attrs
)
