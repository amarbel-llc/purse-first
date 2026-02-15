# Go MCP server with purse-first plugin generation via postInstall.
# Uses buildGoApplication with gomod2nix overlay.
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
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
          overlays = [
            go.overlays.default
          ];
        };

        version = "0.1.0";

        my-mcp = pkgs.buildGoApplication {
          pname = "my-mcp";
          inherit version;
          src = ./.;
          modules = ./gomod2nix.toml;
          subPackages = [ "cmd/my-mcp" ];

          postInstall = ''
            $out/bin/my-mcp generate-plugin $out/share/purse-first
          '';

          meta = with pkgs.lib; {
            description = "My MCP server description";
            homepage = "https://github.com/owner/my-mcp";
            license = licenses.mit;
          };
        };
      in
      {
        packages = {
          default = my-mcp;
          inherit my-mcp;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [ just ];

          inputsFrom = [
            go.devShells.${system}.default
            shell.devShells.${system}.default
          ];
        };
      }
    );
}
