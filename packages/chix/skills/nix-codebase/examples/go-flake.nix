{
  description = "Go project with buildGoModule";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    go.url = "github:amarbel-llc/purse-first?dir=devenvs/go";
    shell.url = "github:amarbel-llc/purse-first?dir=devenvs/shell";
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
        pkgs = import nixpkgs { inherit system; };

        version = "0.1.0";

        myApp = pkgs.buildGoModule {
          pname = "my-app";
          inherit version;
          src = ./.;
          vendorHash = "<sha256-hash>";
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
