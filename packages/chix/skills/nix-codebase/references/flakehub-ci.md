# FlakeHub & CI Reference

Detailed reference for publishing Nix flakes to FlakeHub and configuring GitHub Actions CI for Nix-backed projects.

## What is FlakeHub

FlakeHub (https://flakehub.com) is a registry for Nix flakes by Determinate Systems. It provides:

- **Flake hosting and discovery** — publish flakes with semantic versioning or rolling releases
- **Binary caching** — `flakehub-cache-action` caches Nix store paths across CI runs
- **OIDC authentication** — GitHub Actions authenticate via OIDC tokens (no secrets needed)
- **Pinned inputs** — flake.lock resolves FlakeHub URLs to pinned API tarballs

## FlakeHub Naming Convention

Flakes are published as `<github-username>/<flake-name>`:

| Repo | FlakeHub Name | Notes |
|------|---------------|-------|
| `friedenberg/eng` | `friedenberg/eng` | Monorepo, published from root |
| `amarbel-llc/dodder` | `friedenberg/dodder-go` | Subdirectory flake, suffixed |
| `amarbel-llc/lux` | `friedenberg/lux` | Standard single-flake repo |

When the flake lives in a subdirectory, the `directory` parameter in the push action points to it, and the FlakeHub name may include a suffix to distinguish it.

## GitHub Actions Workflow

Every Nix-backed project uses a two-job workflow: one to publish to FlakeHub, one to build across platforms.

### Workflow File Location

`.github/workflows/nix.yml`

### Trigger

Publish on every push to `master`:

```yaml
on:
  push:
    branches:
      - master
```

### Job 1: FlakeHub Publish

```yaml
flakehub-publish:
  runs-on: "ubuntu-latest"
  permissions:
    id-token: "write"
    contents: "read"
  steps:
    - uses: "actions/checkout@v4"
    - uses: "DeterminateSystems/nix-installer-action@main"
      with:
        determinate: true
    - uses: "DeterminateSystems/flakehub-push@main"
      with:
        name: "<github-username>/<flake-name>"
        rolling: true
        visibility: "public"
        include-output-paths: true
        directory: ./
```

Key parameters:

| Parameter | Value | Purpose |
|-----------|-------|---------|
| `name` | `"friedenberg/<name>"` | FlakeHub flake identifier |
| `rolling` | `true` | Creates rolling release tags (no manual version bumps) |
| `visibility` | `"public"` | Public flake — anyone can use it as an input |
| `include-output-paths` | `true` | Includes Nix output paths in the release metadata |
| `directory` | `./` or `./go` etc. | Directory containing the `flake.nix` to publish |

### Job 2: Multi-Platform Build

```yaml
build-nix-package:
  strategy:
    matrix:
      include:
        - os: ubuntu-22.04
          system: x86_64-linux
        - os: macos-14
          system: x86_64-darwin
        - os: macos-15
          system: aarch64-darwin
  runs-on: ${{ matrix.os }}
  permissions:
    contents: read
    id-token: write
  steps:
    - uses: actions/checkout@v4
    - uses: DeterminateSystems/nix-installer-action@main
      with:
        determinate: true
    - uses: DeterminateSystems/flakehub-cache-action@main
    - run: nix build
```

Key actions:

| Action | Purpose |
|--------|---------|
| `DeterminateSystems/nix-installer-action@main` | Installs Nix with Determinate Nix (flakes enabled by default) |
| `DeterminateSystems/flakehub-cache-action@main` | Configures FlakeHub binary cache for faster CI builds |

### Required Permissions

Both jobs require:

```yaml
permissions:
  id-token: "write"    # OIDC token for FlakeHub authentication
  contents: "read"     # Read repo contents
```

The `id-token: write` permission is critical — FlakeHub uses GitHub's OIDC provider for authentication. No API keys or secrets are needed.

## Platform Matrix

Standard matrix across all projects:

| Runner | Nix System | Notes |
|--------|------------|-------|
| `ubuntu-22.04` | `x86_64-linux` | Primary Linux build |
| `macos-14` | `x86_64-darwin` | Intel Mac |
| `macos-15` | `aarch64-darwin` | Apple Silicon |

Some older projects use `macos-13` for x86_64-darwin — prefer `macos-14` or later for new projects.

## Subdirectory Flakes

When the flake lives in a subdirectory (e.g., `./go` in dodder):

1. Set `directory` in the `flakehub-push` action:
   ```yaml
   directory: ./go
   ```

2. Set `working-directory` default for build job:
   ```yaml
   defaults:
     run:
       working-directory: ./go
   ```

3. Or set `working-directory` on individual steps:
   ```yaml
   - run: nix build
     working-directory: ./go
   ```

## FlakeHub URLs as Flake Inputs

FlakeHub-hosted flakes can be referenced by URL in `flake.nix` inputs:

```nix
inputs = {
  # FlakeHub URL format: https://flakehub.com/f/<owner>/<name>/<version>
  utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

  # Tarball format (specific version):
  fh.url = "https://flakehub.com/f/DeterminateSystems/fh/0.1.21.tar.gz";
};
```

In `flake.lock`, these resolve to pinned API URLs:
```
https://api.flakehub.com/f/pinned/numtide/flake-utils/0.1.102/...
```

### When to Use FlakeHub URLs

- **Third-party flakes** available on FlakeHub: `flake-utils`, `crane`, `fenix`, `fh`
- **Your own published flakes** once they're on FlakeHub

### When to Use GitHub URLs

- **Devenv references**: `github:friedenberg/eng?dir=devenvs/go`
- **Nixpkgs pinned SHAs**: `github:NixOS/nixpkgs/<sha>`
- **Unpublished repos** not yet on FlakeHub

## FlakeHub CLI (`fh`)

The `fh` CLI is available in the `devenvs/nix` devshell and provides FlakeHub operations:

### Add a flake input

```bash
# Add flake-utils from FlakeHub
fh add numtide/flake-utils

# Add with specific input name
fh add --input-name utils numtide/flake-utils

# Add with version constraint
fh add "NixOS/nixpkgs/0.2411.*"
```

### Search for flakes

```bash
fh search "flake-utils"
```

### List releases

```bash
fh list releases NixOS/nixpkgs
```

### Check status

```bash
fh status
```

## Setting Up FlakeHub for a New Project

### 1. Ensure the flake builds

```bash
nix build
nix flake check
```

### 2. Create the GitHub Actions workflow

Copy `examples/flakehub-workflow.yml` to `.github/workflows/nix.yml` and update:

- `name` in `flakehub-push` → your FlakeHub flake name
- `directory` → path to your `flake.nix` (usually `./`)
- Build matrix runners if needed

### 3. Push to master

The workflow triggers on push to master. No FlakeHub account setup or API keys are needed — OIDC handles authentication automatically.

### 4. Verify

Check `https://flakehub.com/flake/<owner>/<name>` for your published flake.

## Justfile Integration

Add CI-related targets to the justfile:

```
# Run the same build that CI runs
ci: build test

# Check flake health
check-flake:
    nix flake check
    nix flake show
```

## Common CI Failures

### 1. OIDC authentication failure

**Cause:** Missing `id-token: write` permission.
**Fix:** Add to job permissions:
```yaml
permissions:
  id-token: "write"
  contents: "read"
```

### 2. Build fails on macOS but not Linux

**Cause:** Missing Darwin-specific build inputs.
**Fix:** Add conditional buildInputs:
```nix
buildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin [
  pkgs.darwin.apple_sdk.frameworks.Security
  pkgs.darwin.apple_sdk.frameworks.SystemConfiguration
];
```

### 3. Cache not working

**Cause:** `flakehub-cache-action` not included or `id-token: write` missing from build job.
**Fix:** Ensure the build job has both the cache action and OIDC permission.

### 4. Wrong flake published (monorepo)

**Cause:** `directory` parameter not set correctly.
**Fix:** Set `directory` to the subdirectory containing the target `flake.nix`.
