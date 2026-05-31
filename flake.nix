{
  description = "Package framework for bundling CLIs, MCP servers, and skills";

  inputs = {
    igloo.url = "github:amarbel-llc/igloo";
    nixpkgs-master.url = "github:NixOS/nixpkgs/d233902339c02a9c334e7e593de68855ad26c4cb";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    gomod2nix = {
      # Fork: adds go.work support and a build cache fix for large projects.
      # Upstream PR: https://github.com/nix-community/gomod2nix (no go.work tracking issue yet).
      url = "github:amarbel-llc/gomod2nix";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.flake-utils.follows = "utils";
    };
    # treelint: the linter + formatter multiplexer (treefmt successor).
    # Config lives in ./treelint.toml.
    treelint = {
      url = "github:amarbel-llc/treelint";
      inputs.igloo.follows = "igloo";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };
  };

  outputs =
    {
      self,
      igloo,
      nixpkgs-master,
      utils,
      gomod2nix,
      treelint,
    }:
    let
      mkMarketplace = import ./lib/mkMarketplace.nix;

      # Per-system Nix interface to the Go workspace. See gomod.nix.
      # Memoized across systems so `marketplaceOutputs`'s plugins callback
      # and the outer eachDefaultSystem block share one evaluation.
      gomodBySystem = igloo.lib.genAttrs utils.lib.defaultSystems (
        import ./gomod.nix {
          nixpkgs = igloo;
          inherit
            nixpkgs-master
            gomod2nix
            self
            ;
        }
      );

      buildDevenvs =
        system:
        let
          pkgs = import igloo { inherit system; };
          pkgs-master = import nixpkgs-master { inherit system; };
        in
        {
          go = import ./devenvs/go { inherit pkgs pkgs-master gomod2nix; };
          shell = import ./devenvs/shell { inherit pkgs; };
          bats = import ./devenvs/bats { inherit pkgs; };
        };

      marketplaceOutputs = mkMarketplace {
        nixpkgs = igloo;
        inherit nixpkgs-master utils;
        name = "purse-first";
        owner = {
          name = "friedenberg";
          email = "sasha@friedenberg.me";
        };
        description = "Package framework for bundling CLIs, MCP servers, and skills";
        repo = "amarbel-llc/purse-first";
        # Self-reference: mkMarketplace reads
        # `self.packages.${system}.purse-first` (built by gomod.nix below).
        purse-first-cli = self;
        plugins = system: [ gomodBySystem.${system}.manpages ];
        skills = ./skills;
        packageToml = ./package.toml;
        pluginConfig = builtins.fromJSON (builtins.readFile ./marketplace-config.json);
        devShellPackages = system: pkgs: pkgs-master: [
          pkgs.gum
          pkgs.openssh
          pkgs-master.claude-code
          # treelint (treefmt successor) + the one formatter binary its
          # treelint.toml drives that the devshell doesn't already carry.
          # gofumpt/goimports/shfmt/shellcheck come from the go/shell devenvs;
          # nixfmt is added here now that the treefmt-nix wrapper is gone.
          # `dagnabit export` resolves `treelint` from PATH for facade formatting.
          treelint.packages.${system}.default
          pkgs.nixfmt-rfc-style
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
          ];
        devShellHook = ''
          echo "purse-first - dev environment"
        '';
      };
    in
    igloo.lib.recursiveUpdate marketplaceOutputs (
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
          pkgs = import igloo { inherit system; };
          pkgs-master = import nixpkgs-master { inherit system; };

          # `nix fmt` entry point: treelint (the treefmt successor) wrapped with
          # the formatter binaries its ./treelint.toml drives on PATH. Formatting
          # drift is gated by `just lint` (treelint check), not a flake check.
          treelintFmt = pkgs.writeShellApplication {
            name = "treelint-fmt";
            runtimeInputs = [
              treelint.packages.${system}.default
              pkgs-master.gofumpt
              pkgs-master.gotools
              pkgs.nixfmt-rfc-style
              pkgs.shfmt
              pkgs.shellcheck
            ];
            text = ''exec treelint "$@"'';
          };
        in
        {
          packages = (marketplaceOutputs.packages.${system} or { }) // gomodBySystem.${system}.packages;

          devShells = {
            default = marketplaceOutputs.devShells.${system}.default;
            go = devenvs.go.devShells.default;
            shell = devenvs.shell.devShells.default;
            bats = devenvs.bats.devShells.default;
          };

          # `nix fmt` runs treelint (see treelintFmt above).
          formatter = treelintFmt;
        }
      ))
    );
}
