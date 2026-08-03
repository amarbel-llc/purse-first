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

  # Per eng-versioning(7): single source of truth lives in the repo-root
  # version.env. One version covers every artifact (purse-first CLI +
  # marketplace, libs/dewey, libs/go-mcp); the release recipe tags them as a
  # set at the same version.
  readVersion =
    path: varName: builtins.head (builtins.match ".*${varName}=([^\n]+).*" (builtins.readFile path));

  purseFirstVersion = readVersion ./version.env "PURSE_FIRST_VERSION";

  commit = self.shortRev or self.dirtyShortRev or "dirty";

  deweyBuildinfo = "code.linenisgreat.com/purse-first/libs/dewey/internal/0/buildinfo";

  deweyLdflags = [
    "-X ${deweyBuildinfo}.Version=${purseFirstVersion}"
    "-X ${deweyBuildinfo}.Commit=${commit}"
  ];

  inherit
    (goPkgs.mkGoPkgs {
      src = self;
      # extras are non-.go files the published (RFC 0001) go-pkgs source must
      # still carry: manpages, plus any go:embed target. wasm_exec_strict.js is
      # embedded by libs/dewey/internal/echo/dagnabit/init_smoke_run.go, so it
      # must survive into go-pkgs-test or `nix build .#dagnabit` (built from that
      # source) fails "pattern wasm_exec_strict.js: no matching files found"
      # (purse-first#180). It is the only .js in the tree; the broad pattern
      # matches the .1/.7 style and covers future go:embed .js.
      extras = [
        ".*\\.1$"
        ".*\\.7$"
        ".*\\.js$"
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
      version = purseFirstVersion;
      subPackages = [ "libs/dewey/cmd/${name}" ];
      ldflags = deweyLdflags;
    };

  # The dewey custom golangci-lint binary: stock golangci-lint with
  # libs/dewey/gclplugin (module plugin) linked in — the pure-nix
  # replacement for `golangci-lint custom` (purse-first#134). The module
  # is standalone (outside go.work) with its own gomod2nix.toml so the
  # linter's dependency closure stays out of the shared workspace
  # lockfile. The golangci-lint version is read from that go.mod — the
  # single source of truth for the binary AND the plugin ABI, which match
  # by construction because both compile from this same source tree.
  gclDeweyDir = "cmd/golangci-lint-dewey";

  golangciLintVersion = builtins.head (
    builtins.match ".*github\\.com/golangci/golangci-lint/v2 v([^ \n\t]+).*" (
      builtins.readFile (./. + "/${gclDeweyDir}/go.mod")
    )
  );

  golangciLintDeweyVersion = "${golangciLintVersion}-dewey.${purseFirstVersion}";

  golangciLintDewey = goPkgs.buildGoApplication {
    pname = "golangci-lint-dewey";
    version = golangciLintDeweyVersion;
    go = pkgs-master.go_1_26;
    src = self;
    # pwd is eval-time: where the builder parses go.mod (and resolves the
    # `replace ... => ../../libs/dewey` into the vendor env). modRoot is
    # build-time: the hooks cd here, plant vendor/, and build subPackages
    # relative to it.
    pwd = self + "/${gclDeweyDir}";
    modRoot = gclDeweyDir;
    modules = self + "/${gclDeweyDir}/gomod2nix.toml";
    subPackages = [ "." ];
    # go.work discovery walks up from modRoot and would find the root
    # workspace, which does not list this module; force module mode.
    GOWORK = "off";
    ldflags = [
      "-s"
      "-w"
      "-X main.version=${golangciLintDeweyVersion}"
      "-X main.commit=${commit}"
      "-X main.date=${self.lastModifiedDate or ""}"
    ];
    meta = with goPkgs.lib; {
      description = "golangci-lint with the dewey module plugin linked in";
      license = licenses.gpl3Plus;
    };
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
      version = purseFirstVersion;
      subPackages = [ "cmd/dagnabit" ];
      # Burn the dewey buildinfo (version+commit) into the binary so
      # `dagnabit version` and the generated-facade header report real
      # provenance instead of dev+unknown (purse-first#164, #165).
      ldflags = deweyLdflags;
      nativeBuildInputs = [ goPkgs.makeWrapper ];
      # Rust mode shells out to ast-grep for crate-rename source rewrites.
      # --suffix (not --prefix): a user-provided ast-grep on PATH wins; the
      # wrapped store path is the fallback. cargo is intentionally NOT
      # wrapped — plain runtime-PATH expectation, like go. ast-grep comes
      # from pkgs-master to match devenvs/rust.
      postInstall = ''
        install -Dm644 $src/cmd/dagnabit/dagnabit.1 $out/share/man/man1/dagnabit.1
        wrapProgram $out/bin/dagnabit \
          --suffix PATH : ${goPkgs.lib.makeBinPath [ pkgs-master.ast-grep ]}
      '';
    };

    golangci-lint-dewey = golangciLintDewey;

    actx = mkDeweyBin "actx";
    defererr = mkDeweyBin "defererr";
    paramobj = mkDeweyBin "paramobj";
    repool = mkDeweyBin "repool";
    seqerror = mkDeweyBin "seqerror";
    testui = mkDeweyBin "testui";
    reflexive-interface-generator = mkDeweyBin "reflexive-interface-generator";
  };
}
