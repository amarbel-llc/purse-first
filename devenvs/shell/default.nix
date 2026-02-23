{ pkgs }:
{
  devShell = pkgs.mkShell {
    packages = with pkgs; [
      bats
      nodePackages.bash-language-server
      shellcheck
      shfmt
    ];
  };
}
