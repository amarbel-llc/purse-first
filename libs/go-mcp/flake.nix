{
  description = "MCP (Model Context Protocol) library for Go";

  inputs = {
    nixpkgs-master.url = "github:NixOS/nixpkgs/b28c4999ed71543e71552ccfd0d7e68c581ba7e9";
    nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    go.url = "github:amarbel-llc/eng?dir=devenvs/go";
    shell.url = "github:amarbel-llc/eng?dir=devenvs/shell";
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      go,
      shell,
      nixpkgs-master,
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

        go-lib-mcp = pkgs.buildGoModule {
          pname = "go-lib-mcp";
          inherit version;
          src = ./.;
          vendorHash = null;  # Library with no dependencies

          meta = with pkgs.lib; {
            description = "MCP (Model Context Protocol) library for building MCP servers in Go";
            homepage = "https://github.com/amarbel-llc/go-lib-mcp";
            license = licenses.mit;
          };
        };
      in
      {
        packages = {
          default = go-lib-mcp;
          inherit go-lib-mcp;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            just
            golangci-lint
          ];

          inputsFrom = [
            go.devShells.${system}.default
            shell.devShells.${system}.default
          ];

          shellHook = ''
            echo "go-lib-mcp: MCP library for Go - dev environment"
          '';
        };
      }
    );
}
