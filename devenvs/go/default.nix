# devenvs/go/default.nix
#
# Args:
#   pkgs        — stable nixpkgs
#   pkgs-master — unstable nixpkgs (for latest tooling)
#   gomod2nix   — the gomod2nix flake (for overlay + CLI package)
#
{
  pkgs,
  pkgs-master,
  gomod2nix,
}:
let
  packages = {
    inherit (pkgs-master)
      delve
      gofumpt
      golangci-lint
      golines
      gopls
      gotools
      govulncheck
      parallel
      ;

    # Flake-pinned wasm test runtimes for `dagnabit init-smoke run` (the
    # purse-first#180 gate lane): bun hosts the js/wasm strict loader (Go's
    # generic wasm_exec.js), wasmtime the wasip1/wasm strict loader. Pinned here
    # (stable nixpkgs, runtimes) rather than pulled via `nix shell nixpkgs#…` at
    # run time, so the default gate is reproducible (purse-first#174).
    inherit (pkgs)
      bun
      wasmtime
      ;

    go = pkgs-master.go_1_26;

    gomod2nix = gomod2nix.packages.${pkgs.stdenv.hostPlatform.system}.default;
  };
in
{
  overlay = gomod2nix.overlays.default;
  inherit packages;

  devShells.default = pkgs-master.mkShell {
    packages = builtins.attrValues packages;

  };
}
