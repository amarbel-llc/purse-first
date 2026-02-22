# Add sandcastle to a Nix flake for test isolation.
#
# In flake inputs:
#
#   sandcastle = {
#     url = "github:amarbel-llc/sandcastle";
#     inputs.nixpkgs.follows = "nixpkgs";
#   };
#
# In outputs, include sandcastle in the devShell:
{
  pkgs,
  sandcastle,
  system,
  ...
}:
pkgs.mkShell {
  packages = (with pkgs; [
    bats
    just
  ]) ++ [
    sandcastle.packages.${system}.default
  ];
}
