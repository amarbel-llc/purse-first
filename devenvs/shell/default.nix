{ pkgs }:
{
  devShells.default = pkgs.mkShell {
    packages = with pkgs; [
      bats
      nodePackages.bash-language-server
      shellcheck
      shfmt
    ];
  };
}
