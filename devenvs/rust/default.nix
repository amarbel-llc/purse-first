# devenvs/rust/default.nix
#
# Rust toolchain for dagnabit's rust mode: cargo/rustc for fixture
# workspaces and the `cargo metadata` / `cargo check` gates; ast-grep
# for crate-rename source rewrites.
#
# Args:
#   pkgs        — stable nixpkgs (runtimes)
#   pkgs-master — unstable nixpkgs (latest tooling)
#
{
  pkgs,
  pkgs-master,
}:
let
  packages = {
    inherit (pkgs) cargo rustc rustfmt;
    inherit (pkgs-master) ast-grep;
  };
in
{
  inherit packages;

  devShells.default = pkgs.mkShell {
    packages = builtins.attrValues packages;
  };
}
