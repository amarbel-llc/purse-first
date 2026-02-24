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
  pname = "potato";
  subPackages = [ "packages/potato/cmd/potato" ];

  meta = with pkgs.lib; {
    description = "pomodoro timer that requires the potato to rest for 5 minutes";
    homepage = "https://github.com/friedenberg/potato";
    license = licenses.mit;
  };
}
