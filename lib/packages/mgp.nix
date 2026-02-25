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
  pname = "mgp";
  subPackages = [ "packages/mgp/cmd/mgp" ];

  postInstall = ''
    $out/bin/mgp generate-plugin $out
  '';

  meta = with pkgs.lib; {
    description = "Model graph protocol — query and execute MCP tools via GraphQL";
    homepage = "https://github.com/amarbel-llc/purse-first";
    license = licenses.mit;
  };
}
