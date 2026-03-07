{
  pkgs,
  goWorkspaceSrc,
  goVendorHash,
}:

let
  mkGoModule = import ../mkGoWorkspaceModule.nix {
    inherit pkgs goWorkspaceSrc goVendorHash;
  };
in
mkGoModule {
  pname = "lux";
  subPackages = [ "packages/lux/cmd/lux" ];

  nativeBuildInputs = [ pkgs.scdoc ];

  ldflags = [ "-X main.version=0.1.0" ];

  postInstall = ''
    $out/bin/lux generate-plugin $out

    mkdir -p $out/share/man/man5
    scdoc < ${goWorkspaceSrc}/packages/lux/doc/lux-config.5.scd > $out/share/man/man5/lux-config.5
  '';

  meta = with pkgs.lib; {
    description = "LSP Multiplexer that routes requests to language servers based on file type";
    homepage = "https://github.com/amarbel-llc/lux";
    license = licenses.mit;
  };
}
