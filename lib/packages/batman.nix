{ pkgs, src, sandcastle, purse-first-cli, tap-dancer-cli }:

let
  bats-support = pkgs.stdenvNoCC.mkDerivation {
    pname = "bats-support";
    version = "0.3.0";
    src = "${src}/lib/bats-support";
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/bats/bats-support/src
      cp load.bash $out/share/bats/bats-support/
      cp src/*.bash $out/share/bats/bats-support/src/
    '';
  };

  bats-assert = pkgs.stdenvNoCC.mkDerivation {
    pname = "bats-assert";
    version = "2.1.0";
    src = "${src}/lib/bats-assert";
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/bats/bats-assert/src
      cp load.bash $out/share/bats/bats-assert/
      cp src/*.bash $out/share/bats/bats-assert/src/
    '';
  };

  bats-assert-additions = pkgs.stdenvNoCC.mkDerivation {
    pname = "bats-assert-additions";
    version = "0.1.0";
    src = "${src}/lib/bats-assert-additions";
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/bats/bats-assert-additions/src
      cp load.bash $out/share/bats/bats-assert-additions/
      cp src/*.bash $out/share/bats/bats-assert-additions/src/
    '';
  };

  tap-writer = pkgs.stdenvNoCC.mkDerivation {
    pname = "tap-writer";
    version = "0.1.0";
    src = "${src}/lib/tap-writer";
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/bats/tap-writer/src
      cp load.bash $out/share/bats/tap-writer/
      cp src/*.bash $out/share/bats/tap-writer/src/
    '';
  };

  bats-island = pkgs.stdenvNoCC.mkDerivation {
    pname = "bats-island";
    version = "0.1.0";
    src = "${src}/lib/bats-island";
    dontUnpack = true;
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/bats/bats-island/src
      cp $src/load.bash $out/share/bats/bats-island/
      cp $src/src/*.bash $out/share/bats/bats-island/src/
    '';
  };

  bats-emo = pkgs.stdenvNoCC.mkDerivation {
    pname = "bats-emo";
    version = "0.1.0";
    src = "${src}/lib/bats-emo";
    dontUnpack = true;
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/bats/bats-emo/src
      cp $src/load.bash $out/share/bats/bats-emo/
      cp $src/src/*.bash $out/share/bats/bats-emo/src/
    '';
  };

  bats-libs = pkgs.symlinkJoin {
    name = "bats-libs";
    paths = [ bats-support bats-assert bats-assert-additions tap-writer bats-island bats-emo ];
  };

  bats = pkgs.writeShellApplication {
    name = "bats";
    runtimeInputs = [
      pkgs.bats
      pkgs.coreutils
      pkgs.gawk
      sandcastle
      tap-dancer-cli
    ];
    text = ''
      bin_dirs=()
      sandbox=true
      allow_unix_sockets=false
      no_tempdir_cleanup=false
      hide_passing=false

      while (( $# > 0 )); do
        case "$1" in
          --bin-dir)
            bin_dirs+=("$(realpath "$2")")
            shift 2
            ;;
          --no-sandbox)
            sandbox=false
            shift
            ;;
          --allow-unix-sockets)
            allow_unix_sockets=true
            shift
            ;;
          --no-tempdir-cleanup)
            no_tempdir_cleanup=true
            shift
            ;;
          --hide-passing)
            hide_passing=true
            shift
            ;;
          --)
            shift
            break
            ;;
          *)
            break
            ;;
        esac
      done

      # Prepend --bin-dir directories to PATH (leftmost = highest priority)
      for (( i = ''${#bin_dirs[@]} - 1; i >= 0; i-- )); do
        export PATH="''${bin_dirs[$i]}:$PATH"
      done

      # Append batman's bats-libs to BATS_LIB_PATH (caller paths take precedence)
      export BATS_LIB_PATH="''${BATS_LIB_PATH:+$BATS_LIB_PATH:}${bats-libs}/share/bats"

      # Default to TAP output unless a formatter flag is already present
      has_formatter=false
      for arg in "$@"; do
        case "$arg" in
          --tap|--formatter|-F|--output) has_formatter=true; break ;;
        esac
      done
      use_tap14=false
      if ! $has_formatter; then
        set -- "$@" --tap
        use_tap14=true
      fi

      filter_tap() {
        if $hide_passing; then
          awk '
            /^  ---$/ { in_yaml = 1; if (show) print; next }
            /^  \.\.\.$/ { in_yaml = 0; if (show) print; next }
            in_yaml { if (show) print; next }
            /^ok / { show = ($0 ~ /# [Ss][Kk][Ii][Pp]/ || $0 ~ /# [Tt][Oo][Dd][Oo]/); if (show) print; next }
            /^not ok / { show = 1; print; next }
            { show = 1; print }
          '
        else
          cat
        fi
      }

      reformat_tap() {
        if $use_tap14; then
          tap-dancer reformat
        else
          cat
        fi
      }

      if $sandbox; then
        config="$(mktemp)"
        trap 'rm -f "$config"' EXIT

        cat >"$config" <<SANDCASTLE_CONFIG
      {
        "filesystem": {
          "denyRead": [
            "$HOME/.ssh",
            "$HOME/.aws",
            "$HOME/.gnupg",
            "$HOME/.config",
            "$HOME/.local",
            "$HOME/.password-store",
            "$HOME/.kube"
          ],
          "denyWrite": [],
          "allowWrite": [
            "/tmp",
            "/private/tmp"
          ]
        },
        "network": {
          "allowedDomains": [],
          "deniedDomains": []$(if $allow_unix_sockets; then echo ',
          "allowAllUnixSockets": true'; fi)
        }
      }
      SANDCASTLE_CONFIG

            sandcastle_args=()
            if $no_tempdir_cleanup; then
              sandcastle_args+=(--no-tempdir-cleanup)
              set -- --no-tempdir-cleanup "$@"
            fi

            sandcastle "''${sandcastle_args[@]}" --shell bash --config "$config" -- bats "$@" | filter_tap | reformat_tap
      else
        if $no_tempdir_cleanup; then
          set -- --no-tempdir-cleanup "$@"
        fi
        bats "$@" | filter_tap | reformat_tap
      fi
    '';
  };

  robin = pkgs.stdenvNoCC.mkDerivation {
    pname = "robin";
    version = "0.1.0";
    inherit src;
    dontBuild = true;
    nativeBuildInputs = [ purse-first-cli ];
    installPhase = ''
      purse-first generate-plugin \
        --root $src \
        --output $out \
        --skills-dir $src/skills
    '';
  };
in
{
  default = pkgs.symlinkJoin {
    name = "batman";
    paths = [ bats-libs bats robin ];
  };
  inherit bats-support bats-assert bats-assert-additions tap-writer bats-island bats-emo bats-libs bats robin;
}
