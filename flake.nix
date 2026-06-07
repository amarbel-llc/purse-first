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
    # conformist is deliberately NOT a flake input: conformist itself takes
    # purse-first as an input (golangci-lint-dewey), so a conformist input
    # here forms a cycle that unrolls igloo duplicates into every consumer's
    # lock graph. conformist resolves from PATH instead (the eng devshell
    # ships a cwd-aware wrapper everywhere); formatting/linting goes through
    # `just codemod-fmt` / `just lint-conformist`, driven by ./conformist.toml.
  };

  outputs =
    {
      self,
      igloo,
      nixpkgs-master,
      utils,
      gomod2nix,
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
          rust = import ./devenvs/rust { inherit pkgs pkgs-master; };
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
              # conformist itself comes from the ambient environment (the eng
              # devshell ships it; see the inputs comment for why it must not
              # be a flake input). The devShell carries the formatter binaries
              # its conformist.toml drives: gofumpt/goimports/shfmt/shellcheck
              # via the go/shell devenvs, nixfmt here. `dagnabit export` and
              # the just recipes resolve `conformist` from PATH.
              pkgs.nixfmt
            ];
            inputsFrom = [
              devenvs.go.devShells.default
              devenvs.shell.devShells.default
              devenvs.bats.devShells.default
              devenvs.rust.devShells.default
            ];
            shellHook = ''
              echo "purse-first - dev environment"
            '';
          };
          go = devenvs.go.devShells.default;
          shell = devenvs.shell.devShells.default;
          bats = devenvs.bats.devShells.default;
          rust = devenvs.rust.devShells.default;
        };

        # No `formatter` output: it would need conformist, which must not be
        # a flake input (see the inputs comment). Use `just codemod-fmt`.
      }
    );
}
