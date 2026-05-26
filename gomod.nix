# gomod.nix
#
# The Nix interface to the Go workspace defined by go.mod / go.work.
#
# Per-system function returning:
#   - manpages — go-mcp-docs manpage tree (consumed as a marketplace plugin)
#   - packages — every Go binary built from this workspace, plus the
#                RFC 0001 go-pkgs / go-pkgs-test source derivations
#                (mkGoModule wires src/pwd to go-pkgs-test so checkPhase
#                exercises the published artifact)
#
{
  nixpkgs,
  nixpkgs-master,
  gomod2nix,
  self,
}:

system:
let
  goPkgs = import nixpkgs {
    inherit system;
    overlays = [ gomod2nix.overlays.default ];
  };
  pkgs-master = import nixpkgs-master { inherit system; };

  # Per eng-versioning(7): single source of truth lives in version.env.
  # Hybrid layout — repo root for the purse-first CLI + marketplace, and
  # one version.env per independently-tagged library (currently just
  # libs/dewey; go-mcp + rust-mcp migration tracked separately).
  readVersion =
    path: varName: builtins.head (builtins.match ".*${varName}=([^\n]+).*" (builtins.readFile path));

  purseFirstVersion = readVersion ./version.env "PURSE_FIRST_VERSION";
  deweyVersion = readVersion ./libs/dewey/version.env "DEWEY_VERSION";

  commit = self.shortRev or self.dirtyShortRev or "dirty";

  deweyBuildinfo = "github.com/amarbel-llc/purse-first/libs/dewey/internal/0/buildinfo";

  deweyLdflags = [
    "-X ${deweyBuildinfo}.Version=${deweyVersion}"
    "-X ${deweyBuildinfo}.Commit=${commit}"
  ];

  inherit
    (goPkgs.mkGoPkgs {
      src = self;
      extras = [
        ".*\\.1$"
        ".*\\.7$"
      ];
    })
    go-pkgs
    go-pkgs-test
    ;

  mkGoModule = import ./lib/mkGoWorkspaceModule.nix {
    pkgs = goPkgs;
    go = pkgs-master.go_1_26;
    goWorkspaceSrc = go-pkgs-test;
  };

  mkDeweyBin =
    name:
    mkGoModule {
      pname = name;
      version = deweyVersion;
      subPackages = [ "libs/dewey/cmd/${name}" ];
      ldflags = deweyLdflags;
    };

  goMcpDocsBin = mkGoModule {
    pname = "go-mcp-docs";
    version = "0.0.9";
    subPackages = [ "cmd/go-mcp-docs" ];
  };

  manpages = goPkgs.runCommand "go-mcp-manpages" { } ''
    ${goMcpDocsBin}/bin/go-mcp-docs "$out"
  '';
in
{
  inherit manpages;

  packages = {
    inherit go-pkgs go-pkgs-test;

    purse-first = mkGoModule {
      pname = "purse-first";
      version = purseFirstVersion;
      subPackages = [ "cmd/purse-first" ];
      ldflags = [
        "-s"
        "-w"
      ];
      meta = with goPkgs.lib; {
        description = "MCP-first tool routing for Claude Code";
        license = licenses.mit;
      };
    };

    dagnabit = mkGoModule {
      pname = "dagnabit";
      version = "0.1.0";
      subPackages = [ "cmd/dagnabit" ];
      postInstall = ''
        install -Dm644 $src/cmd/dagnabit/dagnabit.1 $out/share/man/man1/dagnabit.1
      '';
    };

    defererr = mkDeweyBin "defererr";
    repool = mkDeweyBin "repool";
    seqerror = mkDeweyBin "seqerror";
    reflexive-interface-generator = mkDeweyBin "reflexive-interface-generator";
  };
}
