{
  pkgs,
  src,
  goOverlay,
}:

let
  goPkgs = import pkgs.path {
    inherit (pkgs) system;
    overlays = [ goOverlay ];
  };
in
goPkgs.buildGoApplication {
  pname = "grit";
  version = "0.1.0";
  inherit src;
  modules = "${src}/gomod2nix.toml";
  subPackages = [ "cmd/grit" ];

  postInstall = ''
    $out/bin/grit generate-plugin $out
  '';

  meta = with pkgs.lib; {
    description = "MCP for git, wow that's grit";
    homepage = "https://github.com/amarbel-llc/grit";
    license = licenses.mit;
  };
}
