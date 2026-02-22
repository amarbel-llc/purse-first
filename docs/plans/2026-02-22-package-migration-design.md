# Package Migration Design: sandcastle, and-so-can-you-repo, potato, spinclass

Date: 2026-02-22

## Goal

Migrate four packages into the purse-first monorepo:
- **sandcastle** (TypeScript/Node) — sandbox runtime
- **and-so-can-you-repo** (Bash) — repo scaffolding tool
- **potato** (Go) — pomodoro timer TUI
- **spinclass** (Go, renamed from sweatshop) — git worktree session manager

## Approach

Copy source files directly into `packages/<name>/`. No git history preservation. Original repos remain intact.

## Per-Package Details

### sandcastle

- **Source:** `/home/sasha/eng/repos/sandcastle`
- **Language:** TypeScript/Node.js
- **Copy:** `src/`, `vendor/`, `skills/`, `sandcastle-cli.mjs`, `package.json`, `package-lock.json`, `tsconfig.json`, `.npmrc`, `.claude-plugin/`, `zz-tests_bats/`
- **Exclude:** `.git/`, `node_modules/`, `dist/`, `.direnv/`, `result`
- **Nix build:** `buildNpmPackage` with `makeWrapper` for socat, ripgrep, bubblewrap
- **Flake change:** Remove `sandcastle` flake input, build from local source
- **Type:** CLI + Skill package

### and-so-can-you-repo

- **Source:** `/home/sasha/eng/repos/and-so-can-you-repo`
- **Language:** Bash
- **Copy:** `bin/and-so-can-you-repo.bash`
- **Exclude:** `.git/`, `.direnv/`, `result`
- **Nix build:** `writeScriptBin` + `symlinkJoin` wrapping gum, gh into PATH
- **Type:** CLI package

### potato

- **Source:** `/home/sasha/eng/repos/potato`
- **Language:** Go
- **Copy:** `cmd/`, `internal/`, `go.mod`, `go.sum`, `gomod2nix.toml`
- **Exclude:** `.git/`, `.direnv/`, `result`, `go.work`, `go.work.sum`
- **Nix build:** `buildGoApplication` with gomod2nix
- **go.work:** Add `./packages/potato`
- **Type:** CLI package

### spinclass (renamed from sweatshop)

- **Source:** `/home/sasha/eng/repos/sweatshop`
- **Language:** Go (Cobra CLI)
- **Copy:** `cmd/`, `internal/`, `completions/`, `docs/`, `tests/`, `go.mod`, `go.sum`, `gomod2nix.toml`
- **Exclude:** `.git/`, `.direnv/`, `result`, `go.work`, `go.work.sum`, `.env`
- **Nix build:** `buildGoApplication` + shell completions via `runCommand`
- **go.work:** Add `./packages/spinclass`
- **Type:** CLI package

#### Rename scope

| What | Old | New |
|------|-----|-----|
| Directory | `packages/sweatshop/` | `packages/spinclass/` |
| Go module | `github.com/amarbel-llc/sweatshop` | `github.com/amarbel-llc/spinclass` |
| Binary | `sweatshop` | `spinclass` |
| Cobra root command | `sweatshop` | `spinclass` |
| All Go imports | `amarbel-llc/sweatshop/internal/...` | `amarbel-llc/spinclass/internal/...` |

**Kept unchanged:** `sweatfile` config name, `internal/sweatfile/` package

## Integration Points

### flake.nix

- Remove `sandcastle` flake input and `sandcastlePkg` variable
- Add `sandcastlePkg`, `andSoCanYouRepoPkg`, `potatoPkg`, `spinclassPkg` to `buildPackages`
- Update `batmanPkgs` to use locally-built sandcastle
- Register all four in `packages` output attribute set

### go.work

Add entries:
```
./packages/potato
./packages/spinclass
```

### marketplace-config.json

Add entries for all four packages with descriptions and tags.
