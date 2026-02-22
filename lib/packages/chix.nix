{ pkgs, src, craneLib, fhPkg, rustMcpSrc }:

let
  # chix Cargo.toml has: mcp-server = { path = "../../libs/rust-mcp" }
  # Vendor rust-mcp into the chix source so the path dep resolves in the sandbox
  chixSrc = pkgs.runCommand "chix-src" { } ''
    cp -r ${src} $out
    chmod -R u+w $out
    mkdir -p $out/vendor-libs
    cp -r ${rustMcpSrc} $out/vendor-libs/rust-mcp
    sed -i 's|path = "../../libs/rust-mcp"|path = "vendor-libs/rust-mcp"|' $out/Cargo.toml
  '';

  rustSrc = craneLib.cleanCargoSource chixSrc;

  commonArgs = {
    src = rustSrc;
    strictDeps = true;
  };

  cargoArtifacts = craneLib.buildDepsOnly commonArgs;

  chix-unwrapped = craneLib.buildPackage (
    commonArgs // { inherit cargoArtifacts; }
  );

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

    mkdir -p $out/share/purse-first/chix/hooks
    cp ${src}/.claude-plugin/plugin.json $out/share/purse-first/chix/plugin.json
    install -m 755 ${formatNixHook} $out/share/purse-first/chix/hooks/format-nix
  ''
