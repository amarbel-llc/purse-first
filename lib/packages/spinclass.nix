# Workspace build: uses the full Go monorepo source with `go work vendor`.
# The vendorHash only covers external dependencies — workspace modules
# (go-mcp, etc.) stay in-tree, so local code changes never invalidate it.
{
  pkgs,
  goWorkspaceSrc,
  goVendorHash,
  src, # Original source for non-Go assets (completions)
}:

let
  spinclass = pkgs.buildGoModule {
    pname = "spinclass";
    version = "0.1.0";
    src = goWorkspaceSrc;
    vendorHash = goVendorHash;

    # Enable workspace mode (buildGoModule defaults to GOWORK=off)
    GOWORK = "";

    overrideModAttrs = _: _: {
      GOWORK = "";
      buildPhase = ''
        runHook preBuild
        go work vendor -e
        runHook postBuild
      '';
    };

    subPackages = [ "packages/spinclass/cmd/spinclass" ];
  };

  shellCompletions = pkgs.runCommand "spinclass-completions" { } ''
    install -Dm644 ${src}/completions/spinclass.bash-completion \
      $out/share/bash-completion/completions/spinclass
    install -Dm644 ${src}/completions/spinclass.fish \
      $out/share/fish/vendor_completions.d/spinclass.fish
  '';
in
pkgs.symlinkJoin {
  name = "spinclass";
  paths = [
    spinclass
    shellCompletions
  ];
}
