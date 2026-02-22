# Go + Nix Workflow Reference

Detailed reference for building Go projects with Nix using gomod2nix. This covers the full lifecycle from project setup through dependency management, building, and common patterns.

## gomod2nix.toml Format

The file uses schema version 3 (current). Each Go module gets an entry with its version and Nix-compatible SHA256 hash:

```toml
schema = 3

[mod]
  [mod.'github.com/BurntSushi/toml']
    version = 'v1.3.2'
    hash = 'sha256-FIwyH67KryRWI9Bk4R8s1zFP0IgKR4L66wNQJYQZLeg='

  [mod.'golang.org/x/sys']
    version = 'v0.32.0'
    hash = 'sha256-6Kbezz1PBjGTHIGnMKgpK8jVYmThbNXB+CKKFjfqG80='
```

Both direct and indirect (transitive) dependencies are listed. The `gomod2nix` command reads `go.sum` and produces this file automatically.

## buildGoApplication Options

The full set of commonly used attributes for `pkgs.buildGoApplication`:

```nix
pkgs.buildGoApplication {
  # Required
  pname = "project-name";
  version = "0.1.0";
  src = ./.;
  modules = ./gomod2nix.toml;

  # Optional: build specific binaries
  subPackages = [ "cmd/binary" ];

  # Optional: inject version at build time
  ldflags = [
    "-X main.version=${version}"
    "-X main.commit=${self.shortRev or "dirty"}"
  ];

  # Optional: override Go version
  go = pkgs.go_1_25;
  GOTOOLCHAIN = "local";

  # Optional: disable CGO for static binary
  CGO_ENABLED = "0";

  # Optional: build-time dependencies
  nativeBuildInputs = with pkgs; [ scdoc ];

  # Optional: post-build installation steps
  postInstall = ''
    mkdir -p $out/share/man/man1
    $out/bin/binary genman $out/share/man/man1
  '';

  # Optional: metadata
  meta = with pkgs.lib; {
    description = "Project description";
    homepage = "https://github.com/org/project";
    license = licenses.mit;
  };
}
```

## Multi-Binary Projects

For projects with multiple commands under `cmd/`:

```
project/
├── cmd/
│   ├── server/
│   │   └── main.go
│   └── cli/
│       └── main.go
├── go.mod
├── go.sum
└── gomod2nix.toml
```

Build specific binaries with `subPackages`:

```nix
# Build only the server
subPackages = [ "cmd/server" ];

# Build both
subPackages = [ "cmd/server" "cmd/cli" ];
```

Without `subPackages`, all packages with a `main` function are built.

## Version Injection with ldflags

Embed version information at build time:

```nix
version = "0.1.0";

myApp = pkgs.buildGoApplication {
  inherit version;
  ldflags = [
    "-X main.version=${version}"
  ];
  # ...
};
```

Access in Go code:

```go
var version = "dev"

func main() {
    fmt.Println("version:", version)
}
```

## postInstall Hooks

Common postInstall patterns:

### Generate man pages
```nix
postInstall = ''
  mkdir -p $out/share/man/man1
  $out/bin/tool genman $out/share/man/man1
'';
```

### Generate shell completions
```nix
postInstall = ''
  mkdir -p $out/share/bash-completion/completions
  $out/bin/tool completion bash > $out/share/bash-completion/completions/tool

  mkdir -p $out/share/zsh/site-functions
  $out/bin/tool completion zsh > $out/share/zsh/site-functions/_tool
'';
```

### Generate MCP plugin manifest
```nix
postInstall = ''
  mkdir -p $out/share
  $out/bin/tool gen-manifest > $out/share/mcp-manifest.json
'';
```

## The Overlay Chain

How `buildGoApplication` becomes available:

1. **Go devenv** (`devenvs/go/flake.nix`) imports gomod2nix and exports its overlay
2. **Project flake** imports the Go devenv and applies the overlay:
   ```nix
   pkgs = import nixpkgs {
     inherit system;
     overlays = [ go.overlays.default ];
   };
   ```
3. `pkgs.buildGoApplication` and `pkgs.gomod2nix` are now available

Without step 2, `pkgs.buildGoApplication` does not exist and the build fails with an attribute error.

## Go Version Pinning

Some projects pin a specific Go version:

```nix
myApp = pkgs.buildGoApplication {
  go = pkgs.go_1_25;
  GOTOOLCHAIN = "local";
  # ...
};
```

`GOTOOLCHAIN = "local"` prevents the Go toolchain from downloading a different version at build time. This is important for reproducibility.

When the Go version changes:
1. Update `go` directive in `go.mod`
2. Update `go = pkgs.go_1_XX` in `flake.nix`
3. Run `just deps` to regenerate hashes with the new toolchain

## Pseudo-Versions

Go modules without proper releases use pseudo-versions:

```
v0.0.0-20260215160001-e634f96c4717
```

Format: `v0.0.0-<timestamp>-<commit-hash>`

These are normal in `gomod2nix.toml` and `go.mod` for:
- Unpublished internal modules
- Pre-release dependencies
- Modules using `replace` directives

## Dependency Update Workflow (Detailed)

### Adding a new dependency

```bash
# 1. Add to Go code (import the package)
# 2. Run deps to sync everything
just deps

# Equivalent manual steps:
nix develop --command go get github.com/new/package
nix develop --command go mod tidy
nix develop --command gomod2nix
```

### Updating a single dependency

```bash
nix develop --command go get -u github.com/some/package
just deps
```

### Updating all dependencies

```bash
nix develop --command go get -u ./...
just deps
```

### Removing a dependency

```bash
# 1. Remove imports from Go code
# 2. Run deps to clean up
just deps
```

## Debugging Build Failures

### Reading the error

Nix build errors for Go projects typically look like:

```
error: hash mismatch in fixed-output derivation '/nix/store/...-source':
  specified: sha256-OLD_HASH
  got:       sha256-NEW_HASH
```

This means `gomod2nix.toml` has the old hash but the actual module content produces a different hash. Run `just deps` to fix.

### Checking for staleness

Compare timestamps:

```bash
# If go.mod is newer than gomod2nix.toml, it's likely stale
ls -la go.mod go.sum gomod2nix.toml
```

Or check git diff:

```bash
git diff go.mod   # If modified, gomod2nix.toml needs regeneration
```

### Verbose build output

```bash
nix build --show-trace
```

## Aggregation Pattern

For projects that aggregate multiple Go binaries (like a plugin marketplace):

```nix
packages.default = pkgs.symlinkJoin {
  name = "marketplace";
  paths = [
    grit.packages.${system}.default
    lux.packages.${system}.default
  ];
};
```

Each sub-project manages its own `gomod2nix.toml` independently.
