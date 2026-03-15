# devenvs/go/default.nix
#
# Args:
#   pkgs        — stable nixpkgs
#   pkgs-master — unstable nixpkgs (for latest tooling)
#   gomod2nix   — the gomod2nix flake (for overlay + CLI package)
#
{
  pkgs,
  pkgs-master,
  gomod2nix,
}:
let
  packages = {
    inherit (pkgs-master)
      delve
      gofumpt
      golangci-lint
      golines
      gopls
      gotools
      govulncheck
      parallel
      ;

    inherit (pkgs)
      go
      ;

    gomod2nix = gomod2nix.packages.${pkgs.stdenv.hostPlatform.system}.default;
  };
in
{
  overlay = gomod2nix.overlays.default;
  inherit packages;

  devShell = pkgs-master.mkShell {
    packages = builtins.attrValues packages;

  };
}
