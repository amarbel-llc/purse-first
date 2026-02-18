# Marketplace Framework Design

## Problem

purse-first is currently a single, hardcoded marketplace. Its `flake.nix` directly imports plugin flake inputs (grit, lux, chix, etc.) and assembles them inline. Other teams, orgs, or curators cannot use purse-first's machinery to build their own marketplaces without forking the entire repo.

## Goal

Make purse-first a **framework** for creating Claude plugin marketplaces. Any repo should be able to import purse-first as a flake input and produce a fully functional marketplace with minimal configuration.

## Design Decisions

- **Approach:** `lib.mkMarketplace` Nix function + `nix flake init` template (over pure templates or NixOS modules)
- **CLI:** Full purse-first CLI included in every downstream marketplace (same binary, different content)
- **Skills:** Downstream marketplaces define their own skills; purse-first's skills are NOT inherited unless explicitly included
- **No-hooks variant:** Always generated alongside the main marketplace
- **Dogfooding:** purse-first itself is refactored to call `mkMarketplace`, becoming the first consumer of its own framework
- **CI:** Template includes GitHub Actions workflow for build + optional FlakeHub publish

## Architecture

```
purse-first repo (framework + first marketplace)
├── lib/
│   └── mkMarketplace.nix        # Framework function
├── templates/
│   └── marketplace/             # nix flake init scaffold
│       ├── flake.nix
│       ├── justfile
│       ├── .envrc
│       ├── .github/workflows/ci.yml
│       └── .gitignore
├── flake.nix                    # Calls lib.mkMarketplace (dogfood)
│   └── exports: lib.mkMarketplace, templates.marketplace
├── cmd/purse-first/             # CLI (unchanged)
├── internal/                    # Go internals (unchanged)
├── skills/                      # purse-first's own skills
└── marketplace-config.json      # purse-first's own plugin metadata
```

## `lib.mkMarketplace` API

```nix
mkMarketplace {
  # Required — Nix infrastructure (inherited from purse-first or provided)
  nixpkgs;              # stable nixpkgs input
  nixpkgs-master;       # master/unstable nixpkgs input
  utils;                # flake-utils input

  # Required — marketplace identity
  name;                 # string: marketplace name (used in paths, derivation names)
  owner;                # attrset: { name, email }

  # Required — plugin set
  plugins;              # function: system → list of plugin packages

  # Optional — metadata
  description;          # string: marketplace description
  repo;                 # string: "owner/repo" for GitHub metadata

  # Optional — customization
  pluginConfig;         # attrset: per-plugin metadata (description, version, tags)
  skills;               # path or null: skills directory (replaces purse-first's)
  wrappers;             # function: system → pkgs → attrset of plugin wrapping functions
}
```

### Outputs (per system)

```nix
{
  packages.${system} = {
    default = marketplace;               # full marketplace
    marketplace-no-hooks = ...;          # hook-stripped variant
  };
  apps.${system}.default = {
    type = "app";
    program = "${marketplace}/bin/purse-first";
  };
  devShells.${system}.default = ...;     # dev environment
}
```

## Purse-First Dogfood Refactor

After the refactor, purse-first's `flake.nix` calls its own `mkMarketplace`:

```nix
outputs = { self, nixpkgs, nixpkgs-master, utils, grit, lux, ... }:
  let
    lib.mkMarketplace = import ./lib/mkMarketplace.nix;
  in
  (lib.mkMarketplace {
    inherit nixpkgs nixpkgs-master utils;
    purse-first-src = ./.;

    name = "purse-first";
    owner = { name = "friedenberg"; email = "sasha@friedenberg.me"; };
    description = "MCP servers and tool routing for Claude Code, built with Nix";
    repo = "amarbel-llc/purse-first";

    plugins = system: [
      grit.packages.${system}.default
      lux.packages.${system}.default
      chix.packages.${system}.default
      batman.packages.${system}.robin
      tap-dancer.packages.${system}.default
    ];

    wrappers = system: pkgs: {
      get-hubbed = pkg: /* wrap with gh on PATH */;
    };

    skills = ./skills;
    pluginConfig = builtins.fromJSON (builtins.readFile ./marketplace-config.json);
  })
  // {
    inherit lib;
    templates.marketplace = {
      path = ./templates/marketplace;
      description = "Bootstrap a new Claude plugin marketplace";
    };
  };
```

## Downstream Marketplace Usage

### 1. Scaffold

```bash
mkdir my-marketplace && cd my-marketplace
nix flake init -t purse-first#marketplace
```

### 2. Configure

Edit `flake.nix`: add plugin inputs, set name/owner/description.

### 3. Build

```bash
nix build           # produces marketplace + marketplace-no-hooks
nix run             # installs into Claude Code
```

### 4. Optionally customize

- Add skills to `./skills/` with `SKILL.md` files
- Add `marketplace-config.json` for per-plugin metadata
- Enable FlakeHub publishing in `.github/workflows/ci.yml`

## Template Contents

### `flake.nix`

```nix
{
  description = "My Claude Plugin Marketplace";

  inputs = {
    purse-first.url = "github:amarbel-llc/purse-first";
    nixpkgs.follows = "purse-first/nixpkgs";
    nixpkgs-master.follows = "purse-first/nixpkgs-master";
    utils.follows = "purse-first/utils";

    # --- Add your plugin inputs below ---
    # example-plugin = {
    #   url = "github:my-org/my-plugin";
    #   inputs.nixpkgs.follows = "nixpkgs";
    #   inputs.nixpkgs-master.follows = "nixpkgs-master";
    # };
  };

  outputs = { purse-first, nixpkgs, nixpkgs-master, utils, ... }@inputs:
    purse-first.lib.mkMarketplace {
      inherit nixpkgs nixpkgs-master utils;

      name = "my-marketplace";
      owner = { name = "my-org"; email = "team@org.com"; };

      plugins = system: [
        # inputs.example-plugin.packages.${system}.default
      ];

      # skills = ./skills;
      # pluginConfig = builtins.fromJSON (builtins.readFile ./marketplace-config.json);
    };
}
```

### `justfile`

```make
build:
  nix build --show-trace

install:
  nix run .#default

check:
  nix flake check

update:
  nix flake update
```

### `.github/workflows/ci.yml`

Build on push/PR for `x86_64-linux`, `x86_64-darwin`, `aarch64-darwin`. Optional FlakeHub publish step (commented out, ready to enable).

## Meta Plugin (Skills Carrier)

Each marketplace gets a "meta plugin" named after the marketplace. This generalizes purse-first's current "bob" plugin:

- Named `${name}` (e.g., `my-marketplace`)
- Contains bundled skills under `share/purse-first/${name}/skills/`
- Gets a generated `plugin.json` via `purse-first generate-local-plugin`
- Included in the symlinkJoin alongside MCP plugin packages
- Only created if `skills` parameter is non-null

## Implementation inside `mkMarketplace.nix`

The function:

1. Calls `utils.lib.eachDefaultSystem` to iterate over systems
2. Builds the purse-first CLI from `purse-first-src` (or uses the one from the purse-first flake input)
3. Applies `wrappers` to plugin packages that need wrapping
4. Creates the meta plugin with skills (if provided)
5. `symlinkJoin` all plugin packages + meta plugin
6. Wraps purse-first CLI with `PURSE_FIRST_PLUGINS_DIR`
7. Runs `generate-marketplace` to produce `marketplace.json`
8. Builds no-hooks variant (always)
9. Returns `{ packages, apps, devShells }`

For downstream consumers (not purse-first itself), the purse-first CLI is obtained from the purse-first flake input's packages, not rebuilt from source. The `purse-first-src` parameter is only used when purse-first builds itself.

## Scope of Changes

| Area | Change type |
|---|---|
| `lib/mkMarketplace.nix` | New file — extracted marketplace assembly logic |
| `flake.nix` | Refactor — call mkMarketplace, export lib + templates |
| `templates/marketplace/` | New directory — scaffold files |
| `templates/marketplace/.github/` | New — CI workflow |
| Go code (`internal/`, `cmd/`) | No changes |
| Skills, schemas, protocol | No changes |

## Open Questions

- Should `mkMarketplace` accept additional `devShell` packages/inputs for downstream dev environments, or is the default sufficient?
- Should the template include a `CLAUDE.md` with marketplace-specific guidance?
