{ pkgs, src, craneLib, purse-first-cli, goWorkspaceSrc, goVendorHash, rustWorkspaceSrc, rustCargoArtifacts }:

let
  version = "0.1.0";

  mkGoModule = import ../mkGoWorkspaceModule.nix {
    inherit pkgs goWorkspaceSrc goVendorHash;
  };

  tap-dancer-cli = mkGoModule {
    pname = "tap-dancer";
    inherit version;
    subPackages = [ "packages/tap-dancer/go/cmd/tap-dancer" ];
    meta = with pkgs.lib; {
      description = "TAP-14 validator and writer toolkit";
      homepage = "https://github.com/amarbel-llc/tap-dancer";
      license = licenses.mit;
    };
  };

  tap-dancer-rust = craneLib.buildPackage {
    src = rustWorkspaceSrc;
    cargoArtifacts = rustCargoArtifacts;
    pname = "tap-dancer";
    version = "0.1.0";
    cargoExtraArgs = "-p tap-dancer";
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
