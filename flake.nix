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

    # conformist is purse-first's formatter + eng-convention linter library.
    # It is now an upstream input (the former cycle is gone): conformist no
    # longer consumes purse-first — it builds golangci-lint-dewey itself from a
    # pinned source FOD (conformist master a8471278), so this input does not
    # close a loop. purse-first consumes conformist.lib.evalModule + presets.eng
    # via ./conformist.nix; `just lint-conformist` / `just codemod-fmt` drive the
    # generated check / wrapper. The follows collapse the shared lock subtree.
    conformist = {
      url = "github:amarbel-llc/conformist/a8471278ac47e7156581bd214ae5d12c42b4a52d";
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

        # The conformist binary purse-first runs for `conformist check` /
        # repair. Its output is input-addressed and version-stamped, so it
        # churns only when the conformist input is deliberately bumped — not
        # per purse-first commit.
        conformistBin = conformist.packages.${system}.default;

        # conformist's own evalModule, fed purse-first's ./conformist.nix plus
        # the eng-convention preset (presets.eng is a path exposed on
        # conformist.lib; importing it here keeps ./conformist.nix free of any
        # `conformist` reference) and purse-first's own correct
        # justfile-default linter (./nix/linters/justfile-default-native.nix —
        # mkLinterModule/mkFormatterModule are in scope via evalModule's
        # specialArgs). Yields:
        #   .config.build.wrapper      — repair-mode runner (`nix fmt`)
        #   .config.build.check <tree> — read-only `conformist check` gate
        conformistEval = conformist.lib.evalModule pkgs {
          imports = [
            ./conformist.nix
            ./nix/linters/justfile-default-native.nix
            conformist.lib.presets.eng
          ];
          package = conformistBin;
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
              # conformist itself, pinned via the flake input — on PATH so
              # dagnabit's FormatOutput() (libs/dewey/.../exporter_treefmt.go)
              # resolves `conformist` for facade formatting, and for interactive
              # use. The formatter binaries it drives still come from the
              # devenvs: gofumpt/goimports/shfmt/shellcheck via go/shell, nixfmt
              # here.
              conformistBin
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

        # Repair-mode formatter (`nix fmt`): conformist wrapped with the
        # generated config. `just codemod-fmt-conformist` drives this.
        formatter = conformistEval.config.build.wrapper;

        # Read-only gate: the project tree passes `conformist check` (formatters
        # would make no change; linters report no findings) without mutating
        # anything. `just lint-conformist` builds this. build.check takes the
        # tree path (self), mirroring conformist's own checks.formatting.
        checks.formatting = conformistEval.config.build.check self;
      }
    );
}
