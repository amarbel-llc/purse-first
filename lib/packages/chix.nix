{ pkgs, src, craneLib, fhPkg, rustWorkspaceSrc, rustCargoArtifacts }:

let
  chix-unwrapped = craneLib.buildPackage {
    src = rustWorkspaceSrc;
    cargoArtifacts = rustCargoArtifacts;
    pname = "chix";
    version = "0.1.0";
    cargoExtraArgs = "-p chix";
    strictDeps = true;
  };

  formatNixHook = pkgs.writeShellScript "format-nix" ''
    set -euo pipefail
    input=$(cat)
    file_path=$(${pkgs.jq}/bin/jq -r '.tool_input.file_path // empty' <<< "$input")
    if [[ -n "$file_path" && "$file_path" == *.nix ]]; then
      ${pkgs.nixfmt-rfc-style}/bin/nixfmt "$file_path" 2>/dev/null || true
    fi
  '';
in
pkgs.runCommand "chix"
  {
    nativeBuildInputs = [ pkgs.makeWrapper ];
  }
  ''
    mkdir -p $out/bin
    makeWrapper ${chix-unwrapped}/bin/chix $out/bin/chix \
      --prefix PATH : ${
        pkgs.lib.makeBinPath [
          fhPkg
          pkgs.cachix
          pkgs.nil
        ]
      }

    $out/bin/chix generate-plugin $out --skills-dir ${src}/skills
    install -m 755 ${formatNixHook} $out/share/purse-first/chix/hooks/format-nix
  ''
