# Package Brew Command Design

## Problem

Downstream repos that build purse-first marketplaces need a way to produce
Homebrew tap artifacts (formulas + tarballs) without depending on Nix. The
existing `mkBrewTap.nix` + shell script flow was never used and couples formula
generation to the Nix build system.

## Solution

Add a `purse-first package brew` command that reads a `brew-config.json` file,
produces tarballs from pre-built binaries, generates Formula.rb files with real
sha256 hashes, generates marketplace.json, and writes everything to an output
directory. A single command produces a complete, ready-to-push Homebrew tap.

## Command Interface

```
purse-first package brew \
  --config brew-config.json \
  --output ./brew-tap \
  --no-auto-install
```

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--config` | yes | — | Path to `brew-config.json` |
| `--output` | no | `.` | Directory to write output |
| `--no-auto-install` | no | false | Omit `purse-first install` from meta-formula `post_install` |

## brew-config.json Schema

Single source of truth for marketplace metadata, brew settings, and package
definitions.

```json
{
  "name": "my-marketplace",
  "description": "My purse-first marketplace",
  "owner": { "name": "org", "email": "contact@org.com" },
  "releaseRepo": "org/homebrew-tap",
  "tapName": "org/tap",
  "license": "MIT",
  "packages": {
    "grit": {
      "description": "Git operations via MCP",
      "version": "0.1.0",
      "binary": true,
      "homepage": "https://github.com/org/repo",
      "category": "development",
      "tags": ["git", "mcp"],
      "platforms": {
        "darwin-arm64": "/path/to/darwin-arm64/bin/grit",
        "darwin-amd64": "/path/to/darwin-amd64/bin/grit",
        "linux-amd64": "/path/to/linux-amd64/bin/grit"
      },
      "share": "/path/to/share/purse-first/grit",
      "brewDeps": []
    },
    "bob": {
      "description": "Skills for purse-first development",
      "version": "0.1.0",
      "binary": false,
      "share": "/path/to/share/purse-first/bob",
      "brewDeps": []
    }
  }
}
```

### Field Definitions

- `name` — marketplace name, used as the meta-formula class name
- `releaseRepo` — GitHub repo for release URLs in formulas
- `tapName` — Homebrew tap name for README
- `license` — SPDX license identifier
- `packages.<name>.binary` — whether the package has an executable
- `packages.<name>.platforms` — map of platform string to binary path (only for
  binary packages). Only platforms present get formula URL blocks.
- `packages.<name>.share` — path to the package's `share/purse-first/<name>/`
  directory (always required)
- `packages.<name>.brewDeps` — Homebrew formula dependencies

## Output Structure

```
<output>/
├── Formula/
│   ├── grit.rb
│   ├── bob.rb
│   └── my-marketplace.rb
├── tarballs/
│   ├── grit-0.1.0-darwin-arm64.tar.gz
│   ├── grit-0.1.0-linux-amd64.tar.gz
│   ├── bob-0.1.0.tar.gz
│   └── my-marketplace-0.1.0.tar.gz
├── .claude-plugin/
│   └── marketplace.json
└── README.md
```

## Tarball Contents

### Binary package tarball (`grit-0.1.0-darwin-arm64.tar.gz`)

```
./bin/grit
./share/purse-first/grit/
    .claude-plugin/plugin.json
    skills/...
    hooks/...
    mappings.json
```

### Skill-only package tarball (`bob-0.1.0.tar.gz`)

```
./share/purse-first/bob/
    .claude-plugin/plugin.json
    skills/...
```

Single platform-independent tarball. Formula uses one top-level `url`/`sha256`.

### Meta-formula tarball (`my-marketplace-0.1.0.tar.gz`)

```
./.claude-plugin/marketplace.json
```

Single platform-independent tarball.

## Formula Structure

### Binary package formula

```ruby
class Grit < Formula
  desc "Git operations via MCP"
  homepage "https://github.com/org/repo"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/org/homebrew-tap/releases/download/v0.1.0/grit-0.1.0-darwin-arm64.tar.gz"
      sha256 "<real hash>"
    end
  end

  on_linux do
    if Hardware::CPU.intel?
      url "https://github.com/org/homebrew-tap/releases/download/v0.1.0/grit-0.1.0-linux-amd64.tar.gz"
      sha256 "<real hash>"
    end
  end

  def install
    bin.install "grit"
    (share/"purse-first/grit").install Dir["share/purse-first/grit/*"]
  end

  test do
    system bin/"grit", "--help"
  end
end
```

Only platform blocks for which binaries are configured appear.

### Skill-only package formula

```ruby
class Bob < Formula
  desc "Skills for purse-first development"
  url "https://github.com/org/homebrew-tap/releases/download/v0.1.0/bob-0.1.0.tar.gz"
  sha256 "<real hash>"
  version "0.1.0"
  license "MIT"

  def install
    (share/"purse-first/bob").install Dir["share/purse-first/bob/*"]
  end

  test do
    assert_predicate share/"purse-first/bob/.claude-plugin/plugin.json", :exist?
  end
end
```

### Meta-formula

```ruby
class MyMarketplace < Formula
  desc "My purse-first marketplace"
  url "https://github.com/org/homebrew-tap/releases/download/v0.1.0/my-marketplace-0.1.0.tar.gz"
  sha256 "<real hash>"
  version "0.1.0"
  license "MIT"

  depends_on "grit"
  depends_on "bob"

  def install
    (prefix/".claude-plugin").install ".claude-plugin/marketplace.json"
  end

  def post_install
    system "purse-first", "install"
  end

  test do
    assert_predicate prefix/".claude-plugin/marketplace.json", :exist?
  end
end
```

With `--no-auto-install`, the `post_install` block is omitted.

## marketplace.json Generation

The command scans each package's `share` directory to discover MCP servers
(from `.claude-plugin/plugin.json`) and skills (from `skills/*/SKILL.md`),
reusing `marketplace.DiscoverPlugins` and `marketplace.Generate` from
`internal/marketplace/`.

## Removals

This command replaces the unused Nix-based brew tap infrastructure:

- `lib/mkBrewTap.nix`
- `bin/brew-build-tarball.bash`
- `bin/brew-update-hashes.bash`
- `brewConfig` parameter from `lib/mkMarketplace.nix`
- `brewConfig` usage in `flake.nix`

## Rollback

New additive command. Removals are of unused code. No rollback needed.
