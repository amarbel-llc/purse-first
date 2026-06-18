
cmd_nix_dev := "nix develop " + justfile_directory() + " --command "

default: validate lint build test

# ──── validate ──────────────────────────────────────────────────────
# Pre-build correctness checks. Hard failures (parser, schema, drift).

[group('pre-build')]
validate: validate-nix validate-purse-first-manifest

# `nix flake check` evaluates every flake check, including
# `checks.formatting` (the conformist read-only gate: formatter drift across the
# whole tree + shellcheck + the eng-convention linters) and the per-package
# builds the flake exposes.
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
lint: lint-go lint-dewey_pkgs_drift lint-conformist lint-dewey-self

# `go vet` across the workspace.
[group('pre-build')]
lint-go:
    {{ cmd_nix_dev }} go vet ./...

# Drift lint: verify libs/dewey/pkgs/ matches a fresh `dagnabit export --library`
# without mutating the tree. `--check` renders + formats facades into a temp dir
# and compares against the committed ones; it exits nonzero (naming the
# out-of-sync packages) on drift and fails loud if the formatter (conformist) is
# missing — no more silent skip / phantom drift. Depends on `build-dagnabit` so
# the binary under test is the one in the current working tree. Runs the binary
# ambient (not via `nix develop`) so dewey's `-tags test` build env is honored.
[group('pre-build')]
lint-dewey_pkgs_drift: build-dagnabit
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit export --check --library

# conformist check: read-only format + lint gate. Builds the flake's
# `checks.<sys>.formatting` (conformistEval.config.build.check self) — the
# sandboxed `conformist check` over the whole tree (Go/Nix/shell formatter drift
# + shellcheck + the eng-convention linters from presets.eng), driven by
# ./conformist.nix. The read-write counterpart is `codemod-fmt-conformist`.
[group('pre-build')]
lint-conformist:
    nix build {{ justfile_directory() }}#checks.$(nix eval --impure --raw --expr builtins.currentSystem).formatting --no-link

[group('pre-build')]
lint-dewey-extra: lint-dewey lint-dewey-analyzers

# go vet -tags test across the dewey library.
[group('pre-build')]
lint-dewey:
    {{ cmd_nix_dev }} go vet -tags test ./libs/dewey/...

# Build one dewey analyzer (defererr|repool|seqerror) and run it via -vettool.
[group('pre-build')]
lint-dewey-analyzer name:
    {{ cmd_nix_dev }} go build -o build/{{ name }} ./libs/dewey/cmd/{{ name }}
    {{ cmd_nix_dev }} go vet -vettool={{ justfile_directory() }}/build/{{ name }} -tags test ./libs/dewey/...

[group('pre-build')]
lint-dewey-analyzers: (lint-dewey-analyzer "defererr") (lint-dewey-analyzer "repool") (lint-dewey-analyzer "seqerror")

# Dogfood the dewey analyzers over dewey's OWN source via the published
# golangci-lint-dewey custom binary (not -vettool), exercising the exact
# gclplugin default set (defererr/seqerror/repool/testui). golangci-lint v2
# is go.work-unaware, so run from the module dir with GOWORK=off; the root
# .golangci.yml (found by walk-up) carries build-tags=test and the level-0/
# alfa testui exclusion. Scoped to libs/dewey for now (go-mcp + root are a
# tracked follow-up).
[group('pre-build')]
lint-dewey-self: build-go-gcl
    cd {{ justfile_directory() }}/libs/dewey && {{ cmd_nix_dev }} env GOWORK=off {{ justfile_directory() }}/build/golangci-lint-dewey run ./...

# ──── build ─────────────────────────────────────────────────────────
# Compile / generate artifacts.

[group('build')]
build: build-nix-gomod2nix build-nix

[group('build')]
build-dev: build-go build-dewey build-dagnabit build-go-gcl

[group('build')]
build-artifacts: build-purse-first build-purse-first-cli build-golangci-dewey build-nix-gomod2nix-gcl

# Build the default Nix output (the purse-first CLI).
[group('build')]
build-nix:
    nix build

# Nix-build the purse-first CLI package (result symlink).
[group('build')]
build-purse-first:
    nix build .#purse-first

# Nix-build the purse-first CLI to result-cli (for the bats integration lane).
[group('build')]
build-purse-first-cli:
    nix build .#purse-first -o result-cli

# Nix-build the dewey custom golangci-lint binary (gclplugin linked in,
# purse-first#134) for the bats acceptance lane and downstream consumers.
[group('build')]
build-golangci-dewey:
    nix build .#golangci-lint-dewey -o result-gcl

# Go-build the purse-first CLI into build/ for the dev loop (no nix).
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

# Compile the standalone golangci-lint-dewey module into build/ for the
# dev loop. It is not in go.work, so `go build ./...` never touches it;
# this is the cheap compile gate (and local smoke-test binary) before the
# nix build. GOWORK=off for the same reason as build-nix-gomod2nix-gcl.
[group('build')]
build-go-gcl:
    cd {{ justfile_directory() }}/cmd/golangci-lint-dewey && \
      {{ cmd_nix_dev }} env GOWORK=off go build -o {{ justfile_directory() }}/build/golangci-lint-dewey .

# Regenerate the standalone golangci-lint-dewey lockfiles (go.sum +
# gomod2nix.toml) in cmd/golangci-lint-dewey. That module is
# deliberately NOT in go.work — golangci-lint's closure is a lint tool's
# dependency set, isolated from the shared workspace lockfile so product
# binaries don't rebuild on linter bumps (purse-first#134). GOWORK=off
# keeps go in module mode despite the workspace above it; without it go
# refuses to run in a module that the surrounding go.work doesn't list.
# Run after changing the golangci-lint pin or the module's go.mod.
[group('build')]
build-nix-gomod2nix-gcl:
    cd {{ justfile_directory() }}/cmd/golangci-lint-dewey && \
      {{ cmd_nix_dev }} env GOWORK=off go mod tidy
    cd {{ justfile_directory() }}/cmd/golangci-lint-dewey && \
      {{ cmd_nix_dev }} env GOWORK=off gomod2nix \
        --target linux/amd64 \
        --target linux/arm64 \
        --target darwin/amd64 \
        --target darwin/arm64

# Rebuild build/dagnabit from source. Not a dep of the codemod-dewey-* recipes
# because those need to work mid-bootstrap when cmd/dagnabit's imports
# may temporarily reference paths that don't compile yet. Run this
# manually when you've changed dagnabit's source.
[group('build')]
build-dagnabit:
    {{ cmd_nix_dev }} go build -o {{ justfile_directory() }}/build/dagnabit ./cmd/dagnabit

# ──── test ──────────────────────────────────────────────────────────
# Post-build: run test suites against built artifacts and source.

[group('post-build')]
test: \
    test-go \
    test-go-mcp \
    test-dewey \
    test-integration \
    test-golangci-dewey \
    test-dagnabit-rust

[group('post-build')]
test-extra: test-v test-race test-validate test-validate-mcp

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

# Run the golangci-lint-dewey BATS lane against the nix-built custom
# binary — the purse-first#134 acceptance test (plugin loads, analyzers fire).
[group('post-build')]
test-golangci-dewey: build-golangci-dewey
    GOLANGCI_LINT_DEWEY_BIN={{ justfile_directory() }}/result-gcl/bin/golangci-lint-dewey {{ cmd_nix_dev }} bats --tap zz-tests_bats/golangci_lint_dewey.bats

# Dev-loop variant of test-golangci-dewey: run the bats lane against the
# go-built binary in build/ (no nix build). The version-suffix assertion
# self-skips here — the "dewey" suffix is injected by nix ldflags only.
[group('explore')]
explore-test-golangci-dewey-dev: build-go-gcl
    GOLANGCI_LINT_DEWEY_BIN={{ justfile_directory() }}/build/golangci-lint-dewey {{ cmd_nix_dev }} bats --tap zz-tests_bats/golangci_lint_dewey.bats

# BATS lane for dagnabit rust mode (reposition / export / rename against
# cargo fixture workspaces). Tests skip gracefully when cargo/ast-grep
# are not on PATH, so the CI gate stays green without the rust devshell.
[group('post-build')]
test-dagnabit-rust: build-dagnabit
    {{ cmd_nix_dev }} bats --tap zz-tests_bats/dagnabit_rust.bats

# Run MCP validation tests.
[group('post-build')]
test-validate-mcp: build-purse-first-cli
    PURSE_FIRST_BIN={{ justfile_directory() }}/result-cli/bin/purse-first {{ cmd_nix_dev }} bats --tap zz-tests_bats/validate_mcp.bats

# ──── codemod ───────────────────────────────────────────────────────
# Modifies source code.

[group('codemod')]
codemod-fmt: codemod-fmt-conformist codemod-fmt-go

# Repo-wide format via conformist: Go (goimports -> gofumpt), Nix (nixfmt), and
# shell (shfmt -i 2 -s -ci), per ./conformist.nix. Runs the flake `formatter`
# output (conformistEval.config.build.wrapper, repair mode). The read-only
# counterpart is `lint-conformist`.
[group('codemod')]
codemod-fmt-conformist:
    nix fmt

# `go fmt ./...` for a quick Go-only reformat. The canonical repo-wide
# formatter is `codemod-fmt-conformist`.
[group('codemod')]
codemod-fmt-go:
    {{ cmd_nix_dev }} go fmt ./...

# Dry-run a single-package rename: print the proposed move as NDJSON,
# touch nothing on disk. `new_leaf` is optional; defaults to src's leaf.
[group('debug')]
debug-dewey-rename pkg new_leaf="":
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit rename -n {{ pkg }} {{ new_leaf }}

# Apply a single-package rename. Emits an NDJSON `move` event on success.
[group('debug')]
debug-dewey-rename-apply pkg new_leaf="":
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit rename {{ pkg }} {{ new_leaf }}

# Dry-run a full reposition of libs/dewey/internal/. Prints NDJSON
# `would-move` events for each package that needs repositioning.
[group('debug')]
debug-dewey-reposition:
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit -n internal

# Apply a full reposition of libs/dewey/internal/. Real moves print
# NDJSON `move` events as they happen.
[group('debug')]
debug-dewey-reposition-apply:
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit internal

# Generate a public facade in libs/dewey/pkgs/ for a single internal
# package. `pkg` is the path inside libs/dewey, e.g. `internal/0/go_module`.
[group('debug')]
debug-dewey-export pkg:
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit export ./{{ pkg }}

# Generate pkgs/ facades for every package under libs/dewey/internal/ (library mode).
# Fails if any //go:generate dagnabit export directives are found.
[group('debug')]
debug-dewey-export-library *flags:
    cd {{ justfile_directory() }}/libs/dewey && {{ justfile_directory() }}/build/dagnabit export --library {{ flags }}

# ──── maintenance ───────────────────────────────────────────────────
# Refresh dependencies, bump versions, tag/release, clean.

[group('maintenance')]
update: update-nix update-go

# Refresh the flake.lock inputs.
[group('maintenance')]
update-nix:
    nix flake update

# Resync go.work and refresh gomod2nix.toml. (Regenerates the lockfile from
# the current go.mod / go.sum / go.work; does not bump pins.) Delegates to
# build-nix-gomod2nix via a body call rather than a dependency so that recipe
# stays owned by exactly one aggregate (build) per the task-hierarchy rule.
[group('maintenance')]
update-go:
    just build-nix-gomod2nix

# Clean build artifacts.
[group('maintenance')]
clean-build:
    rm -f purse-first
    rm -rf build/
    rm -rf result result-cli result-gcl

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

# One-off probe: copy dewey to a temp dir, reposition NATO levels into
# internal/, run dagnabit export, and isolate a single facade build on failure.
# Scratch reproduction for the dagnabit export/reposition bootstrap; delete once
# its question is answered.
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
