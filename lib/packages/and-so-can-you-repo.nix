{ pkgs, src }:

let
  name = "and-so-can-you-repo";
  script =
    (pkgs.writeScriptBin name (builtins.readFile "${src}/bin/and-so-can-you-repo.bash")).overrideAttrs
      (old: {
        buildCommand = "${old.buildCommand}\n patchShebangs $out";
      });
  buildInputs = with pkgs; [
    gum
    gh
  ];
in
pkgs.symlinkJoin {
  inherit name;
  paths = [ script ] ++ buildInputs;
  buildInputs = [ pkgs.makeWrapper ];
  postBuild = "wrapProgram $out/bin/${name} --prefix PATH : $out/bin";
}
