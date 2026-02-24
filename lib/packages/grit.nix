# Workspace build: uses the full Go monorepo source with `go work vendor`.
# The vendorHash only covers external dependencies — workspace modules
# (go-mcp, etc.) stay in-tree, so local code changes never invalidate it.
{
  pkgs,
  goWorkspaceSrc,
  goVendorHash,
}:

pkgs.buildGoModule {
  pname = "grit";
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

  subPackages = [ "packages/grit/cmd/grit" ];

  postInstall = ''
    $out/bin/grit generate-plugin $out
  '';

  meta = with pkgs.lib; {
    description = "MCP for git, wow that's grit";
    homepage = "https://github.com/amarbel-llc/grit";
    license = licenses.mit;
  };
}
