{
  pkgs,
  src,
  goOverlay,
}:

let
  goPkgs = import pkgs.path {
    inherit (pkgs.stdenv.hostPlatform) system;
    overlays = [ goOverlay ];
  };
in
goPkgs.buildGoApplication {
  pname = "potato";
  version = "0.1.0";
  inherit src;
  modules = "${src}/gomod2nix.toml";
  subPackages = [ "cmd/potato" ];

  meta = with pkgs.lib; {
    description = "pomodoro timer that requires the potato to rest for 5 minutes";
    homepage = "https://github.com/friedenberg/potato";
    license = licenses.mit;
  };
}
