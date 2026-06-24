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
    # It is an upstream input (no cycle): conformist no longer consumes
    # purse-first — it builds golangci-lint-dewey itself from a pinned source
    # FOD, so this input does not close a loop. purse-first consumes
    # conformist.lib.evalModule + presets.eng via ./conformist.nix; `just
    # lint-conformist` / `just codemod-fmt` drive the generated check / wrapper.
    # The follows collapse the shared lock subtree. Pinned at 1b8e32d for the
    # canonical fixed justfile-default and the programs.shfmt.caseIndent option.
    conformist = {
      url = "github:amarbel-llc/conformist/1b8e32db0ee450601b0e70bb84b3b784e5f7cc3d";
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

      # Reusable conformist linter modules for dewey-layout repos (internal/ →
      # pkgs/ facade-export drift + NATO reposition), published as the top-level
      # `lib.conformistLinters` output (purse-first#163 Step 2) and dogfooded by
      # conformistImpureEval below. The paths are system-independent; downstream
      # repos (madder) import these and set deweyDir / library / dagnabitPackage /
      # conformistConfig. dagnabit coupling lives here — in the repo that owns
      # dewey+dagnabit — NOT upstream in conformist (which deliberately does not
      # depend on purse-first).
      conformistLinters = {
        dewey-facade-export = ./nix/linters/dewey-facade-export.nix;
        dewey-reposition = ./nix/linters/dewey-reposition.nix;
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
        # `conformist` reference). Yields:
        #   .config.build.wrapper      — repair-mode runner (`nix fmt`)
        #   .config.build.check <tree> — read-only `conformist check` gate
        conformistEval = conformist.lib.evalModule pkgs {
          imports = [
            ./conformist.nix
            conformist.lib.presets.eng
          ];
          package = conformistBin;
        };

        # IMPURE conformist config: whole-tree checks that need the live working
        # tree + host tools (the Go module cache / `go list` via the dewey
        # reposition + facade-export linters) and so cannot run in the sandboxed
        # checks.formatting. `just lint-worktree` builds the impure WRAPPER
        # (build.wrapper, exposed below as conformist-impure) and runs
        # `conformist check` against the working tree. See ./conformist-impure.nix.
        conformistImpureEval = conformist.lib.evalModule pkgs {
          imports = [
            ./conformist-impure.nix
            # Dogfood the published modules (same files as lib.conformistLinters);
            # purse-first's own usage is the in-repo proof they compose. Defaults
            # (deweyDir = libs/dewey, library = true, dagnabitPackage = null) match
            # purse-first, so only conformistConfig is set below.
            conformistLinters.dewey-reposition
            conformistLinters.dewey-facade-export
          ];
          package = conformistBin;
          # The facade-export linter's scripts run dagnabit's facade-format pass,
          # which needs purse-first's PURE formatter config (goimports/gofumpt) —
          # the same `.#conformist-config` the standalone facade-drift recipe
          # (now debug-dewey-pkgs-drift) passes via DAGNABIT_CONFORMIST_CONFIG
          # (purse-first#159). Baking the
          # store path here removes the per-invocation env-var contract (#163).
          linters.dewey-facade-export.conformistConfig = conformistEval.config.build.configFile;
        };
      in
      {
        packages = gomod.packages // {
          # `nix build` (no attr) → the purse-first CLI.
          default = gomod.packages.purse-first;
          # The go-mcp manpage tree (formerly bundled into the marketplace).
          manpages = gomod.manpages;
          # The generated conformist config (TOML) for ./conformist.nix +
          # presets.eng. dagnabit's facade-format lane builds this and points
          # `dagnabit export` at it via DAGNABIT_CONFORMIST_CONFIG, so dagnabit
          # formats facades with purse-first's REAL (Nix-generated) config
          # rather than searching upward for a nonexistent conformist.toml and
          # escalating to a stray ancestor (purse-first#159).
          conformist-config = conformistEval.config.build.configFile;
          # The generated config for the impure (working-tree) self-checks,
          # consumed by `just lint-worktree`. See ./conformist-impure.nix.
          conformist-impure-config = conformistImpureEval.config.build.configFile;
          # The HERMETIC conformist wrapper for the impure lane: the conformist
          # binary with the impure config + `--tree-root-file=flake.nix` baked in
          # (build.wrapper). `just lint-worktree` builds this and runs it as
          # `conformist-impure check`, so the lane runs on purse-first's pinned
          # conformist toolchain rather than the home-manager profile binary that
          # plain `conformist` would resolve to (purse-first#161 — the
          # working-tree analogue of #155). Unlike conformist-config's sandboxed
          # `conformistEval.config.build.check self`, the impure lane needs the
          # LIVE worktree + host tools (`go list`, `dagnabit`), so it uses the
          # wrapper (which locates the worktree via `--tree-root-file`) rather
          # than a `/nix/store`-copy check derivation.
          conformist-impure = conformistImpureEval.config.build.wrapper;
          # The store-pinned per-commit repair hook: `conformist --staged
          # --exit-zero-on-fix` with purse-first's PURE config + formatter
          # toolchain baked in (build.preCommit, conformist#47). On the devShell
          # PATH below as `conformist-pre-commit`; the sweatfile names it as the
          # spinclass [hooks].pre-commit command so every commit auto-formats its
          # staged *.nix/*.go/*.sh content at authoring time — what would have
          # repaired the Step 2 nixfmt miss before it ever reached the merge gate.
          # Hermetic (bundles the formatters), so it never silently skips a
          # filetype whose formatter is missing from the bare PATH (conformist#51).
          conformist-pre-commit = conformistEval.config.build.preCommit;
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
              # The per-commit conformist repair hook on PATH as
              # `conformist-pre-commit` (= packages.conformist-pre-commit), so the
              # spinclass git pre-commit hook (sweatfile [hooks].pre-commit, which
              # runs the command with the ambient/devShell PATH — no nix-develop
              # wrap) can resolve it. Takes effect after a session restart /
              # direnv reload, when spinclass re-installs the hook.
              conformistEval.config.build.preCommit
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
    )
    // {
      # System-independent reusable outputs. `lib.conformistLinters.<name>` are
      # the dewey-layout conformist linter module paths (purse-first#163 Step 2),
      # imported by downstream repos into their own `conformist.lib.evalModule`
      # (e.g. madder: `imports = [ purse-first.lib.conformistLinters.dewey-facade-export ]`
      # with `deweyDir = "go"; library = false; dagnabitPackage = …; conformistConfig = …`).
      lib.conformistLinters = conformistLinters;
    };
}
