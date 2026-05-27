
cmd_nix_dev := "nix develop " + justfile_directory() + " --command "

# `just` = CI gate. Aggregates only — see eng-design_patterns-justfile(7)
# §DEFAULT RECIPE. Pre-merge hook (`merge-this-session`) runs this; if
# it passes, the project is in a good state.
default: validate lint build test

# ──── validate ──────────────────────────────────────────────────────
# Pre-build correctness checks. Hard failures (parser, schema, drift).

[group('pre-build')]
validate: validate-nix validate-purse-first-manifest

# `nix flake check` evaluates every flake check, including
# `checks.treefmt` (catches formatter drift across the whole tree)
# and the per-package builds the flake exposes.
[group('pre-build')]
validate-nix:
    nix flake check

# Validate the marketplace's own plugin manifest.
[group('pre-build')]
validate-purse-first-manifest:
    {{ cmd_nix_dev }} go run ./cmd/purse-first validate .claude-plugin/plugin.json

# Verify mkMarketplace.nix parses.
[group('pre-build')]
check-lib:
    nix-instantiate --parse lib/mkMarketplace.nix

# Verify template parses.
[group('pre-build')]
check-template:
    nix-instantiate --parse templates/marketplace/flake.nix

# ──── lint ──────────────────────────────────────────────────────────
# Read-only style / convention / drift checks. Does not modify code.

[group('pre-build')]
lint: lint-go lint-dewey-pkgs-drift

# `go vet` across the workspace.
[group('pre-build')]
lint-go:
    {{ cmd_nix_dev }} go vet ./...

# Drift lint: ensure libs/dewey/pkgs/ matches what `dagnabit export --library`
# (plus the integrated treefmt pass) produces from the current internal/ tree.
# Run from CI to catch stale facades before they merge. Depends on
# `dagnabit-build` so the binary under test is the one in the current
# working tree, and on `dewey-export-library` so the export actually runs.
[group('pre-build')]
lint-dewey-pkgs-drift: dagnabit-build dewey-export-library
    git diff --exit-code -- libs/dewey/pkgs/

# Vet dewey library.
[group('pre-build')]
vet-dewey:
    {{ cmd_nix_dev }} go vet -tags test ./libs/dewey/...

# Build one dewey analyzer (defererr|repool|seqerror) and run it via -vettool.
[group('pre-build')]
analyze-dewey name:
    {{ cmd_nix_dev }} go build -o build/{{ name }} ./libs/dewey/cmd/{{ name }}
    {{ cmd_nix_dev }} go vet -vettool={{ justfile_directory() }}/build/{{ name }} -tags test ./libs/dewey/...

# Run all three dewey static analyzers.
[group('pre-build')]
analyze-dewey-all: (analyze-dewey "defererr") (analyze-dewey "repool") (analyze-dewey "seqerror")

# ──── build ─────────────────────────────────────────────────────────
# Compile / generate artifacts.

[group('build')]
build: build-nix-gomod2nix build-marketplace

# Build the marketplace bundle (default Nix output).
[group('build')]
build-marketplace:
    nix build

[group('build')]
build-purse-first:
    nix build .#purse-first

[group('build')]
build-purse-first-cli:
    nix build .#purse-first -o result-cli

# Build marketplace without hooks.
[group('build')]
build-no-hooks:
    nix build .#marketplace-no-hooks

[group('build')]
build-go:
    {{ cmd_nix_dev }} go build -o build/purse-first ./cmd/purse-first

# Build dewey library (all layers + CLI tools). Injects DEWEY_VERSION
# and the short commit into the buildinfo package via -ldflags, matching
# what the Nix derivations do for dev parity.
[group('build')]
build-dewey:
    #!/usr/bin/env bash
    set -euo pipefail
    . libs/dewey/version.env
    commit=$(git rev-parse --short HEAD 2>/dev/null || echo dirty)
    bi=github.com/amarbel-llc/purse-first/libs/dewey/internal/0/buildinfo
    {{ cmd_nix_dev }} go build \
      -ldflags "-X $bi.Version=$DEWEY_VERSION -X $bi.Commit=$commit" \
      ./libs/dewey/...

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
[group('build')]
build-nix-gomod2nix:
    {{ cmd_nix_dev }} go work sync
    {{ cmd_nix_dev }} go work vendor
    {{ cmd_nix_dev }} gomod2nix \
        --target linux/amd64 \
        --target linux/arm64 \
        --target darwin/amd64 \
        --target darwin/arm64

# Rebuild build/dagnabit from source. Not a dep of the dewey-* recipes
# because those need to work mid-bootstrap when cmd/dagnabit's imports
# may temporarily reference paths that don't compile yet. Run this
# manually when you've changed dagnabit's source.
[group('build')]
dagnabit-build:
    {{ cmd_nix_dev }} go build -o {{ justfile_directory() }}/build/dagnabit ./cmd/dagnabit

# ──── test ──────────────────────────────────────────────────────────
# Post-build: run test suites against built artifacts and source.

[group('post-build')]
test: \
    test-go \
    test-go-mcp \
    test-dewey \
    test-rust-mcp \
    test-integration \
    test-lifecycle \
    test-package-brew \
    test-template

# Run Go tests.
[group('post-build')]
test-go:
    {{ cmd_nix_dev }} go test ./...

# Run Go tests with verbose output.
[group('post-build')]
test-v:
    {{ cmd_nix_dev }} go test -v ./...

# Run Go tests with race detector.
[group('post-build')]
test-race:
    {{ cmd_nix_dev }} go test -race ./...

# Test go-mcp library.
[group('post-build')]
test-go-mcp:
    {{ cmd_nix_dev }} go test -v ./libs/go-mcp/...

# Test dewey library (all layers including build-tag-gated test helpers).
[group('post-build')]
test-dewey:
    {{ cmd_nix_dev }} go test -tags test ./libs/dewey/...

# Test rust-mcp library.
[group('post-build')]
test-rust-mcp:
    cd libs/rust-mcp && {{ cmd_nix_dev }} cargo test

# Run BATS integration tests.
[group('post-build')]
test-integration: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{ justfile_directory() }}/result-cli/bin/purse-first {{ cmd_nix_dev }} bats --tap --jobs {{ num_cpus() }} \
      zz-tests_bats/validate_marketplace.bats \
      zz-tests_bats/validate_documents.bats \
      zz-tests_bats/validate_mcp.bats

# Run validate-specific BATS tests.
[group('post-build')]
test-validate: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{ justfile_directory() }}/result-cli/bin/purse-first {{ cmd_nix_dev }} bats --tap --jobs {{ num_cpus() }} zz-tests_bats/validate_documents.bats

# Run lifecycle tests.
[group('post-build')]
test-lifecycle: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{ justfile_directory() }}/result-cli/bin/purse-first {{ cmd_nix_dev }} bats --tap --jobs {{ num_cpus() }} zz-tests_bats/hook_lifecycle.bats

# Run MCP validation tests.
[group('post-build')]
test-validate-mcp: build-purse-first-cli
    PURSE_FIRST_BIN={{ justfile_directory() }}/result-cli/bin/purse-first {{ cmd_nix_dev }} bats --tap zz-tests_bats/validate_mcp.bats

# Run package brew tests.
[group('post-build')]
test-package-brew: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{ justfile_directory() }}/result-cli/bin/purse-first {{ cmd_nix_dev }} bats --tap --jobs {{ num_cpus() }} zz-tests_bats/package_brew.bats

# Run template tests.
[group('post-build')]
test-template:
    {{ cmd_nix_dev }} bats --tap --jobs {{ num_cpus() }} zz-tests_bats/marketplace_template.bats

# ──── operational ───────────────────────────────────────────────────
# Side-effects on the running system.

# Install this marketplace's packages into Claude Code.
[group('operational')]
install: build-purse-first-cli
    nix build
    {{ justfile_directory() }}/result-cli/bin/purse-first install {{ justfile_directory() }}/result

# ──── codemod ───────────────────────────────────────────────────────
# Modifies source code. Prefer `nix fmt` for the repo-wide treefmt
# pass (covers Go via gofumpt, Nix via nixfmt, shell via shfmt).

# `go fmt ./...` for a quick Go-only reformat. The canonical
# repo-wide formatter is `nix fmt` (treefmt-nix wrapper).
[group('codemod')]
fmt:
    {{ cmd_nix_dev }} go fmt ./...

# Dry-run a single-package rename: print the proposed move as NDJSON,
# touch nothing on disk. `new_leaf` is optional; defaults to src's leaf.
[group('codemod')]
dewey-rename pkg new_leaf="":
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit rename -n {{ pkg }} {{ new_leaf }}

# Apply a single-package rename. Emits an NDJSON `move` event on success.
[group('codemod')]
dewey-rename-apply pkg new_leaf="":
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit rename {{ pkg }} {{ new_leaf }}

# Dry-run a full reposition of libs/dewey/internal/. Prints NDJSON
# `would-move` events for each package that needs repositioning.
[group('codemod')]
dewey-reposition:
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit -n internal

# Apply a full reposition of libs/dewey/internal/. Real moves print
# NDJSON `move` events as they happen.
[group('codemod')]
dewey-reposition-apply:
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit internal

# Generate a public facade in libs/dewey/pkgs/ for a single internal
# package. `pkg` is the path inside libs/dewey, e.g. `internal/0/go_module`.
[group('codemod')]
dewey-export pkg:
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit export ./{{ pkg }}

# Generate pkgs/ facades for every package under libs/dewey/internal/ (library mode).
# Fails if any //go:generate dagnabit export directives are found.
[group('codemod')]
dewey-export-library *flags:
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit export --library {{ flags }}

# ──── maintenance ───────────────────────────────────────────────────
# Refresh dependencies, bump versions, tag/release, clean.

[group('maintenance')]
update: update-nix

[group('maintenance')]
update-nix:
    nix flake update

# Update go dependencies, tidy all modules, and refresh gomod2nix.toml.
[group('maintenance')]
deps: build-nix-gomod2nix

# Clean build artifacts.
[group('maintenance')]
clean:
    rm -f purse-first
    rm -rf build/
    rm -rf result result-cli

# Per eng-versioning(7): each independently-versioned target (purse-first
# repo, libs/dewey, libs/go-mcp, libs/rust-mcp) has its own bump-version /
# tag / release triple reading from its own version.env.

# Rewrite PURSE_FIRST_VERSION in version.env. Pure mutation — release owns
# the commit and tag steps.
[group('maintenance')]
bump-version-purse-first new_version:
    sed -E -i 's/^(export PURSE_FIRST_VERSION)=.*/\1={{ new_version }}/' version.env

# Read PURSE_FIRST_VERSION from version.env, create a signed annotated
# tag v<sem>, push, and verify.
[group('maintenance')]
tag-purse-first message:
    #!/usr/bin/env bash
    set -euo pipefail
    . version.env
    tag="v${PURSE_FIRST_VERSION:?missing PURSE_FIRST_VERSION in version.env}"
    git tag -s -m "{{ message }}" "$tag"
    gum log --level info "Created tag: $tag"
    git push origin "$tag"
    gum log --level info "Pushed $tag"
    git tag -v "$tag"

# Full purse-first release flow: changelog → bump → commit → tag → gh release.
[group('maintenance')]
release-purse-first new_version:
    #!/usr/bin/env bash
    set -euo pipefail
    branch=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$branch" != "master" ]]; then
        gum log --level error "release-purse-first only allowed from master (on '$branch')"
        exit 1
    fi
    prev=$(git tag --sort=-v:refname -l "v*" | grep -E '^v[0-9]' | head -1 || true)
    header="release v{{ new_version }}"
    if [[ -n "$prev" ]]; then
        summary=$(git log --format='- %s' "$prev"..HEAD)
        msg="$header"$'\n\n'"$summary"
    else
        msg="$header"
    fi
    just bump-version-purse-first "{{ new_version }}"
    git add version.env
    git commit -m "$header"
    just tag-purse-first "$msg"
    gh release create "v{{ new_version }}" --title "$header" --notes "$msg"

# Rewrite DEWEY_VERSION in libs/dewey/version.env. Pure mutation.
[group('maintenance')]
bump-version-dewey new_version:
    sed -E -i 's/^(export DEWEY_VERSION)=.*/\1={{ new_version }}/' libs/dewey/version.env

# Read DEWEY_VERSION, create signed annotated tag libs/dewey/v<sem>, push,
# and verify. Tag prefix matches the sub-module path so the Go module proxy
# resolves it.
[group('maintenance')]
tag-dewey message:
    #!/usr/bin/env bash
    set -euo pipefail
    . libs/dewey/version.env
    tag="libs/dewey/v${DEWEY_VERSION:?missing DEWEY_VERSION in libs/dewey/version.env}"
    git tag -s -m "{{ message }}" "$tag"
    gum log --level info "Created tag: $tag"
    git push origin "$tag"
    gum log --level info "Pushed $tag"
    git tag -v "$tag"

# Full dewey release flow. Changelog filters commits touching libs/dewey/.
[group('maintenance')]
release-dewey new_version:
    #!/usr/bin/env bash
    set -euo pipefail
    branch=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$branch" != "master" ]]; then
        gum log --level error "release-dewey only allowed from master (on '$branch')"
        exit 1
    fi
    prev=$(git tag --sort=-v:refname -l "libs/dewey/v*" | head -1)
    header="release libs/dewey/v{{ new_version }}"
    if [[ -n "$prev" ]]; then
        summary=$(git log --format='- %s' "$prev"..HEAD -- libs/dewey/)
        if [[ -n "$summary" ]]; then
            msg="$header"$'\n\n'"$summary"
        else
            msg="$header"
        fi
    else
        msg="$header"
    fi
    just bump-version-dewey "{{ new_version }}"
    git add libs/dewey/version.env
    git commit -m "$header"
    just tag-dewey "$msg"
    gh release create "libs/dewey/v{{ new_version }}" --title "$header" --notes "$msg"

# Rewrite GO_MCP_VERSION in libs/go-mcp/version.env. Pure mutation.
[group('maintenance')]
bump-version-go-mcp new_version:
    sed -E -i 's/^(export GO_MCP_VERSION)=.*/\1={{ new_version }}/' libs/go-mcp/version.env

# Read GO_MCP_VERSION, create signed annotated tag libs/go-mcp/v<sem>, push,
# and verify. Tag prefix matches the sub-module path so the Go module proxy
# resolves it.
[group('maintenance')]
tag-go-mcp message:
    #!/usr/bin/env bash
    set -euo pipefail
    . libs/go-mcp/version.env
    tag="libs/go-mcp/v${GO_MCP_VERSION:?missing GO_MCP_VERSION in libs/go-mcp/version.env}"
    git tag -s -m "{{ message }}" "$tag"
    gum log --level info "Created tag: $tag"
    git push origin "$tag"
    gum log --level info "Pushed $tag"
    git tag -v "$tag"

# Full go-mcp release flow. Changelog filters commits touching libs/go-mcp/.
[group('maintenance')]
release-go-mcp new_version:
    #!/usr/bin/env bash
    set -euo pipefail
    branch=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$branch" != "master" ]]; then
        gum log --level error "release-go-mcp only allowed from master (on '$branch')"
        exit 1
    fi
    prev=$(git tag --sort=-v:refname -l "libs/go-mcp/v*" | head -1)
    header="release libs/go-mcp/v{{ new_version }}"
    if [[ -n "$prev" ]]; then
        summary=$(git log --format='- %s' "$prev"..HEAD -- libs/go-mcp/)
        if [[ -n "$summary" ]]; then
            msg="$header"$'\n\n'"$summary"
        else
            msg="$header"
        fi
    else
        msg="$header"
    fi
    just bump-version-go-mcp "{{ new_version }}"
    git add libs/go-mcp/version.env
    git commit -m "$header"
    just tag-go-mcp "$msg"
    gh release create "libs/go-mcp/v{{ new_version }}" --title "$header" --notes "$msg"

# Rewrite RUST_MCP_VERSION in libs/rust-mcp/version.env AND the [package]
# version in libs/rust-mcp/Cargo.toml in lock-step. The Cargo.toml sed
# anchors on the package-table version (first `version = "..."` after
# `[package]`) so dependency version pins are untouched.
[group('maintenance')]
bump-version-rust-mcp new_version:
    #!/usr/bin/env bash
    set -euo pipefail
    sed -E -i 's/^(export RUST_MCP_VERSION)=.*/\1={{ new_version }}/' libs/rust-mcp/version.env
    sed -E -i '/^\[package\]/,/^\[/{ s/^version = "[^"]*"$/version = "{{ new_version }}"/ }' libs/rust-mcp/Cargo.toml

# Read RUST_MCP_VERSION, create signed annotated tag libs/rust-mcp/v<sem>,
# push, and verify.
[group('maintenance')]
tag-rust-mcp message:
    #!/usr/bin/env bash
    set -euo pipefail
    . libs/rust-mcp/version.env
    tag="libs/rust-mcp/v${RUST_MCP_VERSION:?missing RUST_MCP_VERSION in libs/rust-mcp/version.env}"
    git tag -s -m "{{ message }}" "$tag"
    gum log --level info "Created tag: $tag"
    git push origin "$tag"
    gum log --level info "Pushed $tag"
    git tag -v "$tag"

# Full rust-mcp release flow. Changelog filters commits touching libs/rust-mcp/.
[group('maintenance')]
release-rust-mcp new_version:
    #!/usr/bin/env bash
    set -euo pipefail
    branch=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$branch" != "master" ]]; then
        gum log --level error "release-rust-mcp only allowed from master (on '$branch')"
        exit 1
    fi
    prev=$(git tag --sort=-v:refname -l "libs/rust-mcp/v*" | head -1)
    header="release libs/rust-mcp/v{{ new_version }}"
    if [[ -n "$prev" ]]; then
        summary=$(git log --format='- %s' "$prev"..HEAD -- libs/rust-mcp/)
        if [[ -n "$summary" ]]; then
            msg="$header"$'\n\n'"$summary"
        else
            msg="$header"
        fi
    else
        msg="$header"
    fi
    just bump-version-rust-mcp "{{ new_version }}"
    git add libs/rust-mcp/version.env libs/rust-mcp/Cargo.toml
    git commit -m "$header"
    just tag-rust-mcp "$msg"
    gh release create "libs/rust-mcp/v{{ new_version }}" --title "$header" --notes "$msg"

# ──── explore ───────────────────────────────────────────────────────
# Discovery / one-off experiments. Promoted to debug-* if they outlive
# their question.

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
    GOWORK=off "{{ justfile_directory() }}/build/dagnabit" export ./internal/... 2>&1
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
