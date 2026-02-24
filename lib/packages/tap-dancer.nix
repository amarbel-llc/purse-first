{ pkgs, src, craneLib, purse-first-cli, goWorkspaceSrc, goVendorHash }:

let
  version = "0.1.0";

  # Workspace build: uses the full Go monorepo source with `go work vendor`.
  # The vendorHash only covers external dependencies — workspace modules
  # (go-mcp, etc.) stay in-tree, so local code changes never invalidate it.
  tap-dancer-cli = pkgs.buildGoModule {
    pname = "tap-dancer";
    inherit version;
    src = goWorkspaceSrc;
    vendorHash = goVendorHash;

    GOWORK = "";

    overrideModAttrs = _: _: {
      GOWORK = "";
      buildPhase = ''
        runHook preBuild
        go work vendor -e
        runHook postBuild
      '';
    };

    subPackages = [ "packages/tap-dancer/go/cmd/tap-dancer" ];
    meta = with pkgs.lib; {
      description = "TAP-14 validator and writer toolkit";
      homepage = "https://github.com/amarbel-llc/tap-dancer";
      license = licenses.mit;
    };
  };

  rustSrc = craneLib.cleanCargoSource "${src}/rust";
  cargoArtifacts = craneLib.buildDepsOnly {
    src = rustSrc;
    strictDeps = true;
  };
  tap-dancer-rust = craneLib.buildPackage {
    src = rustSrc;
    inherit cargoArtifacts;
    strictDeps = true;
  };

  tap-dancer-skill = pkgs.runCommand "tap-dancer-skill"
    { nativeBuildInputs = [ purse-first-cli ]; }
    ''
      ${purse-first-cli}/bin/purse-first generate-plugin \
        --root ${src} \
        --output $out \
        --skills-dir ${src}/skills
    '';

  tap-dancer-bash = pkgs.stdenvNoCC.mkDerivation {
    pname = "tap-dancer-bash";
    inherit version;
    src = "${src}/bash";
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/tap-dancer/lib/src
      cp load.bash $out/share/tap-dancer/lib/
      cp src/*.bash $out/share/tap-dancer/lib/src/
      mkdir -p $out/nix-support
      echo 'export TAP_DANCER_LIB="'"$out"'/share/tap-dancer/lib"' > $out/nix-support/setup-hook
    '';
  };
in
{
  default = pkgs.symlinkJoin {
    name = "tap-dancer";
    paths = [ tap-dancer-cli tap-dancer-rust tap-dancer-skill tap-dancer-bash ];
  };
  cli = tap-dancer-cli;
  rust = tap-dancer-rust;
  skill = tap-dancer-skill;
  bash-lib = tap-dancer-bash;
}
