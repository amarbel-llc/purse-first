{
  description = "A Nix-flake-based Nix development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/6d41bc27aaf7b6a3ba6b169db3bd5d6159cfaa47";
    nixpkgs-master.url = "github:NixOS/nixpkgs/5b7e21f22978c4b740b3907f3251b470f466a9a2";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    fh.url = "https://flakehub.com/f/DeterminateSystems/fh/0.1.21.tar.gz";
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-master,
      utils,
      fh,
    }:
    (utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };

        packages = {
          inherit (pkgs)
            parallel
            nil
            nixfmt
            ;

          fh = fh.packages.${system}.default;
        };

      in

      {
        formatter = pkgs.nixfmt;

        apps.fmt = {
          type = "app";
          program = "${pkgs.nixfmt}/bin/nixfmt";
        };

        packages.default = pkgs.symlinkJoin {
          name = "devenv-nix";
          paths = builtins.attrValues packages;
        };

        devShells.default = pkgs.mkShell {
          packages = builtins.attrValues packages;
        };
      }
    ));
}
