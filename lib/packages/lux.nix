# Workspace build: uses the full Go monorepo source with `go work vendor`.
# The vendorHash only covers external dependencies — workspace modules
# (go-mcp, etc.) stay in-tree, so local code changes never invalidate it.
{
  pkgs,
  goWorkspaceSrc,
  goVendorHash,
}:

pkgs.buildGoModule {
  pname = "lux";
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

  subPackages = [ "packages/lux/cmd/lux" ];

  nativeBuildInputs = [ pkgs.scdoc ];

  ldflags = [ "-X main.version=0.1.0" ];

  postInstall = ''
    $out/bin/lux _generate $out

    mkdir -p $out/share/man/man5
    scdoc < ${goWorkspaceSrc}/packages/lux/doc/lux-config.5.scd > $out/share/man/man5/lux-config.5
  '';

  meta = with pkgs.lib; {
    description = "LSP Multiplexer that routes requests to language servers based on file type";
    homepage = "https://github.com/amarbel-llc/lux";
    license = licenses.mit;
  };
}
