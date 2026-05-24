{
  description = "Package framework for bundling CLIs, MCP servers, and skills";

  inputs = {
    nixpkgs.url = "github:amarbel-llc/nixpkgs";
    nixpkgs-master.url = "github:NixOS/nixpkgs/d233902339c02a9c334e7e593de68855ad26c4cb";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    gomod2nix = {
      # Fork: adds go.work support and a build cache fix for large projects.
      # Upstream PR: https://github.com/nix-community/gomod2nix (no go.work tracking issue yet).
      url = "github:amarbel-llc/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.flake-utils.follows = "utils";
    };
    crane.url = "github:ipetkov/crane";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
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
    }:
    let
      mkMarketplace = import ./lib/mkMarketplace.nix;

      # Per eng-versioning(7): single source of truth lives in version.env.
      # Hybrid layout — repo root for the purse-first CLI + marketplace, and
      # one version.env per independently-tagged library (currently just
      # libs/dewey; go-mcp + rust-mcp migration tracked separately).
      readVersion =
        path: varName:
        builtins.head (
          builtins.match ".*${varName}=([^\n]+).*" (builtins.readFile path)
        );

      purseFirstVersion = readVersion ./version.env "PURSE_FIRST_VERSION";
      deweyVersion = readVersion ./libs/dewey/version.env "DEWEY_VERSION";

      commit = self.shortRev or self.dirtyShortRev or "dirty";

      deweyBuildinfo = "github.com/amarbel-llc/purse-first/libs/dewey/internal/0/buildinfo";

      deweyLdflags = [
        "-X ${deweyBuildinfo}.Version=${deweyVersion}"
        "-X ${deweyBuildinfo}.Commit=${commit}"
      ];

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
          version = purseFirstVersion;
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
            # RFC 0001 flake-input-go_mod producer half: expose the
            # filtered Go workspace tree so consumer flakes can wire
            # `goFlakeInputs` against it. `pkgs.goSourceFilter` returns
            # a real derivation (passes both `nix build` and
            # `nix flake check`) and applies the canonical keep-set
            # plus our extras (go.work files and manpages).
            go-pkgs = goPkgs.goSourceFilter {
              src = self;
              extras = [
                "^go\\.work$"
                "^go\\.work\\.sum$"
                ".*\\.1$"
                ".*\\.7$"
              ];
            };

            dagnabit = mkGoModule {
              pname = "dagnabit";
              version = "0.1.0";
              subPackages = [ "cmd/dagnabit" ];
              postInstall = ''
                install -Dm644 $src/cmd/dagnabit/dagnabit.1 $out/share/man/man1/dagnabit.1
              '';
            };

            defererr = mkGoModule {
              pname = "defererr";
              version = deweyVersion;
              subPackages = [ "libs/dewey/cmd/defererr" ];
              ldflags = deweyLdflags;
            };

            repool = mkGoModule {
              pname = "repool";
              version = deweyVersion;
              subPackages = [ "libs/dewey/cmd/repool" ];
              ldflags = deweyLdflags;
            };

            seqerror = mkGoModule {
              pname = "seqerror";
              version = deweyVersion;
              subPackages = [ "libs/dewey/cmd/seqerror" ];
              ldflags = deweyLdflags;
            };

            reflexive-interface-generator = mkGoModule {
              pname = "reflexive-interface-generator";
              version = deweyVersion;
              subPackages = [ "libs/dewey/cmd/reflexive-interface-generator" ];
              ldflags = deweyLdflags;
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
