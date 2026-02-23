{
  pkgs,
  src,
  go-mcp-src,
}:

let
  # Assemble source tree matching the replace directive in go.mod:
  #   replace github.com/amarbel-llc/purse-first/libs/go-mcp => ../../libs/go-mcp
  # In workspace mode (local dev), go.work resolves go-mcp via `use` and this replace is
  # a no-op because workspace replacements take precedence over go.mod replacements.
  combinedSrc = pkgs.runCommand "lux-src" { } ''
    mkdir -p $out/packages $out/libs
    cp -r ${src} $out/packages/lux
    cp -r ${go-mcp-src} $out/libs/go-mcp
  '';
in
pkgs.buildGoModule {
  pname = "lux";
  version = "0.1.0";
  src = combinedSrc;
  sourceRoot = "lux-src/packages/lux";
  vendorHash = "sha256-1g9cXhHsMsQqTJpNveMw0y50RMFY6THlOs8PUEEkT7s=";
  subPackages = [ "cmd/lux" ];

  nativeBuildInputs = [ pkgs.scdoc ];

  ldflags = [ "-X main.version=0.1.0" ];

  postInstall = ''
    $out/bin/lux _generate $out

    mkdir -p $out/share/man/man5
    scdoc < ${src}/doc/lux-config.5.scd > $out/share/man/man5/lux-config.5
  '';

  meta = with pkgs.lib; {
    description = "LSP Multiplexer that routes requests to language servers based on file type";
    homepage = "https://github.com/amarbel-llc/lux";
    license = licenses.mit;
  };
}
