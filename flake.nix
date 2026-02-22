{
  description = "Claude Plugin Marketplace: MCP servers and tool routing for Claude Code";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/6d41bc27aaf7b6a3ba6b169db3bd5d6159cfaa47";
    nixpkgs-master.url = "github:NixOS/nixpkgs/5b7e21f22978c4b740b3907f3251b470f466a9a2";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    go.url = "github:amarbel-llc/eng?dir=devenvs/go";
    shell.url = "github:amarbel-llc/eng?dir=devenvs/shell";
    bats.url = "github:amarbel-llc/eng?dir=devenvs/bats";

    rust.url = "github:amarbel-llc/eng?dir=devenvs/rust";
    crane.url = "github:ipetkov/crane";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    fh.url = "https://flakehub.com/f/DeterminateSystems/fh/*.tar.gz";
    sandcastle = {
      url = "github:amarbel-llc/sandcastle";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
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
      rust,
      crane,
      rust-overlay,
      fh,
      sandcastle,
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
          baseName != "go.work"
          && baseName != "go.work.sum"
          && !nixpkgs.lib.hasPrefix (toString ./libs) path
          && !nixpkgs.lib.hasPrefix (toString ./packages) path;
      };

      buildPackages =
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          pkgs-overlay = import nixpkgs {
            inherit system;
            overlays = [ (import rust-overlay) ];
          };
          craneLib = (crane.mkLib pkgs).overrideToolchain (pkgs-overlay.rust-bin.stable.latest.default);
          goOverlay = go.overlays.default;
          fhPkg = fh.packages.${system}.default;
          sandcastlePkg = sandcastle.packages.${system}.default;

          gritPkg = import ./lib/packages/grit.nix {
            inherit pkgs goOverlay;
            src = ./packages/grit;
          };

          get-hubbed-unwrapped = import ./lib/packages/get-hubbed.nix {
            inherit pkgs goOverlay purse-first-src;
            src = ./packages/get-hubbed;
          };

          get-hubbed-wrapped =
            pkgs.runCommand "get-hubbed"
              {
                nativeBuildInputs = [ pkgs.makeWrapper ];
              }
              ''
                mkdir -p $out/bin
                makeWrapper ${get-hubbed-unwrapped}/bin/get-hubbed $out/bin/get-hubbed \
                  --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.gh ]}

                # Propagate share directory (plugin manifest, etc.)
                if [ -d "${get-hubbed-unwrapped}/share" ]; then
                  cp -r ${get-hubbed-unwrapped}/share $out/share
                fi
              '';

          luxPkg = import ./lib/packages/lux.nix {
            inherit pkgs goOverlay;
            src = ./packages/lux;
          };

          chixPkg = import ./lib/packages/chix.nix {
            inherit pkgs craneLib fhPkg;
            src = ./packages/chix;
            rustMcpSrc = ./libs/rust-mcp;
          };

          tapDancerPkgs = import ./lib/packages/tap-dancer.nix {
            inherit pkgs goOverlay craneLib;
            src = ./packages/tap-dancer;
          };

          # batman needs the purse-first CLI to build robin's plugin.json.
          # We resolve it the same way mkMarketplace does for self-builds.
          purse-first-cli =
            let
              pkgs-go = import nixpkgs {
                inherit system;
                overlays = [ goOverlay ];
              };
            in
            pkgs-go.buildGoApplication {
              pname = "purse-first";
              version = "0.1.0";
              src = purse-first-src;
              modules = ./gomod2nix.toml;
              subPackages = [ "cmd/purse-first" ];
              CGO_ENABLED = "0";
              ldflags = [
                "-s"
                "-w"
              ];
            };

          batmanPkgs = import ./lib/packages/batman.nix {
            inherit pkgs purse-first-cli;
            sandcastle = sandcastlePkg;
            src = ./packages/batman;
          };
        in
        {
          inherit
            gritPkg
            get-hubbed-wrapped
            luxPkg
            chixPkg
            tapDancerPkgs
            batmanPkgs
            ;
        };

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
        plugins =
          system:
          let
            pkgs = buildPackages system;
          in
          [
            pkgs.gritPkg
            pkgs.luxPkg
            pkgs.chixPkg
            pkgs.get-hubbed-wrapped
            pkgs.batmanPkgs.robin
            pkgs.tapDancerPkgs.default
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
          system: pkgs: pkgs-master:
          let
            localPkgs = buildPackages system;
          in
          [
            pkgs-master.claude-code
            localPkgs.batmanPkgs.default
          ];
        devShellInputsFrom = system: [
          go.devShells.${system}.default
          shell.devShells.${system}.default
          bats.devShells.${system}.default
          rust.devShells.${system}.default
        ];
        devShellHook = ''
          echo "purse-first - dev environment"
        '';
      };
    in
    nixpkgs.lib.recursiveUpdate marketplaceOutputs (
      {
        lib.mkMarketplace = mkMarketplace;
        lib.goSrc = purse-first-src;
        templates.marketplace = {
          path = ./templates/marketplace;
          description = "Scaffold a new Claude plugin marketplace with Nix";
        };
      }
      // (utils.lib.eachDefaultSystem (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          localPkgs = buildPackages system;
        in
        {
          packages = (marketplaceOutputs.packages.${system} or { }) // {
            grit = localPkgs.gritPkg;
            get-hubbed = localPkgs.get-hubbed-wrapped;
            lux = localPkgs.luxPkg;
            chix = localPkgs.chixPkg;
            robin = localPkgs.batmanPkgs.robin;
            batman = localPkgs.batmanPkgs.default;
            tap-dancer = localPkgs.tapDancerPkgs.default;
            mcp-all = pkgs.symlinkJoin {
              name = "mcp-all";
              paths = [
                localPkgs.gritPkg
                localPkgs.get-hubbed-wrapped
                localPkgs.luxPkg
                localPkgs.chixPkg
              ];
            };
          };
          devShells.default = marketplaceOutputs.devShells.${system}.default;
        }
      ))
    );
}
