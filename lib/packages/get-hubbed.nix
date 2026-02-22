{
  pkgs,
  src,
  goOverlay,
  purse-first-src,
}:

let
  goPkgs = import pkgs.path {
    inherit (pkgs) system;
    overlays = [ goOverlay ];
  };

  # get-hubbed needs the purse-first source for its replace directive in go.mod
  get-hubbed-src = pkgs.runCommand "get-hubbed-src" { } ''
    cp -r ${src} $out
    chmod -R u+w $out
    rm -f $out/go.work $out/go.work.sum
    mkdir -p $out/deps
    cp -r ${purse-first-src} $out/deps/purse-first
  '';
in
goPkgs.buildGoApplication {
  pname = "get-hubbed";
  version = "0.1.0";
  pwd = get-hubbed-src;
  src = get-hubbed-src;
  modules = "${src}/gomod2nix.toml";
  subPackages = [ "cmd/get-hubbed" ];

  postInstall = ''
    $out/bin/get-hubbed generate-plugin $out/share/purse-first
  '';

  meta = with pkgs.lib; {
    description = "`gh` cli wrapper with MCP support packaged by nix";
    homepage = "https://github.com/amarbel-llc/get-hubbed";
    license = licenses.mit;
  };
}
