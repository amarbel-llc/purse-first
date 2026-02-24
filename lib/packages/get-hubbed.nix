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
  pname = "get-hubbed";
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
