{ pkgs }:
let
  batsLibs = [
    pkgs.bats.libraries.bats-support
    pkgs.bats.libraries.bats-assert
  ];
  batsLibPath = builtins.concatStringsSep ":" (map (lib: "${lib}/share/bats") batsLibs);
  # setupHook propagates through inputsFrom and runs with nix develop --command
  # (unlike shellHook which only runs in interactive shells)
  batsLibPathHook = pkgs.makeSetupHook { name = "bats-lib-path"; } (
    pkgs.writeText "bats-lib-path-hook.sh" ''
      export BATS_LIB_PATH="${batsLibPath}''${BATS_LIB_PATH:+:$BATS_LIB_PATH}"
    ''
  );
in
{
  devShell = pkgs.mkShell {
    packages = [
      pkgs.bats
      pkgs.parallel
      pkgs.shellcheck
      pkgs.shfmt
      batsLibPathHook
    ]
    ++ batsLibs;
  };
}
