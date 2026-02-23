{ pkgs }:
{
  devShell = pkgs.mkShell {
    packages = with pkgs; [
      bats
      parallel
      shellcheck
      shfmt
      # TODO: add bats.libraries.bats-support and bats.libraries.bats-assert
    ];
  };
}
