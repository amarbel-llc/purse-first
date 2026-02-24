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
  pname = "grit";
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
