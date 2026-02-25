# Rust MCP server with static plugin.json copied during build.
# Uses crane for Cargo builds with optional makeWrapper for runtime deps.
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    crane.url = "github:ipetkov/crane";
    rust.url = "github:amarbel-llc/purse-first?dir=devenvs/rust";
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-master,
      utils,
      rust-overlay,
      crane,
      rust,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        overlays = [ (import rust-overlay) ];
        pkgs = import nixpkgs { inherit system overlays; };

        rustToolchain = pkgs.rust-bin.stable.latest.default;
        craneLib = (crane.mkLib pkgs).overrideToolchain rustToolchain;

        src = craneLib.cleanCargoSource ./.;

        commonArgs = {
          inherit src;
          strictDeps = true;
        };

        cargoArtifacts = craneLib.buildDepsOnly commonArgs;

        my-mcp-unwrapped = craneLib.buildPackage (
          commonArgs // { inherit cargoArtifacts; }
        );

        # Wrap with runtime dependencies and add plugin manifest
        my-mcp =
          pkgs.runCommand "my-mcp"
            { nativeBuildInputs = [ pkgs.makeWrapper ]; }
            ''
              mkdir -p $out/bin
              makeWrapper ${my-mcp-unwrapped}/bin/my-mcp $out/bin/my-mcp \
                --prefix PATH : ${pkgs.lib.makeBinPath [ /* runtime deps */ ]}

              mkdir -p $out/share/purse-first/my-mcp
              cp ${./plugin.json} $out/share/purse-first/my-mcp/plugin.json
            '';
      in
      {
        packages = {
          default = my-mcp;
          unwrapped = my-mcp-unwrapped;
        };

        devShells.default = rust.devShells.${system}.default;
      }
    );
}
