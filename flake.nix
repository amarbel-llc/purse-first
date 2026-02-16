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
    batman = {
      url = "github:amarbel-llc/batman";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };
    tap-dancer = {
      url = "github:amarbel-llc/tap-dancer";
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
      batman,
      tap-dancer,
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

        robin-pkg = batman.packages.${system}.robin;
        tap-dancer-pkg = tap-dancer.packages.${system}.default;

        # Filter out go.work and libs/ from the nix build source so that
        # buildGoApplication doesn't enter Go workspace mode.
        purse-first-src = pkgs.lib.cleanSourceWith {
          src = ./.;
          filter =
            path: type:
            let
              baseName = builtins.baseNameOf path;
            in
            baseName != "go.work" && baseName != "go.work.sum" && !pkgs.lib.hasPrefix (toString ./libs) path;
        };

        purse-first-pkg = pkgs.buildGoApplication {
          pname = "purse-first";
          inherit version;
          src = purse-first-src;
          modules = ./gomod2nix.toml;
          subPackages = [ "cmd/purse-first" ];
          CGO_ENABLED = "0";
          ldflags = [
            "-s"
            "-w"
          ];

          postInstall = ''
            mkdir -p $out/share/purse-first/bob/skills
            cp -r ${./skills}/* $out/share/purse-first/bob/skills/

            staging=$(mktemp -d)
            ln -s $out/share/purse-first/bob/skills $staging/skills
            mkdir -p $staging/.claude-plugin
            cp ${./.claude-plugin/plugin.json} $staging/.claude-plugin/plugin.json
            chmod u+w $staging/.claude-plugin/plugin.json
            $out/bin/purse-first generate-local-plugin --root $staging
            cp $staging/.claude-plugin/plugin.json $out/share/purse-first/bob/plugin.json
          '';

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
            robin-pkg
            tap-dancer-pkg
          ];
          nativeBuildInputs = [ pkgs.makeWrapper ];
          postBuild = ''
            cp -r ${purse-first-pkg}/share/purse-first/bob $out/share/purse-first/bob

            makeWrapper ${purse-first-pkg}/bin/purse-first $out/bin/purse-first \
              --set PURSE_FIRST_PLUGINS_DIR "$out/share/purse-first"

            $out/bin/purse-first generate-marketplace \
              --plugins-dir "$out/share/purse-first" \
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
          tap-dancer = tap-dancer-pkg;
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

      }
    );
}
