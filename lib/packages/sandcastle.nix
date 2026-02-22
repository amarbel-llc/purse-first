{ pkgs, src }:

pkgs.buildNpmPackage {
  pname = "sandcastle";
  version = "0.0.37";

  inherit src;

  npmDepsHash = "sha256-LMqLtMWMmzEiHW+VJAPnivqHtoJV2wWWP2S8Z/smfWc=";

  nativeBuildInputs = [ pkgs.makeWrapper ];

  buildPhase = ''
    runHook preBuild
    npm run build
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall

    mkdir -p $out/lib/sandcastle $out/bin

    cp -r dist/* $out/lib/sandcastle/
    cp -r node_modules $out/lib/sandcastle/
    cp package.json $out/lib/sandcastle/
    cp ${src}/sandcastle-cli.mjs $out/lib/sandcastle/sandcastle-cli.mjs

    makeWrapper ${pkgs.nodejs_22}/bin/node $out/bin/sandcastle \
      --add-flags "$out/lib/sandcastle/sandcastle-cli.mjs" \
      --prefix PATH : ${
        pkgs.lib.makeBinPath (
          [
            pkgs.socat
            pkgs.ripgrep
          ]
          ++ pkgs.lib.optionals pkgs.stdenv.isLinux [ pkgs.bubblewrap ]
        )
      }

    runHook postInstall
  '';
}
