
cmd_nix_dev := "nix develop " + justfile_directory() + " --command "

default: build-nix-gomod2nix build test

# Build all packages (default = marketplace bundle)
build: build-nix-gomod2nix
    nix build

build-purse-first:
    nix build .#purse-first

build-purse-first-cli:
    nix build .#purse-first -o result-cli

# Build marketplace without hooks
build-no-hooks:
    nix build .#marketplace-no-hooks

build-go:
    {{cmd_nix_dev}} go build -o build/purse-first ./cmd/purse-first

# Install this marketplace's packages into Claude Code
install: build-purse-first-cli
    nix build
    {{justfile_directory()}}/result-cli/bin/purse-first install {{justfile_directory()}}/result

update: update-nix

update-nix:
    nix flake update

# Run Go tests
test-go:
    {{cmd_nix_dev}} go test ./...

# Run Go tests with verbose output
test-v:
    {{cmd_nix_dev}} go test -v ./...

# Run Go tests with race detector
test-race:
    {{cmd_nix_dev}} go test -race ./...

# Test go-mcp library
test-go-mcp:
    {{cmd_nix_dev}} go test -v ./libs/go-mcp/...

# Test dewey library (all layers including build-tag-gated test helpers)
test-dewey:
    {{cmd_nix_dev}} go test -tags test ./libs/dewey/...

# Build dewey library (all layers + CLI tools)
build-dewey:
    {{cmd_nix_dev}} go build ./libs/dewey/...

# Vet dewey library
vet-dewey:
    {{cmd_nix_dev}} go vet -tags test ./libs/dewey/...

# Build one dewey analyzer (defererr|repool|seqerror) and run it via -vettool
analyze-dewey name:
    {{cmd_nix_dev}} go build -o build/{{name}} ./libs/dewey/cmd/{{name}}
    {{cmd_nix_dev}} go vet -vettool={{justfile_directory()}}/build/{{name}} -tags test ./libs/dewey/...

# Run all three dewey static analyzers
analyze-dewey-all: (analyze-dewey "defererr") (analyze-dewey "repool") (analyze-dewey "seqerror")

# Test rust-mcp library
test-rust-mcp:
    cd libs/rust-mcp && {{cmd_nix_dev}} cargo test

# Format code
fmt:
    {{cmd_nix_dev}} go fmt ./...

# Lint code
lint:
    {{cmd_nix_dev}} go vet ./...

# Sync go.work and regenerate gomod2nix.toml from go.mod / go.sum / go.work.
# Run this whenever go dependencies change. Cheap and idempotent.
#
# Targets are passed explicitly because gomod2nix's flake auto-discovery
# silently fails to find any targets under CI's nix (Determinate v3.19),
# causing it to fall through to host-only generation — the original
# #69 symptom. amarbel-llc/gomod2nix#8 silenced an earlier panic but
# did not actually make discovery work in that environment. Tracking
# the underlying issue separately; keep this list aligned with the
# matrix systems built by CI.
build-nix-gomod2nix:
    {{cmd_nix_dev}} go work sync
    {{cmd_nix_dev}} gomod2nix \
        --target linux/amd64 \
        --target linux/arm64 \
        --target darwin/amd64 \
        --target darwin/arm64

# Update go dependencies, tidy all modules, and refresh gomod2nix.toml
deps: build-nix-gomod2nix

# Run BATS integration tests
test-integration: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap --jobs {{num_cpus()}} \
      zz-tests_bats/validate_marketplace.bats \
      zz-tests_bats/validate_documents.bats \
      zz-tests_bats/validate_mcp.bats

# Run validate-specific BATS tests
test-validate: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap --jobs {{num_cpus()}} zz-tests_bats/validate_documents.bats

# Run lifecycle tests
test-lifecycle: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap --jobs {{num_cpus()}} zz-tests_bats/hook_lifecycle.bats

# Run MCP validation tests
test-validate-mcp: build-purse-first-cli
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap zz-tests_bats/validate_mcp.bats

# Run package brew tests
test-package-brew: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap --jobs {{num_cpus()}} zz-tests_bats/package_brew.bats

# Validate own plugin manifest
validate:
    {{cmd_nix_dev}} go run ./cmd/purse-first validate .claude-plugin/plugin.json

# Run template tests
test-template:
    {{cmd_nix_dev}} bats --tap --jobs {{num_cpus()}} zz-tests_bats/marketplace_template.bats

# Verify mkMarketplace.nix parses
check-lib:
    nix-instantiate --parse lib/mkMarketplace.nix

# Verify template parses
check-template:
    nix-instantiate --parse templates/marketplace/flake.nix

test: \
    test-go \
    test-go-mcp \
    test-dewey \
    test-rust-mcp \
    test-integration \
    test-lifecycle \
    test-package-brew \
    test-template

# Bump version for a package. Usage: just bump-version grit 0.2.0
bump-version package version:
  #!/usr/bin/env bash
  set -euo pipefail
  # Update marketplace-config.json (source of truth)
  jq --arg pkg "{{package}}" --arg ver "{{version}}" \
    '.plugins[$pkg].version = $ver' marketplace-config.json > marketplace-config.json.tmp
  mv marketplace-config.json.tmp marketplace-config.json
  gum log --level info "{{package}}: version bumped to {{version}}"
  gum log --level warn "Remember to update Cargo.toml and SKILL.md frontmatter if applicable"

# Tag a library release. Usage: just tag-lib go-mcp 0.0.11 "feat: add EmbeddedResource types"
tag-lib lib version message:
  #!/usr/bin/env bash
  set -euo pipefail
  tag="libs/{{lib}}/v{{version}}"
  prev=$(git tag --sort=-v:refname -l "libs/{{lib}}/v*" | head -1)
  if [[ -n "$prev" ]]; then
    gum log --level info "Previous: $prev"
    git log --oneline "$prev"..HEAD -- "libs/{{lib}}/"
  fi
  git tag -s -m "{{message}}" "$tag"
  gum log --level info "Created tag: $tag"
  git push origin "$tag"
  gum log --level info "Pushed $tag"
  git tag -v "$tag"

[group('explore')]
explore-dagnabit-export:
    #!/usr/bin/env bash
    set -euo pipefail
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    gum log --level info "Copying dewey to $tmp"
    cp -r libs/dewey/* "$tmp/"
    cd "$tmp"
    # Move NATO levels into internal/
    mkdir -p internal
    for d in 0 alfa bravo charlie delta echo foxtrot golf; do
      [[ -d "$d" ]] && mv "$d" internal/
    done
    # Rewrite import paths
    for level in 0 alfa bravo charlie delta echo foxtrot golf; do
      find . -name '*.go' -exec sed -i "s|libs/dewey/${level}/|libs/dewey/internal/${level}/|g" {} +
    done
    # Ensure go 1.26 toolchain is used (generic function aliases require it)
    sed -i '/^go 1.26$/a toolchain go1.26.1' go.mod
    gum log --level info "Verifying internal/ compiles"
    GOWORK=off go build ./internal/... 2>&1
    gum log --level info "Running dagnabit export"
    GOWORK=off "{{justfile_directory()}}/build/dagnabit" export ./internal/... 2>&1
    gum log --level info "Building pkgs/"
    # Try building; on failure, isolate one package for debugging
    if ! GOWORK=off go build ./pkgs/... 2>&1; then
      gum log --level warn "Full build failed. Isolating pkgs/reset..."
      gum log --level info "go env:"
      GOWORK=off go env GOVERSION GOTOOLCHAIN GOWORK 2>&1
      gum log --level info "go.mod head:"
      head -5 go.mod
      gum log --level info "pkgs/reset/main.go:"
      cat pkgs/reset/main.go
      gum log --level info "Testing equivalent standalone module..."
      mkdir -p /tmp/dagnabit-mvp/{internal/0/reset,pkgs/reset}
      cp internal/0/reset/main.go /tmp/dagnabit-mvp/internal/0/reset/
      cp pkgs/reset/main.go /tmp/dagnabit-mvp/pkgs/reset/
      sed -i "s|github.com/amarbel-llc/purse-first/libs/dewey|dagnabit-mvp|g" /tmp/dagnabit-mvp/pkgs/reset/main.go /tmp/dagnabit-mvp/internal/0/reset/main.go
      cp go.mod /tmp/dagnabit-mvp/go.mod
      sed -i "s|github.com/amarbel-llc/purse-first/libs/dewey|dagnabit-mvp|g" /tmp/dagnabit-mvp/go.mod
      cp go.sum /tmp/dagnabit-mvp/go.sum
      cd /tmp/dagnabit-mvp
      # Strip toolchain directive to test if that's the cause
      sed -i '/^toolchain/d' go.mod
      GOWORK=off go mod tidy 2>&1
      gum log --level info "MVP go.mod (no toolchain):"
      head -5 go.mod
      GOWORK=off go build ./pkgs/reset/ 2>&1 && gum log --level info "MVP builds OK (no toolchain)" || gum log --level error "MVP also fails (no toolchain)"
      exit 1
    fi
    gum log --level info "All pkgs/ facades compile"

# Clean build artifacts
clean:
    rm -f purse-first
    rm -rf build/
    rm -rf result result-cli
