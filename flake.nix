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
    chix = {
      url = "github:amarbel-llc/chix";
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
      chix,
      batman,
      tap-dancer,
    }:
    let
      mkMarketplace = import ./lib/mkMarketplace.nix;

      purse-first-src = nixpkgs.lib.cleanSourceWith {
        src = ./.;
        filter =
          path: type:
          let
            baseName = builtins.baseNameOf path;
          in
          baseName != "go.work" && baseName != "go.work.sum" && !nixpkgs.lib.hasPrefix (toString ./libs) path;
      };

      wrapGetHubbed =
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          get-hubbed-upstream = get-hubbed.packages.${system}.default;
        in
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

      marketplaceOutputs = mkMarketplace {
        inherit nixpkgs nixpkgs-master utils;
        name = "purse-first";
        owner = {
          name = "friedenberg";
          email = "sasha@friedenberg.me";
        };
        description = "MCP servers and tool routing for Claude Code, built with Nix";
        repo = "amarbel-llc/purse-first";
        purse-first-build = {
          src = purse-first-src;
          modules = ./gomod2nix.toml;
          version = "0.1.0";
          overlays = [ go.overlays.default ];
        };
        plugins = system: [
          grit.packages.${system}.default
          lux.packages.${system}.default
          chix.packages.${system}.default
          (wrapGetHubbed system)
          batman.packages.${system}.robin
          tap-dancer.packages.${system}.default
        ];
        skills = ./skills;
        pluginBaseJson = ./.claude-plugin/plugin.json;
        pluginConfig = builtins.fromJSON (builtins.readFile ./marketplace-config.json);
        brewConfig = {
          releaseRepo = "amarbel-llc/purse-first";
          tapName = "amarbel-llc/purse-first";
          exclude = [ "chix" ];
          dependencies = {
            get-hubbed = [ "gh" ];
          };
          binaryPackages = [
            "purse-first"
            "grit"
            "lux"
            "get-hubbed"
          ];
          license = "MIT";
        };
        devShellPackages =
          system: pkgs: pkgs-master: [
            pkgs-master.claude-code
            pkgs.bats
            pkgs.bats.libraries.bats-support
            pkgs.bats.libraries.bats-assert
            sandcastle.packages.${system}.default
          ];
        devShellInputsFrom = system: [
          go.devShells.${system}.default
          shell.devShells.${system}.default
          bats.devShells.${system}.default
        ];
        devShellHook = ''
          echo "purse-first - dev environment"
        '';
      };
    in
    nixpkgs.lib.recursiveUpdate marketplaceOutputs (
      {
        lib.mkMarketplace = mkMarketplace;
        templates.marketplace = {
          path = ./templates/marketplace;
          description = "Scaffold a new Claude plugin marketplace with Nix";
        };
      }
      // (utils.lib.eachDefaultSystem (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          packages = (marketplaceOutputs.packages.${system} or { }) // {
            grit = grit.packages.${system}.default;
            get-hubbed = wrapGetHubbed system;
            lux = lux.packages.${system}.default;
            chix = chix.packages.${system}.default;
            robin = batman.packages.${system}.robin;
            tap-dancer = tap-dancer.packages.${system}.default;
            mcp-all = pkgs.symlinkJoin {
              name = "mcp-all";
              paths = [
                grit.packages.${system}.default
                (wrapGetHubbed system)
                lux.packages.${system}.default
                chix.packages.${system}.default
              ];
            };
          };
          devShells.default = (marketplaceOutputs.devShells.${system}.default).overrideAttrs (old: {
            BATS_LIB_PATH = "${pkgs.bats.libraries.bats-support}/share/bats:${pkgs.bats.libraries.bats-assert}/share/bats";
          });
        }
      ))
    );
}
