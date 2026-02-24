# Workspace build: uses the full Go monorepo source with `go work vendor`.
# The vendorHash only covers external dependencies — workspace modules
# (go-mcp, etc.) stay in-tree, so local code changes never invalidate it.
{
  pkgs,
  goWorkspaceSrc,
  goVendorHash,
}:

pkgs.buildGoModule {
  pname = "potato";
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

  subPackages = [ "packages/potato/cmd/potato" ];

  meta = with pkgs.lib; {
    description = "pomodoro timer that requires the potato to rest for 5 minutes";
    homepage = "https://github.com/friedenberg/potato";
    license = licenses.mit;
  };
}
