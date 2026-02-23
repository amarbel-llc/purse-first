{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/6d41bc27aaf7b6a3ba6b169db3bd5d6159cfaa47";
    nixpkgs-master.url = "github:NixOS/nixpkgs/5b7e21f22978c4b740b3907f3251b470f466a9a2";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      gomod2nix,
      nixpkgs-master,
    }:
    {
      overlays = gomod2nix.overlays;
    }
    // (utils.lib.eachDefaultSystem (
      system:
      let

        pkgs = import nixpkgs {
          inherit system;
        };

        pkgs-master = import nixpkgs-master {
          inherit system;
        };

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

          # gopls = gopls.packages.${system}.default;
          gomod2nix = gomod2nix.packages.${system}.default;
        };

      in

      {
        inherit packages;

        devShells.default = pkgs-master.mkShell {
          packages = builtins.attrValues packages;

          env = {
            GOPATH = "$HOME/.cache/go";
          };
        };
      }
    ));
}
