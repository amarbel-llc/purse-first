# devenvs/rust/default.nix
#
# Args:
#   pkgs          — stable nixpkgs
#   pkgs-master   — unstable nixpkgs
#   rust-overlay  — the rust-overlay flake
#
{
  pkgs,
  pkgs-master,
  rust-overlay,
}:
let
  pkgs-rust = import pkgs-master.path {
    inherit (pkgs) system;
    overlays = [
      rust-overlay.overlays.default
      (final: prev: {
        rustToolchain =
          let
            rust = prev.rust-bin;
          in
          if builtins.pathExists ./rust-toolchain.toml then
            rust.fromRustupToolchainFile ./rust-toolchain.toml
          else if builtins.pathExists ./rust-toolchain then
            rust.fromRustupToolchainFile ./rust-toolchain
          else
            rust.stable.latest.default.override {
              extensions = [
                "rust-src"
                "rustfmt"
              ];
            };
      })
    ];
  };
in
{
  devShell = pkgs-rust.mkShell {
    packages = [
      pkgs-rust.rustToolchain
      pkgs.openssl
      pkgs.pkg-config
      pkgs-rust.cargo-deny
      pkgs-rust.cargo-edit
      pkgs-rust.cargo-watch
      pkgs-rust.rust-analyzer
    ];
  };
}
