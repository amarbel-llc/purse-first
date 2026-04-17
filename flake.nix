{
  description = "Package framework for bundling CLIs, MCP servers, and skills";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/4590696c8693fea477850fe379a01544293ca4e2";
    nixpkgs-master.url = "github:NixOS/nixpkgs/e2dde111aea2c0699531dc616112a96cd55ab8b5";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    gomod2nix = {
      # Fork: adds go.work support and a build cache fix for large projects.
      # Upstream PR: https://github.com/nix-community/gomod2nix (no go.work tracking issue yet).
      url = "github:amarbel-llc/gomod2nix";
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
    {
      self,
      nixpkgs,
      nixpkgs-master,
      utils,
      gomod2nix,
      crane,
      rust-overlay,
      fh,
    }:
    let
      mkMarketplace = import ./lib/mkMarketplace.nix;

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
          || baseName == "gomod2nix.toml"
          || nixpkgs.lib.hasSuffix ".1" baseName
          || nixpkgs.lib.hasSuffix ".7" baseName;
      };

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

      # Build go-mcp-docs binary per-system, then run it to produce manpages.
      buildGoMcpDocs =
        system:
        let
          goPkgs = import nixpkgs {
            inherit system;
            overlays = [ gomod2nix.overlays.default ];
          };
          pkgs-master = import nixpkgs-master { inherit system; };
          mkGoModule = import ./lib/mkGoWorkspaceModule.nix {
            pkgs = goPkgs;
            go = pkgs-master.go_1_26;
            inherit goWorkspaceSrc;
          };
          docsBin = mkGoModule {
            pname = "go-mcp-docs";
            version = "0.0.9";
            subPackages = [ "cmd/go-mcp-docs" ];
          };
        in
        goPkgs.runCommand "go-mcp-manpages" { } ''
          ${docsBin}/bin/go-mcp-docs "$out"
        '';

      marketplaceOutputs = mkMarketplace {
        inherit nixpkgs nixpkgs-master utils;
        name = "purse-first";
        owner = {
          name = "friedenberg";
          email = "sasha@friedenberg.me";
        };
        description = "Package framework for bundling CLIs, MCP servers, and skills";
        repo = "amarbel-llc/purse-first";
        purse-first-build = {
          inherit goWorkspaceSrc;
          goOverlays = [ gomod2nix.overlays.default ];
          version = "0.1.0";
        };
        plugins = system: [ (buildGoMcpDocs system) ];
        skills = ./skills;
        packageToml = ./package.toml;
        pluginConfig = builtins.fromJSON (builtins.readFile ./marketplace-config.json);
        devShellPackages = _system: pkgs: pkgs-master: [
          pkgs.gum
          pkgs.openssh
          pkgs-master.claude-code
        ];
        devShellInputsFrom =
          system:
          let
            devenvs = buildDevenvs system;
          in
          [
            devenvs.go.devShells.default
            devenvs.shell.devShells.default
            devenvs.bats.devShells.default
            devenvs.rust.devShells.default
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
          devenvs = buildDevenvs system;
          goPkgs = import nixpkgs {
            inherit system;
            overlays = [ gomod2nix.overlays.default ];
          };
          pkgs-master = import nixpkgs-master { inherit system; };
          mkGoModule = import ./lib/mkGoWorkspaceModule.nix {
            pkgs = goPkgs;
            go = pkgs-master.go_1_26;
            inherit goWorkspaceSrc;
          };
        in
        {
          packages = (marketplaceOutputs.packages.${system} or { }) // {
            dagnabit = mkGoModule {
              pname = "dagnabit";
              version = "0.1.0";
              subPackages = [ "cmd/dagnabit" ];
              postInstall = ''
                install -Dm644 $src/cmd/dagnabit/dagnabit.1 $out/share/man/man1/dagnabit.1
              '';
            };
          };

          devShells = {
            default = marketplaceOutputs.devShells.${system}.default;
            go = devenvs.go.devShells.default;
            shell = devenvs.shell.devShells.default;
            bats = devenvs.bats.devShells.default;
            rust = devenvs.rust.devShells.default;
          };
        }
      ))
    );
}
