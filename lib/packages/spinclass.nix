{
  pkgs,
  goWorkspaceSrc,
  goVendorHash,
  src, # Original source for non-Go assets (completions)
}:

let
  mkGoModule = import ../mkGoWorkspaceModule.nix {
    inherit pkgs goWorkspaceSrc goVendorHash;
  };

  spinclass = mkGoModule {
    pname = "spinclass";
    subPackages = [ "packages/spinclass/cmd/spinclass" ];
  };

  shellCompletions = pkgs.runCommand "spinclass-completions" { } ''
    install -Dm644 ${src}/completions/spinclass.bash-completion \
      $out/share/bash-completion/completions/spinclass
    install -Dm644 ${src}/completions/spinclass.fish \
      $out/share/fish/vendor_completions.d/spinclass.fish
    install -Dm644 ${src}/completions/sc.bash-completion \
      $out/share/bash-completion/completions/sc
    install -Dm644 ${src}/completions/sc.fish \
      $out/share/fish/vendor_completions.d/sc.fish
  '';
in
pkgs.symlinkJoin {
  name = "spinclass";
  paths = [
    spinclass
    shellCompletions
  ];
  postBuild = ''
    ln -s spinclass $out/bin/sc
  '';
}
