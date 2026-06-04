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
    # conformist: the linter + formatter multiplexer (treefmt successor).
    # Config lives in ./conformist.toml.
    conformist = {
      url = "github:amarbel-llc/conformist";
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
      conformist,
    }:
    let
      # Per-system Nix interface to the Go workspace. See gomod.nix. Builds
      # every Go binary (purse-first CLI, dagnabit, the dewey analyzers), the
      # go-mcp manpage tree, and the RFC 0001 go-pkgs / go-pkgs-test sources.
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
    in
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import igloo { inherit system; };
        pkgs-master = import nixpkgs-master {
          inherit system;
          config.allowUnfree = true;
        };
        devenvs = buildDevenvs system;
        gomod = gomodBySystem.${system};

        # `nix fmt` entry point: conformist (the treefmt successor) wrapped with
        # the formatter binaries its ./conformist.toml drives on PATH. Formatting
        # drift is gated by `just lint` (conformist check), not a flake check.
        conformistFmt = pkgs.writeShellApplication {
          name = "conformist-fmt";
          runtimeInputs = [
            conformist.packages.${system}.default
            pkgs-master.gofumpt
            pkgs-master.gotools
            pkgs.nixfmt
            pkgs.shfmt
            pkgs.shellcheck
          ];
          text = ''exec conformist "$@"'';
        };
      in
      {
        packages = gomod.packages // {
          # `nix build` (no attr) → the purse-first CLI.
          default = gomod.packages.purse-first;
          # The go-mcp manpage tree (formerly bundled into the marketplace).
          manpages = gomod.manpages;
        };

        apps.default = {
          type = "app";
          program = "${gomod.packages.purse-first}/bin/purse-first";
        };

        devShells = {
          default = pkgs.mkShell {
            packages = [
              pkgs.just
              pkgs.gum
              pkgs.openssh
              pkgs-master.claude-code
              # conformist (treefmt successor) + the one formatter binary its
              # conformist.toml drives that the devShell doesn't already carry.
              # gofumpt/goimports/shfmt/shellcheck come from the go/shell devenvs;
              # nixfmt is added here now that the treefmt-nix wrapper is gone.
              # `dagnabit export` resolves `conformist` from PATH for facade formatting.
              conformist.packages.${system}.default
              pkgs.nixfmt
            ];
            inputsFrom = [
              devenvs.go.devShells.default
              devenvs.shell.devShells.default
              devenvs.bats.devShells.default
            ];
            shellHook = ''
              echo "purse-first - dev environment"
            '';
          };
          go = devenvs.go.devShells.default;
          shell = devenvs.shell.devShells.default;
          bats = devenvs.bats.devShells.default;
        };

        # `nix fmt` runs conformist (see conformistFmt above).
        formatter = conformistFmt;
      }
    );
}
