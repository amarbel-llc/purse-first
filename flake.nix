{
  description = "Claude Plugin Marketplace: MCP servers and tool routing for Claude Code";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
    nixpkgs-master.url = "github:NixOS/nixpkgs/b28c4999ed71543e71552ccfd0d7e68c581ba7e9";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    go.url = "github:friedenberg/eng?dir=devenvs/go";
    shell.url = "github:friedenberg/eng?dir=devenvs/shell";
    bats.url = "github:friedenberg/eng?dir=devenvs/bats";
    sandcastle.url = "github:amarbel-llc/sandcastle";

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
      bats,
      sandcastle,
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

        pkgs-master = import nixpkgs-master {
          inherit system;
          config.allowUnfree = true;
        };

        version = "0.1.0";

        # Upstream packages
        grit-pkg = grit.packages.${system}.default;
        lux-pkg = lux.packages.${system}.default;
        nix-mcp-server-pkg = nix-mcp-server.packages.${system}.default;

        # get-hubbed wrapped with gh on PATH
        get-hubbed-upstream = get-hubbed.packages.${system}.default;
        get-hubbed-pkg =
          pkgs.runCommand "get-hubbed"
            {
              nativeBuildInputs = [ pkgs.makeWrapper ];
            }
            ''
              mkdir -p $out/bin
              makeWrapper ${get-hubbed-upstream}/bin/get-hubbed $out/bin/get-hubbed \
                --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.gh ]}

              # Propagate share directory (plugin manifest, etc.)
              if [ -d "${get-hubbed-upstream}/share" ]; then
                cp -r ${get-hubbed-upstream}/share $out/share
              fi
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
          ];
          nativeBuildInputs = [ pkgs.makeWrapper ];
          postBuild = ''
            makeWrapper ${purse-first-pkg}/bin/purse-first $out/bin/purse-first \
              --set PURSE_FIRST_PLUGINS_DIR "$out/share/purse-first"

            cp -r ${./skills} $out/skills

            $out/bin/purse-first generate-marketplace \
              --plugins-dir "$out/share/purse-first" \
              --skills-dir "$out/skills" \
              --config ${./marketplace-config.json} \
              --output "$out/.claude-plugin/marketplace.json"
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
          packages = [
            pkgs.just
            pkgs-master.claude-code
            pkgs.bats
            pkgs.bats.libraries.bats-support
            pkgs.bats.libraries.bats-assert
            sandcastle.packages.${system}.default
          ];

          inputsFrom = [
            go.devShells.${system}.default
            shell.devShells.${system}.default
            bats.devShells.${system}.default
          ];

          BATS_LIB_PATH = "${pkgs.bats.libraries.bats-support}/share/bats:${pkgs.bats.libraries.bats-assert}/share/bats";

          shellHook = ''
            echo "purse-first - dev environment"
          '';
        };

        apps.default = {
          type = "app";
          program = "${marketplace}/bin/purse-first";
        };

        apps.install = {
          type = "app";
          program = toString (
            pkgs.writeShellScript "install-marketplace" ''
              exec ${marketplace}/bin/purse-first install "$@"
            ''
          );
        };
      }
    );
}
