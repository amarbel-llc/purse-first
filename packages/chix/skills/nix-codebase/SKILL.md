---
name: Nix Codebase Workflow
description: This skill should be used when the user asks to "build a nix project", "fix nix build", "add a go dependency", "run gomod2nix", "update flake inputs", "set up a new nix flake", "create a devshell", "add a nix package", "debug nix build failure", "publish to flakehub", "set up CI", "set up github actions", "deploy to flakehub", "add flakehub workflow", or is working in any repository that contains a flake.nix file. Also applies when encountering gomod2nix hash mismatches, stale dependency files, Nix build errors in Go or Rust projects, or CI/CD pipeline setup for Nix projects.
version: 0.2.0
---

# Nix Codebase Workflow

This skill provides procedural knowledge for working with Nix-backed codebases, with emphasis on Go projects using gomod2nix, the stable-first nixpkgs convention, FlakeHub publishing, and GitHub Actions CI. It captures build workflows, dependency management, CI/CD patterns, and common failure modes.

## Critical Rule: Check the Justfile First

Before running any build, test, or format command, **always read the project's justfile**. Every Nix-backed project in this ecosystem uses a justfile as the primary command interface. Never guess at build commands — the justfile is the source of truth.

Common justfile targets across projects:

| Target | Purpose |
|--------|---------|
| `build` | `nix build` (full Nix build) |
| `build-go` | Fast local Go build via `nix develop` |
| `deps` | `go mod tidy` + `gomod2nix` |
| `test` | Run unit tests |
| `test-bats` | Run BATS integration tests |
| `fmt` | Format code |
| `check` / `lint` | Linting |
| `clean` | Remove build artifacts |

## Go + Nix Build Order

When modifying Go code in a Nix-backed project, follow this exact sequence:

### After changing Go source code only (no new imports):

```
just fmt        →  just build  →  just test
```

### After adding, removing, or updating Go dependencies:

```
just deps       →  just build  →  just test
```

The `deps` target runs two commands in sequence:

1. `go mod tidy` — updates `go.mod` and `go.sum`
2. `gomod2nix` — regenerates `gomod2nix.toml` from `go.sum`

**Both steps run inside `nix develop` to access the correct toolchain.**

### NEVER skip `gomod2nix` after dependency changes

If `gomod2nix.toml` is stale (out of sync with `go.mod`/`go.sum`), `nix build` will fail with hash mismatches. This is the single most common build failure in Go+Nix projects.

**Signs of a stale `gomod2nix.toml`:**
- `nix build` fails with "hash mismatch" errors
- Error references a specific Go module version
- `go.mod` was modified but `gomod2nix.toml` was not

**Fix:** Run `just deps` (or `nix develop --command gomod2nix` directly).

## The gomod2nix Workflow

### File Relationship

```
go.mod          ← Developer edits (dependency declarations)
    ↓ go mod tidy
go.sum          ← Go toolchain generates (checksums)
    ↓ gomod2nix
gomod2nix.toml  ← gomod2nix generates (Nix-compatible hashes)
    ↓ nix build
result/         ← Nix produces (reproducible binary)
```

### When to regenerate gomod2nix.toml

Regenerate after ANY of these actions:
- `go get <package>` (adding a dependency)
- `go get -u <package>` (updating a dependency)
- `go mod tidy` (cleaning up dependencies)
- Editing `go.mod` directly
- Changing the Go version in `go.mod`

### How to regenerate

Prefer the justfile target:
```bash
just deps
```

Manual equivalent:
```bash
nix develop --command go mod tidy
nix develop --command gomod2nix
```

**Important:** The `gomod2nix` binary is only available inside `nix develop`. Running it outside the devshell produces "command not found."

## Nix Flake Conventions

### Stable-First Nixpkgs

All flakes follow the stable-first convention — never deviate:

- `nixpkgs` → stable branch (runtimes, core tools)
- `nixpkgs-master` → master/unstable (LSPs, linters, formatters)
- Variables: `pkgs = import nixpkgs`, `pkgs-master = import nixpkgs-master`

### Go Project Flake Structure

Go projects use `buildGoApplication` from the gomod2nix overlay, NOT `buildGoModule`:

```nix
inputs = {
  go.url = "github:friedenberg/eng?dir=devenvs/go";
};

# Apply overlay:
pkgs = import nixpkgs {
  inherit system;
  overlays = [ go.overlays.default ];
};

# Build:
pkgs.buildGoApplication {
  pname = "project-name";
  version = "0.1.0";
  src = ./.;
  modules = ./gomod2nix.toml;     # ← Required
  subPackages = [ "cmd/binary" ];  # ← Which binaries to build
};
```

### DevShell Pattern

Compose devshells from devenv flakes:

```nix
devShells.default = pkgs.mkShell {
  packages = with pkgs; [ just gum ];
  inputsFrom = [
    go.devShells.${system}.default
    shell.devShells.${system}.default
  ];
};
```

This provides: `go`, `gopls`, `gofumpt`, `golangci-lint`, `gomod2nix`, `shellcheck`, `shfmt`, and more.

## Rust + Nix Build Order

Rust projects are simpler — `Cargo.lock` is managed by cargo directly:

```
cargo build     →  cargo test    →  cargo clippy
```

For Nix builds: `just build` (runs `nix build`).

No equivalent of the gomod2nix regeneration step exists. `Cargo.lock` is read directly by `buildRustPackage` or `crane`.

## Common Failure Modes

### 1. Hash mismatch during `nix build` (Go projects)

**Cause:** `gomod2nix.toml` is stale.
**Fix:** `just deps`

### 2. "command not found: gomod2nix"

**Cause:** Running outside the Nix devshell.
**Fix:** Use `nix develop --command gomod2nix` or enter the shell with `nix develop`.

### 3. Build fails after updating Go version

**Cause:** gomod2nix was run with a different Go version, producing different hashes.
**Fix:** Run `just deps` inside the correct devshell (which pins the Go version).

### 4. Flake input not found

**Cause:** Missing or outdated flake.lock.
**Fix:** `nix flake update` or `nix flake lock --update-input <name>`.

### 5. Overlay not applied

**Cause:** `buildGoApplication` not available because overlay was not applied.
**Fix:** Ensure `overlays = [ go.overlays.default ]` is in the nixpkgs import.

### 6. GPG signing failure on commit

**Cause:** GPG agent is locked.
**Fix:** Ask the user to unlock their GPG agent. Do NOT bypass with `--no-gpg-sign`.

## Dependency Update Checklist

When asked to update dependencies in a Go+Nix project, follow this checklist:

1. Read the justfile to find the correct targets
2. Run `just deps` (or equivalent `go mod tidy` + `gomod2nix`)
3. Verify `gomod2nix.toml` was modified (check `git diff`)
4. Run `just build` to verify the Nix build succeeds
5. Run `just test` to verify tests pass
6. Stage all three files: `go.mod`, `go.sum`, `gomod2nix.toml`

## FlakeHub Publishing

All Nix-backed projects publish to FlakeHub on every push to master via GitHub Actions. No API keys are needed — authentication uses GitHub OIDC tokens.

### Setting Up FlakeHub CI for a New Project

1. Ensure the flake builds locally: `just build` or `nix build`
2. Copy `examples/flakehub-workflow.yml` to `.github/workflows/nix.yml`
3. Update the `name` field in `flakehub-push` to `"friedenberg/<project-name>"`
4. Update `directory` if the flake is in a subdirectory (e.g., `./go`)
5. Push to master — the workflow publishes automatically

### Workflow Structure

Every project uses a two-job workflow:

| Job | Purpose |
|-----|---------|
| `flakehub-publish` | Publishes the flake to FlakeHub with rolling releases |
| `build-nix-package` | Builds across x86_64-linux, x86_64-darwin, aarch64-darwin with FlakeHub cache |

### Key Actions

- **`DeterminateSystems/nix-installer-action@main`** — Installs Determinate Nix (flakes enabled)
- **`DeterminateSystems/flakehub-push@main`** — Publishes flake to FlakeHub
- **`DeterminateSystems/flakehub-cache-action@main`** — Binary cache for CI builds

### Required Permissions

Both jobs need OIDC for FlakeHub authentication:

```yaml
permissions:
  id-token: "write"
  contents: "read"
```

### FlakeHub URLs in Flake Inputs

FlakeHub-hosted flakes can be used as inputs:

```nix
utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
```

Use FlakeHub URLs for third-party flakes (`flake-utils`, `crane`, `fenix`). Use GitHub URLs for devenvs and pinned nixpkgs SHAs.

### FlakeHub CLI (`fh`)

Available in the `devenvs/nix` devshell:

```bash
fh add numtide/flake-utils              # Add a flake input
fh add --input-name utils numtide/flake-utils  # With custom input name
fh search "flake-utils"                 # Search for flakes
fh status                               # Check auth status
```

## New Project Setup

When creating a new Nix-backed project:

1. Consult `references/flake-conventions.md` for the flake template
2. For Go projects, consult `references/go-nix-workflow.md` for gomod2nix integration
3. Set up FlakeHub CI using `references/flakehub-ci.md` and `examples/flakehub-workflow.yml`

## Additional Resources

### Reference Files

For detailed patterns and advanced techniques, consult:
- **`references/go-nix-workflow.md`** — Complete gomod2nix lifecycle, buildGoApplication options, multi-binary projects, version injection, postInstall hooks
- **`references/flake-conventions.md`** — Full flake templates for Go and Rust, devenv inheritance, stable-first nixpkgs rationale, MCP server installation pattern
- **`references/flakehub-ci.md`** — FlakeHub publishing, GitHub Actions workflow, OIDC authentication, multi-platform builds, FlakeHub CLI usage, common CI failures

### Example Files

Working templates in `examples/`:
- **`examples/go-flake.nix`** — Canonical Go project flake with buildGoApplication
- **`examples/go-justfile`** — Standard justfile for Go+Nix projects
- **`examples/flakehub-workflow.yml`** — Canonical GitHub Actions workflow for FlakeHub publishing and multi-platform builds
