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
    # `nix fmt` driver. Config lives in ./treefmt.nix.
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "igloo";
    };
  };

  outputs =
    {
      self,
      igloo,
      nixpkgs-master,
      utils,
      gomod2nix,
      treefmt-nix,
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

      # treefmtEval per system. The formatter wrapper is added to the
      # default devshell so `treefmt` is on PATH for tools that integrate
      # with it (notably `dagnabit export`, which invokes treefmt on its
      # output directory if a config is present).
      treefmtEvalBySystem = igloo.lib.genAttrs utils.lib.defaultSystems (
        system: treefmt-nix.lib.evalModule (import igloo { inherit system; }) ./treefmt.nix
      );

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
          # Treefmt wrapper carrying the configured formatter chain from
          # ./treefmt.nix. `dagnabit export` looks this binary up on PATH.
          treefmtEvalBySystem.${system}.config.build.wrapper
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
          treefmtEval = treefmtEvalBySystem.${system};
        in
        {
          packages = (marketplaceOutputs.packages.${system} or { }) // gomodBySystem.${system}.packages;

          devShells = {
            default = marketplaceOutputs.devShells.${system}.default;
            go = devenvs.go.devShells.default;
            shell = devenvs.shell.devShells.default;
            bats = devenvs.bats.devShells.default;
          };

          # `nix fmt` entry point.
          formatter = treefmtEval.config.build.wrapper;

          # Sandboxed treefmt check for `nix flake check`. Runs formatters
          # over the source tree and exits non-zero on drift.
          checks.treefmt = treefmtEval.config.build.check self;
        }
      ))
    );
}
