
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

# Validate the repo's own plugin.json manifest.
[group('pre-build')]
validate-purse-first-manifest:
    {{ cmd_nix_dev }} go run ./cmd/purse-first validate .claude-plugin/plugin.json

# ──── lint ──────────────────────────────────────────────────────────
# Read-only style / convention / drift checks. Does not modify code.

[group('pre-build')]
lint: lint-go lint-dewey_pkgs_drift lint-treelint

# `go vet` across the workspace.
[group('pre-build')]
lint-go:
    {{ cmd_nix_dev }} go vet ./...

# Drift lint: verify libs/dewey/pkgs/ matches a fresh `dagnabit export --library`
# without mutating the tree. `--check` renders + formats facades into a temp dir
# and compares against the committed ones; it exits nonzero (naming the
# out-of-sync packages) on drift and fails loud if the formatter (treelint) is
# missing — no more silent skip / phantom drift. Depends on `dagnabit-build` so
# the binary under test is the one in the current working tree. Runs the binary
# ambient (not via `nix develop`) so dewey's `-tags test` build env is honored.
[group('pre-build')]
lint-dewey_pkgs_drift: dagnabit-build
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit export --check --library

# treelint check: read-only format + lint gate. Verifies formatter drift via
# sandbox-and-diff (Go/Nix/shell, per ./treelint.toml) plus shellcheck.
# treelint is the treefmt successor; this replaces the treefmt-nix check.
[group('pre-build')]
lint-treelint:
    {{ cmd_nix_dev }} treelint check

# Lint dewey library.
[group('pre-build')]
lint-dewey:
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
build: build-nix-gomod2nix build-nix

# Build the default Nix output (the purse-first CLI).
[group('build')]
build-nix:
    nix build

[group('build')]
build-purse-first:
    nix build .#purse-first

[group('build')]
build-purse-first-cli:
    nix build .#purse-first -o result-cli

[group('build')]
build-go:
    {{ cmd_nix_dev }} go build -o build/purse-first ./cmd/purse-first

# Build dewey library (all layers + CLI tools). Injects PURSE_FIRST_VERSION
# and the short commit into the buildinfo package via -ldflags, matching
# what the Nix derivations do for dev parity.
[group('build')]
build-dewey:
    #!/usr/bin/env bash
    set -euo pipefail
    . version.env
    commit=$(git rev-parse --short HEAD 2>/dev/null || echo dirty)
    bi=github.com/amarbel-llc/purse-first/libs/dewey/internal/0/buildinfo
    {{ cmd_nix_dev }} go build \
      -ldflags "-X $bi.Version=$PURSE_FIRST_VERSION -X $bi.Commit=$commit" \
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
    test-integration

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

# Run BATS integration tests.
[group('post-build')]
test-integration: build-purse-first-cli
    PURSE_FIRST_BIN={{ justfile_directory() }}/result-cli/bin/purse-first {{ cmd_nix_dev }} bats --tap --jobs {{ num_cpus() }} \
      zz-tests_bats/validate_documents.bats \
      zz-tests_bats/validate_mcp.bats

# Run validate-specific BATS tests.
[group('post-build')]
test-validate: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{ justfile_directory() }}/result-cli/bin/purse-first {{ cmd_nix_dev }} bats --tap --jobs {{ num_cpus() }} zz-tests_bats/validate_documents.bats

# Run MCP validation tests.
[group('post-build')]
test-validate-mcp: build-purse-first-cli
    PURSE_FIRST_BIN={{ justfile_directory() }}/result-cli/bin/purse-first {{ cmd_nix_dev }} bats --tap zz-tests_bats/validate_mcp.bats

# ──── codemod ───────────────────────────────────────────────────────
# Modifies source code.

# Format aggregate: repo-wide treelint pass plus the Go-only quick reformat.
[group('codemod')]
codemod-fmt: codemod-fmt-treelint codemod-fmt-go

# Repo-wide format via treelint (the treefmt successor): Go (goimports ->
# gofumpt), Nix (nixfmt), and shell (shfmt), per ./treelint.toml. Replaces the
# old `nix fmt` (treefmt-nix) path. The read-only counterpart is `lint-treelint`.
[group('codemod')]
codemod-fmt-treelint:
    {{ cmd_nix_dev }} treelint

# `go fmt ./...` for a quick Go-only reformat. The canonical repo-wide
# formatter is `codemod-fmt-treelint`.
[group('codemod')]
codemod-fmt-go:
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
update: update-nix update-go

[group('maintenance')]
update-nix:
    nix flake update

# Resync go.work and refresh gomod2nix.toml. (Regenerates the lockfile from
# the current go.mod / go.sum / go.work; does not bump pins.)
[group('maintenance')]
update-go: build-nix-gomod2nix

# Clean build artifacts.
[group('maintenance')]
clean:
    rm -f purse-first
    rm -rf build/
    rm -rf result result-cli

# Per eng-versioning(7) MULTI-ARTIFACT RELEASE: one version covers every
# artifact (purse-first CLI + marketplace, libs/dewey, libs/go-mcp), sourced
# from the repo-root version.env. A single release recipe creates the whole
# tag set atomically.

# Tag prefixes the release tags, in order. The bare "v" tag is the primary
# (purse-first CLI + marketplace + GitHub release); the path-prefixed entries
# are required by the Go module proxy to resolve the sub-directory modules.
release_tag_prefixes := "v libs/dewey/v libs/go-mcp/v"

# Rewrite PURSE_FIRST_VERSION in version.env. Pure mutation — release owns
# the commit and tag steps.
[group('maintenance')]
bump-version new_version:
    sed -E -i 's/^(export PURSE_FIRST_VERSION)=.*/\1={{ new_version }}/' version.env

# Read PURSE_FIRST_VERSION from version.env, then create the full signed,
# annotated tag set (v<sem>, libs/dewey/v<sem>, libs/go-mcp/v<sem>) at that
# single version, push each, and verify.
# tag accepts the message as $message (an exported env var), NOT {{ message }}.
# just's {{ }} is literal text-splicing into this script — a changelog subject
# containing backticks or $(...) (e.g. a commit titled `export --check`) would
# be evaluated as a command. The $-prefixed parameter is passed via the
# environment, so git sees it as inert data. Do NOT revert to {{ message }}.
[group('maintenance')]
tag $message:
    #!/usr/bin/env bash
    set -euo pipefail
    . version.env
    version="${PURSE_FIRST_VERSION:?missing PURSE_FIRST_VERSION in version.env}"
    # Create the whole tag set locally first, then push — a mid-set failure
    # (signing, pre-existing tag) then leaves only local tags to delete, with
    # nothing pushed to the remote to roll back.
    tags=()
    for prefix in {{ release_tag_prefixes }}; do
        tag="${prefix}${version}"
        git tag -s -m "$message" "$tag"
        gum log --level info "Created tag: $tag"
        tags+=("$tag")
    done
    for tag in "${tags[@]}"; do
        git push origin "$tag"
        gum log --level info "Pushed $tag"
        git tag -v "$tag"
    done

# Full release flow versioning all artifacts together: repo-wide changelog →
# bump → commit → tag set → gh release. The GitHub release points at the
# primary v<sem> tag; its notes enumerate the sibling sub-module tags.
[group('maintenance')]
release new_version:
    #!/usr/bin/env bash
    set -euo pipefail
    branch=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$branch" != "master" ]]; then
        gum log --level error "release only allowed from master (on '$branch')"
        exit 1
    fi
    # Changelog BEFORE the bump — the release-bump commit must not appear in
    # the changelog it announces.
    prev=$(git tag --sort=-v:refname -l "v*" | grep -E '^v[0-9]' | head -1 || true)
    header="release v{{ new_version }}"
    siblings=$'\n\nTags: libs/dewey/v{{ new_version }}, libs/go-mcp/v{{ new_version }}'
    if [[ -n "$prev" ]]; then
        summary=$(git log --format='- %s' "$prev"..HEAD)
        msg="$header"$'\n\n'"$summary""$siblings"
    else
        msg="$header""$siblings"
    fi
    just bump-version "{{ new_version }}"
    git add version.env
    git commit -m "$header"
    just tag "$msg"
    gh release create "v{{ new_version }}" --title "$header" --notes "$msg"

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
