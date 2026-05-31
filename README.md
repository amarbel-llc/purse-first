# purse-first

A package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages for humans and agents like Claude Code.

## Architecture

purse-first has three layers:

| Layer | Description |
|-------|-------------|
| **Protocol** | A convention for Nix derivations to declare Claude Code capabilities via well-known paths under `share/purse-first/`. Language-agnostic and self-describing — see [docs/purse-first-protocol.md](docs/purse-first-protocol.md). |
| **CLI** | The `purse-first` binary — tool routing (hooks), marketplace generation, package installation, and document validation. |
| **Libraries** | `go-mcp` — building blocks for protocol-conforming MCP servers (command framework, server, transports, self-install). `dewey` — multi-tier Go utilities, three static analyzers (+ golangci-lint plugin), and the `dagnabit` rename/export tool. |

## What's in this repo

purse-first was slimmed to the framework itself. The concrete MCP-server
packages (grit, get-hubbed, chix, …) moved to
[amarbel-llc/moxy](https://github.com/amarbel-llc/moxy) as **moxins**; `lux` was
pulled out and is currently **dormant** (not published in moxy or any other
active repo). What remains here:

- **`purse-first`** (CLI) — tool routing (hooks), marketplace generation, installation, and validation.
- **`go-mcp`** (library) — building blocks for protocol-conforming MCP servers: the `command.App` framework, the lower-level `server` API, stdio + Streamable HTTP transports, and `InstallMCP` self-registration.
- **`dewey`** (library) — multi-tier Go utilities organized by NATO-phonetic dependency level, the `dagnabit` rename/export tool, three static analyzers (`defererr`, `repool`, `seqerror`) exposed both as standalone vettools and as a golangci-lint module plugin, and the `reflexive-interface-generator` codegen tool.
- **In-tree skills** — `claude-plugins` and `mcp` (under `skills/`).

> **⚠ `go-mcp` is mid-overhaul.** The library is undergoing a major change and
> its API is in flux. Treat its interfaces as unstable and avoid building new
> downstream code against them until the rework settles.

## Package flavors

The protocol defines three package flavors. Concrete MCP packages now live in
[amarbel-llc/moxy](https://github.com/amarbel-llc/moxy) as moxins; the in-tree
skill packages are `claude-plugins` and `mcp`.

| Flavor | Description | Examples |
|--------|-------------|----------|
| **MCP package** | Ships an MCP server with optional tool mappings | grit, get-hubbed (in moxy) |
| **Skill package** | Ships skills only | claude-plugins, mcp (in-tree) |
| **MCP + Skill package** | Ships both an MCP server and skills | chix (in moxy) |

## CLI commands

| Command | Purpose |
|---------|---------|
| `install <marketplace-root>` | Install a built marketplace's packages into Claude Code |
| `install-self` | Install this repo's own marketplace |
| `install-local` | Set up local dev: skills and MCP servers (`--root`, `--binary`) |
| `install-dev-mcp <binary>` | Generate `.mcp.json` from a locally-built package binary |
| `generate-marketplace` | Discover packages and write `marketplace.json` (`--plugins-dir` required) |
| `generate-plugin` | Generate `plugin.json` from `package.toml` (`--root`, `--output`, `--skills-dir`) |
| `validate [path]` | Validate plugin/mapping/marketplace docs (auto-detects type; `--type`, `--strict`) |
| `validate-mcp <binary> [args...]` | Probe a binary as an MCP server (initialize + tools/list + resources) |
| `package brew` | Generate a Homebrew tap from pre-built packages (`--config` required) |

## Getting started

purse-first packages are built with Nix and installed via the `purse-first` CLI.
A **marketplace** is an aggregation of packages into a single Claude Code plugin
(the `.claude-plugin/` output).

```sh
# Install the purse-first CLI
nix profile install github:amarbel-llc/purse-first#purse-first

# Install this repo's own marketplace into Claude Code
purse-first install-self
```

Marketplace generation is normally a **build-time step** performed inside
`lib/mkMarketplace.nix` (which runs `generate-marketplace` in its `postBuild`),
not a command you run by hand. If you do invoke it directly, `--plugins-dir` is
required and `--output` defaults to `.claude-plugin/marketplace.json`:

```sh
purse-first generate-marketplace \
  --plugins-dir result/share/purse-first \
  --config marketplace-config.json \
  --output .claude-plugin/marketplace.json
```

For the complete protocol specification, see
[docs/purse-first-protocol.md](docs/purse-first-protocol.md).

## Nix package outputs

The flake exposes these `packages.<system>` outputs (see `nix flake show`):

```sh
nix build .#purse-first                    # the purse-first CLI
nix build .#dagnabit                       # dewey rename/export tool
nix build .#defererr                       # dewey static analyzer
nix build .#repool                         # dewey static analyzer
nix build .#seqerror                       # dewey static analyzer
nix build .#reflexive-interface-generator  # dewey codegen tool
nix build .#marketplace                    # default: full marketplace bundle
nix build .#marketplace-no-hooks           # marketplace with hooks stripped
nix build .#go-pkgs                        # RFC 0001 published Go workspace source
nix build .#go-pkgs-test                   # RFC 0001 test-only Go workspace source
```

## dewey: analyzers & golangci-lint plugin

`libs/dewey` is a multi-tier Go library whose packages are ordered by
NATO-phonetic dependency level (`0/`, `alfa/`, `bravo/`, …); a package at level
N may only depend on lower levels. The `dagnabit` tool repositions and renames
packages across levels and enforces that every **leaf** name is unique across
the whole tree.

dewey ships three static analyzers:

- **`defererr`** — flags deferred calls whose returned error is dropped.
- **`repool`** — flags pooled values returned to a `sync.Pool` while still referenced (no-op outside pool-owning packages).
- **`seqerror`** — flags error-handling mistakes in `iter.Seq` sequences.

Each is available two ways:

- **Standalone vettool** — `nix build .#defererr` (etc.), run via `go vet -vettool`.
- **golangci-lint module plugin** — `libs/dewey/gclplugin/` registers all three
  with `register.Plugin("dewey", New)` so repos already gating on
  `golangci-lint` fold them into that run instead of a separate pass. Wire it
  via `.custom-gcl.yml` + `golangci-lint custom`; `Settings` lets a consumer opt
  out per analyzer. The `gclplugin` package doc comment carries the full config
  snippet.

## go-mcp lifecycle (three-mode main)

> See the caution above — this library's interfaces are unstable while it is
> being reworked.

The `command.App` abstraction expects a consuming Go MCP package's `main.go` to
dispatch on its first argument:

1. **`generate-plugin <dir>`** — build-time: writes `plugin.json`, `mappings.json`, and `hooks/`.
2. **`hook`** — Claude Code `PreToolUse` handler: denies a built-in tool when an MCP tool should be used instead (fail-open).
3. **no args** — runtime: starts the MCP server.

`InstallMCP` lets a built MCP binary register itself into `~/.claude/mcp.json`
(stdio for local binaries, http for `MCPURL`-configured remotes) without
marketplace involvement. `cmd/go-mcp-docs` generates the go-mcp manpage tree,
which ships as a marketplace plugin.

## Repository layout

| Directory | Purpose |
|-----------|---------|
| `cmd/purse-first/` | CLI entrypoint |
| `cmd/dagnabit/` | dewey-aware Go rename/export tool |
| `cmd/go-mcp-docs/` | Generates the `go-mcp` manpage tree (shipped as a marketplace plugin) |
| `internal/` | Go internal packages (`install`, `marketplace`, `config`, `validate`, `localplugin`, `mcp`, `packagebrew`, `packagetoml`, `decision`) |
| `libs/go-mcp/` | Go MCP server library (`command`, `server`, `transport`, `output`, `purse`, `jsonrpc`, `executor`, `operation`, `protocol`) |
| `libs/dewey/` | Multi-tier Go library (`internal/`, `pkgs/` stable facades, `cmd/` analyzers, `gclplugin/`) |
| `purse/` | Go package for building package manifests (`plugin.json`) |
| `skills/` | In-tree skill documents (`claude-plugins`, `mcp`) |
| `lib/` | Nix build expressions (`mkMarketplace.nix`, `mkGoWorkspaceModule.nix`) |
| `gomod.nix` | Per-system Nix interface to the Go workspace; builds every binary plus the RFC 0001 `go-pkgs` / `go-pkgs-test` source derivations |
| `devenvs/` | Per-language dev shells composed into the default shell |
| `templates/marketplace/` | `nix flake init -t` template for new marketplaces |
| `zz-tests_bats/` | BATS integration tests |
| `docs/` | Protocol spec, decisions, rfcs, features, plans |

## Development

This is a Nix-based project. The justfile follows
eng-design_patterns-justfile(7): bare-verb recipes are aggregates; leaves are
`verb-noun`. Plain `just` is the CI gate (and also runs inside
`merge-this-session`).

```sh
just                     # default = validate lint build test (the CI gate)
just validate            # nix flake check + plugin manifest validation
just lint                # go vet + dewey-facade drift check
just build               # sync gomod2nix + build the marketplace bundle
just test                # all tests (Go + BATS integration)
nix fmt                  # repo-wide treefmt (Go gofumpt, Nix nixfmt, shell shfmt)
just fmt                 # go fmt ./... only (quick Go reformat)
just build-nix-gomod2nix # sync go.work and regenerate gomod2nix.toml
```

### Go workspace & lockfile

All Go modules share one `go.work` workspace (root, `libs/dewey`,
`libs/go-mcp`, `libs/go-mcp/command/huh`). External modules are pinned by a
single `gomod2nix.toml` lockfile at the workspace root; local code changes never
invalidate it. Run `just build-nix-gomod2nix` after any `go.mod` / `go.sum` /
`go.work` change — CI fails on lockfile drift.

`gomod.nix` also publishes the RFC 0001 dual outputs `go-pkgs` and
`go-pkgs-test`; self-consumption uses `go-pkgs-test` so each binary's
`checkPhase` exercises the same artifact downstream consumers receive.

## Versioning

Per eng-versioning(7) **multi-artifact release**, a single version covers
every published artifact, sourced from one root `version.env`
(`PURSE_FIRST_VERSION`). It is read at build time by `gomod.nix` (CLI version +
dewey buildinfo) and covers the purse-first CLI + marketplace, the `libs/dewey`
library and its binaries, and the `libs/go-mcp` library.

The `maintenance` recipe group exposes one triple:

```sh
just bump-version <sem>  # rewrite PURSE_FIRST_VERSION (pure mutation)
just tag <message>       # create the signed tag set: v<sem>, libs/dewey/v<sem>, libs/go-mcp/v<sem>
just release <sem>       # master-only: changelog → bump → commit → tag set → gh release
```

The path-prefixed tags are required by the Go module proxy to resolve the
sub-directory modules. Pre-1.0, MINOR bumps may include breaking changes;
post-1.0, semver is strict.
