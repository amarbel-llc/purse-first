{
  description = "Claude Plugin Marketplace: MCP servers and tool routing for Claude Code";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
    nixpkgs-master.url = "github:NixOS/nixpkgs/b28c4999ed71543e71552ccfd0d7e68c581ba7e9";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    go.url = "github:friedenberg/eng?dir=devenvs/go";
    shell.url = "github:friedenberg/eng?dir=devenvs/shell";

    grit = {
      url = "github:amarbel-llc/grit";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };
    get-hubbed = {
      url = "github:amarbel-llc/get-hubbed";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };
    lux = {
      url = "github:friedenberg/lux";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };
    nix-mcp-server = {
      url = "github:friedenberg/nix-mcp-server";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-master,
      utils,
      go,
      shell,
      grit,
      get-hubbed,
      lux,
      nix-mcp-server,
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

        # Upstream packages
        grit-pkg = grit.packages.${system}.default;
        lux-pkg = lux.packages.${system}.default;
        nix-mcp-server-pkg = nix-mcp-server.packages.${system}.default;

        # get-hubbed wrapped with gh on PATH
        get-hubbed-pkg =
          pkgs.runCommand "get-hubbed"
            {
              nativeBuildInputs = [ pkgs.makeWrapper ];
            }
            ''
              mkdir -p $out/bin
              makeWrapper ${get-hubbed.packages.${system}.default}/bin/get-hubbed $out/bin/get-hubbed \
                --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.gh ]}
            '';

        purse-first-pkg = pkgs.buildGoApplication {
          pname = "purse-first";
          inherit version;
          src = ./.;
          modules = ./gomod2nix.toml;
          subPackages = [ "cmd/purse-first" ];
          CGO_ENABLED = "0";
          ldflags = [
            "-s"
            "-w"
          ];

          meta = with pkgs.lib; {
            description = "MCP-first tool routing for Claude Code";
            homepage = "https://github.com/friedenberg/purse-first";
            license = licenses.mit;
          };
        };

        # Aggregated packages
        mcp-all = pkgs.symlinkJoin {
          name = "mcp-all";
          paths = [
            grit-pkg
            get-hubbed-pkg
            lux-pkg
            nix-mcp-server-pkg
          ];
        };

        marketplace = pkgs.symlinkJoin {
          name = "claude-plugin-marketplace";
          paths = [
            grit-pkg
            get-hubbed-pkg
            lux-pkg
            nix-mcp-server-pkg
            purse-first-pkg
          ];
          postBuild = ''
            mkdir -p $out/share/purse-first
            ${pkgs.jq}/bin/jq -n \
              --arg grit "${grit-pkg}/bin/grit" \
              --arg get_hubbed "${get-hubbed-pkg}/bin/get-hubbed" \
              --arg lux "${lux-pkg}/bin/lux" \
              --arg nix_mcp "${nix-mcp-server-pkg}/bin/nix-mcp-server" \
              '{
                servers: [
                  {name: "grit", type: "stdio", command: $grit, args: []},
                  {name: "get-hubbed", type: "stdio", command: $get_hubbed, args: []},
                  {name: "lux", type: "stdio", command: $lux, args: ["mcp", "stdio"]},
                  {name: "nix", type: "stdio", command: $nix_mcp, args: []}
                ]
              }' > $out/share/purse-first/marketplace.json
          '';
        };
      in
      {
        packages = {
          default = marketplace;
          inherit mcp-all;
          grit = grit-pkg;
          get-hubbed = get-hubbed-pkg;
          lux = lux-pkg;
          nix-mcp-server = nix-mcp-server-pkg;
          purse-first = purse-first-pkg;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            just
          ];

          inputsFrom = [
            go.devShells.${system}.default
            shell.devShells.${system}.default
          ];

          shellHook = ''
            echo "purse-first - dev environment"
          '';
        };

        apps.default = {
          type = "app";
          program = "${purse-first-pkg}/bin/purse-first";
        };

        apps.install = {
          type = "app";
          program = toString (
            pkgs.writeShellScript "install-marketplace" ''
              export PURSE_FIRST_MARKETPLACE="${marketplace}/share/purse-first/marketplace.json"
              exec ${marketplace}/bin/purse-first install "$@"
            ''
          );
        };
      }
    );
}
