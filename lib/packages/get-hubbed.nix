# Workspace build: uses the full Go monorepo source with `go work vendor`.
# The vendorHash only covers external dependencies — workspace modules
# (go-mcp, etc.) stay in-tree, so local code changes never invalidate it.
{
  pkgs,
  goWorkspaceSrc,
  goVendorHash,
}:

pkgs.buildGoModule {
  pname = "get-hubbed";
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

  subPackages = [ "packages/get-hubbed/cmd/get-hubbed" ];

  postInstall = ''
    $out/bin/get-hubbed generate-plugin $out
  '';

  meta = with pkgs.lib; {
    description = "`gh` cli wrapper with MCP support packaged by nix";
    homepage = "https://github.com/amarbel-llc/get-hubbed";
    license = licenses.mit;
  };
}
