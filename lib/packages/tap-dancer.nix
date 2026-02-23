{ pkgs, src, craneLib, purse-first-cli, go-mcp-src }:

let
  version = "0.1.0";

  # Assemble source tree matching the replace directive in go.mod:
  #   replace github.com/amarbel-llc/purse-first/libs/go-mcp => ../../../libs/go-mcp
  # In workspace mode (local dev), go.work resolves go-mcp via `use` and this replace is
  # a no-op because workspace replacements take precedence over go.mod replacements.
  combinedSrc = pkgs.runCommand "tap-dancer-go-src" {
    nativeBuildInputs = [ pkgs.go ];
  } ''
    mkdir -p $out/packages/tap-dancer $out/libs
    cp -r ${src}/go $out/packages/tap-dancer/go
    cp -r ${go-mcp-src} $out/libs/go-mcp
    chmod -R u+w $out
    cd $out/packages/tap-dancer/go
    export GOWORK=off HOME=$TMPDIR
    go mod vendor
  '';

  tap-dancer-cli = pkgs.buildGoModule {
    pname = "tap-dancer";
    inherit version;
    src = combinedSrc;
    sourceRoot = "tap-dancer-go-src/packages/tap-dancer/go";
    vendorHash = null;
    subPackages = [ "cmd/tap-dancer" ];
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
