{
  description = "A Nix-flake-based BATS development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/6d41bc27aaf7b6a3ba6b169db3bd5d6159cfaa47";
    nixpkgs-master.url = "github:NixOS/nixpkgs/5b7e21f22978c4b740b3907f3251b470f466a9a2";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      nixpkgs-master,
    }:
    (utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };

      in

      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            bats
            parallel
            shellcheck
            shfmt
            # TODO: add bats.libraries.bats-support and bats.libraries.bats-assert
          ];
        };
      }
    ));
}
