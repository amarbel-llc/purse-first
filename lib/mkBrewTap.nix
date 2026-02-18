# lib/mkBrewTap.nix
#
# mkBrewTap — generate a complete Homebrew tap directory from plugin metadata.
#
# Returns a derivation whose output is a ready-to-push tap repo:
#   $out/Formula/<name>.rb   (one per eligible plugin)
#   $out/Formula/purse-first-all.rb  (meta formula)
#   $out/README.md
{
  pkgs,
  lib ? pkgs.lib,
  pluginConfig,
  brewConfig,
}:
let
  # Convert "get-hubbed" -> "GetHubbed"
  capitalize =
    s:
    let
      len = builtins.stringLength s;
    in
    if len == 0 then
      ""
    else
      (lib.toUpper (builtins.substring 0 1 s)) + (builtins.substring 1 len s);

  toClassName =
    name:
    let
      parts = builtins.split "-" name;
      words = builtins.filter builtins.isString parts;
    in
    builtins.concatStringsSep "" (map capitalize words);

  # Eligible plugins: all from pluginConfig minus excluded.
  excluded = brewConfig.exclude or [ ];
  allPluginNames = builtins.attrNames pluginConfig.plugins;
  eligibleNames = builtins.filter (n: !(builtins.elem n excluded)) allPluginNames;

  # Per-plugin dependencies from brewConfig.
  deps = brewConfig.dependencies or { };
  releaseRepo = brewConfig.releaseRepo;
  license = brewConfig.license or "MIT";

  # Binary packages have executables; skill-only packages do not.
  binaryPackages = brewConfig.binaryPackages or [ ];

  mkFormula =
    name:
    let
      meta = pluginConfig.plugins.${name};
      version = meta.version or "0.0.0";
      desc = meta.description or "${name} — purse-first package";
      homepage = meta.homepage or "https://github.com/${releaseRepo}";
      isBinary = builtins.elem name binaryPackages;
      pkgDeps = deps.${name} or [ ];
      depLines = lib.concatMapStringsSep "\n  " (d: ''depends_on "${d}"'') pkgDeps;
      installBin =
        if isBinary then
          ''
            bin.install "${name}"
                (share/"purse-first/${name}").install Dir["share/purse-first/${name}/*"]''
        else
          ''(share/"purse-first/${name}").install Dir["share/purse-first/${name}/*"]'';
      testBlock =
        if isBinary then
          ''system bin/"${name}", "--help"''
        else
          ''assert_predicate share/"purse-first/${name}/plugin.json", :exist?'';
    in
    ''
      class ${toClassName name} < Formula
        desc "${desc}"
        homepage "${homepage}"
        version "${version}"
        license "${license}"

        on_macos do
          if Hardware::CPU.arm?
            url "https://github.com/${releaseRepo}/releases/download/v#{version}/${name}-#{version}-darwin-arm64.tar.gz"
            sha256 "SHA256_PLACEHOLDER_DARWIN_ARM64"
          else
            url "https://github.com/${releaseRepo}/releases/download/v#{version}/${name}-#{version}-darwin-amd64.tar.gz"
            sha256 "SHA256_PLACEHOLDER_DARWIN_AMD64"
          end
        end

        on_linux do
          if Hardware::CPU.arm?
            url "https://github.com/${releaseRepo}/releases/download/v#{version}/${name}-#{version}-linux-arm64.tar.gz"
            sha256 "SHA256_PLACEHOLDER_LINUX_ARM64"
          else
            url "https://github.com/${releaseRepo}/releases/download/v#{version}/${name}-#{version}-linux-amd64.tar.gz"
            sha256 "SHA256_PLACEHOLDER_LINUX_AMD64"
          end
        end

        ${depLines}

        def install
          ${installBin}
        end

        test do
          ${testBlock}
        end
      end
    '';

  metaVersion = (pluginConfig.plugins.purse-first or { version = "0.0.0"; }).version;
  metaDeps = lib.concatMapStringsSep "\n  " (n: ''depends_on "${n}"'') eligibleNames;
  metaFormula = ''
    class PurseFirstAll < Formula
      desc "All purse-first packages for Claude Code"
      homepage "https://github.com/${releaseRepo}"
      version "${metaVersion}"
      license "${license}"

      ${metaDeps}

      def install
        ohai "purse-first-all: all packages installed via dependencies"
      end

      test do
        system "true"
      end
    end
  '';

  readmePluginRows = lib.concatMapStringsSep "\n" (
    n: "| `${n}` | ${(pluginConfig.plugins.${n}).description or ""} |"
  ) eligibleNames;
  readmeInstallLines = lib.concatMapStringsSep "\n" (n: "brew install ${n}") eligibleNames;
  tapName = brewConfig.tapName or "amarbel-llc/purse-first";

  readme = ''
    # purse-first Homebrew Tap

    ${pluginConfig.description or ""}

    ## Installation

    ```bash
    brew tap ${tapName}
    ```

    ## Available Packages

    | Package | Description |
    |---------|-------------|
    ${readmePluginRows}
    | `purse-first-all` | Meta package — installs everything |

    ## Install All

    ```bash
    brew install purse-first-all
    ```

    ## Install Individual Packages

    ```bash
    ${readmeInstallLines}
    ```
  '';

  writeFormulas = lib.concatMapStringsSep "\n" (
    n: ''
      cat > $out/Formula/${n}.rb <<'FORMULA_EOF'
      ${mkFormula n}
      FORMULA_EOF
    ''
  ) eligibleNames;

in
pkgs.runCommand "homebrew-tap" { } ''
  mkdir -p $out/Formula

  ${writeFormulas}

  cat > $out/Formula/purse-first-all.rb <<'FORMULA_EOF'
  ${metaFormula}
  FORMULA_EOF

  cat > $out/README.md <<'README_EOF'
  ${readme}
  README_EOF
''
