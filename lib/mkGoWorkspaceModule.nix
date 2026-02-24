# lib/mkGoWorkspaceModule.nix
#
# Builds a Go package from the workspace using `go work vendor`.
# All Go packages in the monorepo share the same source and vendor hash.
#
# Usage:
#   mkGoModule = import ./lib/mkGoWorkspaceModule.nix {
#     inherit pkgs goWorkspaceSrc goVendorHash;
#   };
#   gritPkg = mkGoModule {
#     pname = "grit";
#     subPackages = [ "packages/grit/cmd/grit" ];
#   };
{
  pkgs,
  goWorkspaceSrc,
  goVendorHash,
}:

attrs:

pkgs.buildGoModule (
  {
    version = "0.1.0";
    src = goWorkspaceSrc;
    vendorHash = goVendorHash;

    # Enable workspace mode (buildGoModule defaults to GOWORK=off)
    GOWORK = "";

    overrideModAttrs = _: _: {
      GOWORK = "";
      buildPhase = ''
        runHook preBuild
        go work vendor -e
        runHook postBuild
      '';
    };
  }
  // attrs
)
