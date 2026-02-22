{
  description = "Go project with gomod2nix";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
    nixpkgs-master.url = "github:NixOS/nixpkgs/b28c4999ed71543e71552ccfd0d7e68c581ba7e9";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    go.url = "github:friedenberg/eng?dir=devenvs/go";
    shell.url = "github:friedenberg/eng?dir=devenvs/shell";
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-master,
      utils,
      go,
      shell,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ go.overlays.default ];
        };

        version = "0.1.0";

        myApp = pkgs.buildGoApplication {
          pname = "my-app";
          inherit version;
          src = ./.;
          modules = ./gomod2nix.toml;
          subPackages = [ "cmd/my-app" ];

          ldflags = [
            "-X main.version=${version}"
          ];

          meta = with pkgs.lib; {
            description = "My application";
            homepage = "https://github.com/amarbel-llc/my-app";
            license = licenses.mit;
          };
        };
      in
      {
        packages.default = myApp;

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            just
            gum
          ];

          inputsFrom = [
            go.devShells.${system}.default
            shell.devShells.${system}.default
          ];
        };

        apps.default = {
          type = "app";
          program = "${myApp}/bin/my-app";
        };
      }
    );
}
