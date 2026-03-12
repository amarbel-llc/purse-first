# Bob Extraction Design

Extract all packages and general-purpose skills from purse-first into a
standalone `bob` repo (`~/eng/repos/bob`). Purse-first becomes framework-only
(libs, CLI, mkMarketplace). Bob becomes a purse-first package that consumes
mkMarketplace as a flake input.

## What Moves to Bob

### Packages (all 11)

grit, get-hubbed, lux, mgp, chix, tap-dancer, batman, sandcastle,
and-so-can-you-repo, potato, spinclass.

### Skills (22 general-purpose)

brainstorming, commit, writing-plans, test-driven-development,
systematic-debugging, verification-before-completion, requesting-code-review,
receiving-code-review, using-git-worktrees, executing-plans,
subagent-driven-development, finishing-a-development-branch,
dispatching-parallel-agents, adr, fdr, rfc, using-superpowers, writing-skills,
freud, voldemort, minus-chorevaults, design_patterns-just.

`design_patterns-just` references purse-first conventions — update to be
self-contained.

### Build Infrastructure

- `lib/packages/*.nix` (all 11 package build expressions)
- `lib/mkGoWorkspaceModule.nix`
- Root `Cargo.toml` (Rust workspace: chix, tap-dancer/rust)
- `devenvs/` (go, rust, bats, shell)
- `dummies/go/` (test fixtures)
- Package-specific BATS tests from `zz-tests_bats/`

## What Stays in Purse-First

### Framework Code

- `cmd/purse-first/`, `internal/`, `purse/` (CLI + internals)
- `libs/go-mcp/`, `libs/rust-mcp/` (libraries)
- `lib/mkMarketplace.nix`, `lib/mkGoWorkspaceModule.nix`
- `docs/`, `templates/`
- Root `go.mod` (purse-first CLI module)

### Framework Skills (8, published as package "purse-first")

overview, creating-packages, using-packages, go-cli-framework, context-saving,
mcp, claude-plugins, design_patterns-downstream_rust.

Purse-first's `package.toml` changes name from "bob" to "purse-first".

### Framework Tests

Framework-level BATS tests (marketplace schema validation, etc.) stay.

## Bob Repo Structure

```
~/eng/repos/bob/
├── flake.nix              # Uses purse-first.lib.mkMarketplace
├── go.work                # Go workspace for all Go packages
├── go.work.sum
├── Cargo.toml             # Rust workspace (chix, tap-dancer/rust)
├── Cargo.lock
├── package.toml           # name = "bob"
├── marketplace-config.json
├── CLAUDE.md
├── packages/
│   ├── grit/
│   ├── get-hubbed/
│   ├── lux/
│   ├── mgp/
│   ├── chix/
│   ├── tap-dancer/
│   ├── batman/
│   ├── sandcastle/
│   ├── and-so-can-you-repo/
│   ├── potato/
│   └── spinclass/
├── skills/                # 22 general-purpose skills
│   ├── brainstorming/
│   ├── writing-plans/
│   └── ...
├── lib/
│   └── packages/          # Nix build expressions
├── devenvs/               # go, rust, bats, shell
├── dummies/go/
├── vendor/                # Go workspace vendor
└── zz-tests_bats/         # Package-specific integration tests
```

## Bob's flake.nix

```nix
{
  inputs = {
    purse-first.url = "github:amarbel-llc/purse-first";
    # purse-first transitively provides: nixpkgs, nixpkgs-master, utils,
    # crane, rust-overlay, gomod2nix
  };

  outputs = { self, purse-first, ... }:
    purse-first.lib.mkMarketplace {
      name = "bob";
      plugins = system: [ /* all 11 packages */ ];
      skills = ./skills;
      packageToml = ./package.toml;
      # ...
    };
}
```

Bob re-exports nixpkgs, crane, etc. from purse-first's inputs or declares its
own. The exact input structure depends on what mkMarketplace requires — it
currently takes `nixpkgs`, `nixpkgs-master`, `utils` as separate arguments.

## Dependency Changes

### Go

All Go packages remove `replace` directives and use published module versions:

```go
// Before (in purse-first workspace):
require github.com/amarbel-llc/purse-first/libs/go-mcp v0.0.3-...
replace github.com/amarbel-llc/purse-first/libs/go-mcp => ../../libs/go-mcp

// After (in bob repo):
require github.com/amarbel-llc/purse-first/libs/go-mcp v0.x.x
// No replace directive — uses published module
```

Bob has its own `go.work` listing all Go package modules. The `go.work` uses
`replace` directives for local development but vendor uses the published
versions.

### Rust

chix and tap-dancer/rust switch from path to git dependencies:

```toml
# Before:
mcp-server = { path = "../../libs/rust-mcp", features = [...] }

# After:
mcp-server = { git = "https://github.com/amarbel-llc/purse-first", path = "libs/rust-mcp", features = [...] }
```

### Nix

Bob's vendor hash is independent of purse-first's. Bob computes its own
`goVendorHash` covering its Go workspace's external deps (which now include
go-mcp as an external dep).

## Purse-First Changes

### flake.nix

Remove all package build logic. The flake shrinks to:
- Build `purse-first` CLI
- Expose `lib.mkMarketplace`
- Expose `lib.mkGoWorkspaceModule` (for downstream consumers)
- Publish framework skills as the "purse-first" package
- Dev shell with go + shell devenvs only (no rust, no bats)

### package.toml

```toml
name = "purse-first"
description = "Package framework for bundling CLIs, MCP servers, and skills"

[author]
name = "friedenberg"
```

### .claude-plugin/plugin.json

Update to list only the 8 framework skills.

### go.work

Shrinks to just the root module and libs:

```
use (
  .
  ./libs/go-mcp
  ./libs/go-mcp/command/huh
)
```

### Cargo.toml

Shrinks to just `libs/rust-mcp`:

```toml
[workspace]
members = ["libs/rust-mcp"]
```

## Skill Discovery

Skills are decentralized: each package declares its own skills via
`plugin.json`, and `generate-marketplace` discovers them from every package's
`share/purse-first/{name}/skills/`. Both purse-first's framework skills
(package "purse-first") and bob's general skills (package "bob") are
independently discoverable when assembled into any marketplace.

## Rollback Strategy

- **Dual-architecture period:** Purse-first's current flake.nix continues to
  work unchanged until bob is verified. Both repos build independently.
- **Promotion criteria:** Bob builds and passes all tests on all 3 platforms
  (x86_64-linux, x86_64-darwin, aarch64-darwin). `purse-first install` works
  with bob as a marketplace.
- **Rollback procedure:** Revert purse-first's flake.nix to the pre-extraction
  state (single commit revert). Bob repo is inert until integrated.

## Open Questions

1. **go-mcp module publishing:** The Go packages need a published version of
   `github.com/amarbel-llc/purse-first/libs/go-mcp`. Is there a tagged release
   available, or do we need to create one first?
2. **Transitive flake inputs:** Does bob declare its own nixpkgs/crane/etc., or
   does mkMarketplace pass them through from purse-first? Current mkMarketplace
   signature requires `nixpkgs`, `nixpkgs-master`, `utils` explicitly.
3. **CI:** Bob needs its own GitHub Actions workflow. Model after purse-first's
   existing CI.
