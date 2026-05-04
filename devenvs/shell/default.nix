{ pkgs }:
{
  devShells.default = pkgs.mkShell {
    packages = with pkgs; [
      bats
      bash-language-server
      shellcheck
      shfmt
    ];
  };
}
