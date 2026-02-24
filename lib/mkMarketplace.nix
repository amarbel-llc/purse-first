# lib/mkMarketplace.nix
#
# mkMarketplace — build a Claude plugin marketplace from a set of plugins.
#
# Called by purse-first's own flake.nix and available to downstream consumers
# via `purse-first.lib.mkMarketplace`.
{
  # Required — Nix infrastructure
  nixpkgs,
  nixpkgs-master,
  utils,

  # Required — marketplace identity
  name,
  owner,

  # Required — plugin set
  # function: system → list of plugin derivations
  plugins,

  # Optional — how to obtain the purse-first CLI.
  # When purse-first builds itself, it passes its own source + build config.
  # Downstream consumers pass the purse-first package from the flake input.
  purse-first-cli ? null,
  purse-first-build ? null,

  # Optional — metadata
  description ? "${name} — Claude plugin marketplace",
  repo ? null,

  # Optional — customization
  pluginConfig ? null,
  skills ? null,
  packageToml ? null,

  # Optional — Homebrew tap generation.
  # When set, produces a packages.homebrew-tap output.
  brewConfig ? null,

  # Optional — extra devShell configuration
  devShellPackages ? (_system: _pkgs: _pkgs-master: []),
  devShellInputsFrom ? (_system: []),
  devShellHook ? ''echo "${name} - dev environment"'',
}:

utils.lib.eachDefaultSystem (
  system:
  let
    pkgs = import nixpkgs { inherit system; };
    pkgs-master = import nixpkgs-master {
      inherit system;
      config.allowUnfree = true;
    };

    # Resolve the purse-first CLI package.
    # Self-build: purse-first-build is an attrset with goWorkspaceSrc, goVendorHash, version.
    # Downstream: purse-first-cli is a package from the flake input.
    cli =
      if purse-first-cli != null then
        purse-first-cli.packages.${system}.purse-first
      else if purse-first-build != null then
        let
          mkGoModule = import ./mkGoWorkspaceModule.nix {
            inherit pkgs;
            inherit (purse-first-build) goWorkspaceSrc goVendorHash;
          };
        in
        mkGoModule {
          pname = "purse-first";
          version = purse-first-build.version or "0.0.0";
          subPackages = [ "cmd/purse-first" ];
          ldflags = [
            "-s"
            "-w"
          ];
          meta = with pkgs.lib; {
            description = "MCP-first tool routing for Claude Code";
            license = licenses.mit;
          };
        }
      else
        throw "mkMarketplace: must provide either purse-first-cli or purse-first-build";

    # Resolve plugin packages for this system.
    pluginPkgs = plugins system;

    # Build the meta plugin (skills carrier) if skills are provided.
    metaPlugin =
      if skills != null then
        pkgs.runCommand "${name}-meta"
          {
            nativeBuildInputs = [ cli ];
          }
          ''
            staging=$(mktemp -d)

            ${
              if packageToml != null then
                "cp ${packageToml} $staging/package.toml"
              else
                ''
                  cat > $staging/package.toml <<'TOML_EOF'
                  name = "${name}"
                  description = "${description}"

                  [author]
                  name = "${owner.name}"
                  TOML_EOF
                ''
            }

            purse-first generate-plugin \
              --root "$staging" \
              --output "$out" \
              --skills-dir ${skills}
          ''
      else
        null;

    # Write marketplace-config.json if pluginConfig is provided.
    configFile =
      if pluginConfig != null then
        pkgs.writeText "${name}-marketplace-config.json" (builtins.toJSON (
          {
            inherit name description;
            inherit owner;
          }
          // (if repo != null then { inherit repo; } else { })
          // (if pluginConfig ? plugins then { inherit (pluginConfig) plugins; } else { })
        ))
      else
        null;

    # All packages to join: plugins + meta plugin (if present).
    allPaths = pluginPkgs ++ pkgs.lib.optional (skills != null) metaPlugin;

    # Main marketplace derivation.
    marketplace = pkgs.symlinkJoin {
      name = "${name}-marketplace";
      paths = allPaths;
      nativeBuildInputs = [ pkgs.makeWrapper ];
      postBuild = ''
        makeWrapper ${cli}/bin/purse-first $out/bin/purse-first \
          --set PURSE_FIRST_PLUGINS_DIR "$out/share/purse-first"

        $out/bin/purse-first generate-marketplace \
          --plugins-dir "$out/share/purse-first" \
          ${if configFile != null then "--config ${configFile}" else ""} \
          --output "$out/.claude-plugin/marketplace.json"
      '';
    };

    # Homebrew tap derivation (optional).
    brewTap =
      if brewConfig != null && pluginConfig != null then
        import ./mkBrewTap.nix {
          inherit pkgs pluginConfig brewConfig;
        }
      else
        null;

    # No-hooks variant.
    marketplace-no-hooks = pkgs.symlinkJoin {
      name = "${name}-marketplace-no-hooks";
      paths = allPaths;
      nativeBuildInputs = [
        pkgs.makeWrapper
        pkgs.jq
      ];
      postBuild = ''
        # Replace plugin.json symlinks with hook-stripped copies.
        for pj in $out/share/purse-first/*/plugin.json; do
          ${pkgs.jq}/bin/jq 'del(.hooks)' "$pj" > "$pj.tmp"
          rm "$pj"
          mv "$pj.tmp" "$pj"
        done

        # Remove hook script directories.
        for d in $out/share/purse-first/*/hooks; do
          [ -e "$d" ] && rm -rf "$d"
        done

        makeWrapper ${cli}/bin/purse-first $out/bin/purse-first \
          --set PURSE_FIRST_PLUGINS_DIR "$out/share/purse-first"

        $out/bin/purse-first generate-marketplace \
          --no-hooks \
          --plugins-dir "$out/share/purse-first" \
          ${if configFile != null then "--config ${configFile}" else ""} \
          --output "$out/.claude-plugin/marketplace.json"
      '';
    };
  in
  {
    packages = {
      default = marketplace;
      inherit marketplace-no-hooks;
    } // (if purse-first-build != null then { purse-first = cli; } else { })
      // (if brewTap != null then { homebrew-tap = brewTap; } else { });

    apps.default = {
      type = "app";
      program = "${marketplace}/bin/purse-first";
    };

    devShells.default = pkgs.mkShell {
      packages = [
        pkgs.just
      ] ++ (devShellPackages system pkgs pkgs-master);

      inputsFrom = devShellInputsFrom system;

      shellHook = devShellHook;
    };
  }
)
