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

  spinclass = goPkgs.buildGoApplication {
    pname = "spinclass";
    version = "0.1.0";
    inherit src;
    modules = "${src}/gomod2nix.toml";
    subPackages = [ "cmd/spinclass" ];
  };

  shellCompletions = pkgs.runCommand "spinclass-completions" { } ''
    install -Dm644 ${src}/completions/spinclass.bash-completion \
      $out/share/bash-completion/completions/spinclass
    install -Dm644 ${src}/completions/spinclass.fish \
      $out/share/fish/vendor_completions.d/spinclass.fish
  '';
in
pkgs.symlinkJoin {
  name = "spinclass";
  paths = [
    spinclass
    shellCompletions
  ];
}
