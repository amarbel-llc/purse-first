{
  description = "Claude Plugin Marketplace: MCP servers and tool routing for Claude Code";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/6d41bc27aaf7b6a3ba6b169db3bd5d6159cfaa47";
    nixpkgs-master.url = "github:NixOS/nixpkgs/5b7e21f22978c4b740b3907f3251b470f466a9a2";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    crane.url = "github:ipetkov/crane";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    fh.url = "https://flakehub.com/f/DeterminateSystems/fh/*.tar.gz";
  };

  outputs =
    { self
    , nixpkgs
    , nixpkgs-master
    , utils
    , gomod2nix
    , crane
    , rust-overlay
    , fh
    ,
    }:
    let
      mkMarketplace = import ./lib/mkMarketplace.nix;

      # Go workspace source: includes all Go modules for `go work vendor`.
      # Used by workspace-built packages (grit, lux, get-hubbed, tap-dancer).
      goWorkspaceSrc = nixpkgs.lib.cleanSourceWith {
        src = ./.;
        filter =
          path: type:
          let
            baseName = builtins.baseNameOf path;
          in
          type == "directory"
          || nixpkgs.lib.hasSuffix ".go" baseName
          || baseName == "go.mod"
          || baseName == "go.sum"
          || baseName == "go.work"
          || baseName == "go.work.sum"
          # Lux man pages
          || nixpkgs.lib.hasSuffix ".scd" baseName;
      };

      # Single vendor hash for the entire Go workspace.
      # Only covers external deps — workspace module changes don't affect it.
      goVendorHash = "sha256-nFnvKmU08onz8Y7MMV+PYuBiYQeiWgzpvI/zNyP96mg=";

      buildDevenvs =
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          pkgs-master = import nixpkgs-master { inherit system; };
        in
        {
          go = import ./devenvs/go { inherit pkgs pkgs-master gomod2nix; };
          shell = import ./devenvs/shell { inherit pkgs; };
          bats = import ./devenvs/bats { inherit pkgs; };
          rust = import ./devenvs/rust { inherit pkgs pkgs-master rust-overlay; };
        };

      buildPackages =
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          pkgs-master = import nixpkgs-master { inherit system; };
          pkgs-overlay = import nixpkgs {
            inherit system;
            overlays = [ (import rust-overlay) ];
          };
          craneLib = (crane.mkLib pkgs).overrideToolchain (pkgs-overlay.rust-bin.stable.latest.default);
          rustWorkspaceSrc = craneLib.cleanCargoSource ./.;
          rustCommonArgs = {
            src = rustWorkspaceSrc;
            pname = "rust-workspace-deps";
            version = "0.1.0";
            strictDeps = true;
          };
          rustCargoArtifacts = craneLib.buildDepsOnly rustCommonArgs;
          fhPkg = fh.packages.${system}.default;

          sandcastlePkg = import ./lib/packages/sandcastle.nix {
            inherit pkgs;
            src = ./packages/sandcastle;
          };

          andSoCanYouRepoPkg = import ./lib/packages/and-so-can-you-repo.nix {
            inherit pkgs;
            src = ./packages/and-so-can-you-repo;
          };

          potatoPkg = import ./lib/packages/potato.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          spinclassPkg = import ./lib/packages/spinclass.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
            src = ./packages/spinclass;
          };

          gritPkg = import ./lib/packages/grit.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          get-hubbed-unwrapped = import ./lib/packages/get-hubbed.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
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
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          mgpPkg = import ./lib/packages/mgp.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          chixPkg = import ./lib/packages/chix.nix {
            inherit
              pkgs
              craneLib
              fhPkg
              purse-first-cli
              rustWorkspaceSrc
              rustCargoArtifacts
              ;
            src = ./packages/chix;
          };

          tapDancerPkgs = import ./lib/packages/tap-dancer.nix {
            inherit
              pkgs
              craneLib
              purse-first-cli
              goWorkspaceSrc
              goVendorHash
              rustWorkspaceSrc
              rustCargoArtifacts
              ;
            src = ./packages/tap-dancer;
          };

          mkGoModule = import ./lib/mkGoWorkspaceModule.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          # batman needs the purse-first CLI to build robin's plugin.json.
          # We resolve it the same way mkMarketplace does for self-builds.
          purse-first-cli = mkGoModule {
            pname = "purse-first";
            subPackages = [ "cmd/purse-first" ];
            ldflags = [
              "-s"
              "-w"
            ];
          };

          batmanPkgs = import ./lib/packages/batman.nix {
            inherit pkgs purse-first-cli;
            sandcastle = sandcastlePkg;
            tap-dancer-cli = tapDancerPkgs.cli;
            src = ./packages/batman;
          };
        in
        {
          inherit
            gritPkg
            get-hubbed-wrapped
            luxPkg
            mgpPkg
            chixPkg
            tapDancerPkgs
            batmanPkgs
            sandcastlePkg
            andSoCanYouRepoPkg
            potatoPkg
            spinclassPkg
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
          inherit goWorkspaceSrc goVendorHash;
          version = "0.1.0";
        };
        plugins =
          system:
          let
            pkgs = buildPackages system;
          in
          [
            pkgs.gritPkg
            pkgs.luxPkg
            pkgs.mgpPkg
            pkgs.chixPkg
            pkgs.get-hubbed-wrapped
            pkgs.batmanPkgs.robin
            pkgs.tapDancerPkgs.default
          ];
        skills = ./skills;
        packageToml = ./package.toml;
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
        devShellInputsFrom =
          system:
          let
            devenvs = buildDevenvs system;
          in
          [
            devenvs.go.devShell
            devenvs.shell.devShell
            devenvs.bats.devShell
            devenvs.rust.devShell
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
          localPkgs = buildPackages system;
          devenvs = buildDevenvs system;
        in
        {
          packages =
            let
              marketplacePkgs = marketplaceOutputs.packages.${system} or { };
              nonPluginPkgs = [
                localPkgs.sandcastlePkg
                localPkgs.andSoCanYouRepoPkg
                localPkgs.potatoPkg
                localPkgs.spinclassPkg
              ];
            in
            marketplacePkgs
            // {
              default = pkgs.symlinkJoin {
                name = "purse-first-all";
                paths = [ marketplacePkgs.default ] ++ nonPluginPkgs;
              };
              grit = localPkgs.gritPkg;
              get-hubbed = localPkgs.get-hubbed-wrapped;
              lux = localPkgs.luxPkg;
              mgp = localPkgs.mgpPkg;
              chix = localPkgs.chixPkg;
              robin = localPkgs.batmanPkgs.robin;
              batman = localPkgs.batmanPkgs.default;
              tap-dancer = localPkgs.tapDancerPkgs.default;
              sandcastle = localPkgs.sandcastlePkg;
              and-so-can-you-repo = localPkgs.andSoCanYouRepoPkg;
              potato = localPkgs.potatoPkg;
              spinclass = localPkgs.spinclassPkg;
              mcp-all = pkgs.symlinkJoin {
                name = "mcp-all";
                paths = [
                  localPkgs.gritPkg
                  localPkgs.get-hubbed-wrapped
                  localPkgs.luxPkg
                  localPkgs.mgpPkg
                  localPkgs.chixPkg
                ];
              };
            };

          devShells = {
            default = marketplaceOutputs.devShells.${system}.default;
            go = devenvs.go.devShell;
            shell = devenvs.shell.devShell;
            bats = devenvs.bats.devShell;
            rust = devenvs.rust.devShell;
          };
        }
      ))
    );
}
