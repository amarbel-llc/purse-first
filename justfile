
cmd_nix_dev := "nix develop " + justfile_directory() + " --command "
cmd_batman_bats := justfile_directory() + "/result-batman/bin/bats"

default: build test

# Build all packages (default = marketplace bundle)
build:
    nix build

# Build individual packages
build-grit:
    nix build .#grit

build-lux:
    nix build .#lux

build-get-hubbed:
    nix build .#get-hubbed

build-chix:
    nix build .#chix

build-purse-first:
    nix build .#purse-first

build-robin:
    nix build .#robin

build-spinclass:
    nix build .#spinclass

build-go:
    {{cmd_nix_dev}} go build -o build/purse-first ./cmd/purse-first

# Build Homebrew tap formula templates
build-brew:
    nix build .#homebrew-tap

# Build marketplace without hooks
build-no-hooks:
    nix build .#marketplace-no-hooks

build-batman:
    nix build .#batman -o result-batman

# Install MCP servers and packages
install:
    nix run .#install

update: update-nix

update-nix:
    nix flake update

cmd-tap-dancer := join(justfile_directory(), "./packages/tap-dancer/go/cmd/tap-dancer")
tap-dancer-go-test := "go run " + cmd-tap-dancer + " go-test -skip-empty"
tap-dancer-cargo-test := "go run " + cmd-tap-dancer + " cargo-test -skip-empty"

# Test individual Go packages
test-grit:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./packages/grit/...

test-get-hubbed:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./packages/get-hubbed/...

test-lux:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./packages/lux/...

test-spinclass:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./packages/spinclass/...

test-tap-dancer-go:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./packages/tap-dancer/go/...

# Test Rust packages
[working-directory: 'packages/chix']
test-chix:
  {{cmd_nix_dev}} {{tap-dancer-cargo-test}} test

[working-directory: 'packages/tap-dancer/rust']
test-tap-dancer-rust:
    {{cmd_nix_dev}} {{tap-dancer-cargo-test}} test

# Run tests
test-go:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} ./...

# Run tests with verbose output
test-v:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} -v ./...

# Format code
fmt:
    {{cmd_nix_dev}} go fmt ./...

# Lint code
lint:
    {{cmd_nix_dev}} go vet ./...

# Regenerate workspace vendor directory after dependency changes
vendor:
    {{cmd_nix_dev}} go work vendor

# Update go dependencies, tidy all modules, and re-vendor
deps:
    {{cmd_nix_dev}} go work sync
    {{cmd_nix_dev}} go work vendor

# Recompute goVendorHash in flake.nix from the local vendor directory
vendor-hash:
    #!/usr/bin/env bash
    set -euo pipefail
    # Hash the vendor directory — matches what nix's fixed-output derivation produces
    hash=$(nix hash path vendor/)
    # Update goVendorHash in flake.nix
    sed -i '' -E 's|(goVendorHash = )"sha256-[^"]+";|\1"'"$hash"'";|' flake.nix
    echo "updated goVendorHash to $hash"

# Run integration tests
test-integration: build-batman
    nix build
    {{cmd_nix_dev}} {{cmd_batman_bats}} --jobs {{num_cpus()}} \
      zz-tests_bats/validate_marketplace.bats \
      zz-tests_bats/validate_documents.bats \
      zz-tests_bats/validate_plugin_repos.bats

# Validate plugin repos have correct .claude-plugin/plugin.json
test-validate-repos: build-batman
    {{cmd_nix_dev}} {{cmd_batman_bats}} --jobs {{num_cpus()}} zz-tests_bats/validate_plugin_repos.bats

# Run validate-specific BATS tests
test-validate: build-batman
    nix build
    {{cmd_nix_dev}} {{cmd_batman_bats}} --jobs {{num_cpus()}} zz-tests_bats/validate_documents.bats

# Run lifecycle tests
test-lifecycle: build-batman
    nix build
    {{cmd_nix_dev}} {{cmd_batman_bats}} --jobs {{num_cpus()}} zz-tests_bats/hook_lifecycle.bats

test-lux-service: build-batman
    nix build
    {{cmd_nix_dev}} {{cmd_batman_bats}} --allow-unix-sockets --jobs {{num_cpus()}} zz-tests_bats/lux_service.bats

# Validate own plugin manifest
validate:
    {{cmd_nix_dev}} go run ./cmd/purse-first validate .claude-plugin/plugin.json

# Run Homebrew tap BATS tests
test-brew: build-batman
    nix build .#homebrew-tap
    {{cmd_nix_dev}} {{cmd_batman_bats}} --jobs {{num_cpus()}} zz-tests_bats/homebrew_tap.bats

test-spinclass-bats: build-batman
    nix build .#spinclass
    PATH="{{justfile_directory()}}/result-batman/bin:$PATH" {{cmd_nix_dev}} just packages/spinclass/zz-tests_bats/test

test-grit-bats: build-batman
    nix build .#grit
    GRIT_BIN={{justfile_directory()}}/result/bin/grit PATH="{{justfile_directory()}}/result-batman/bin:$PATH" {{cmd_nix_dev}} just packages/grit/zz-tests_bats/test

test-batman-bats: build-batman
    BATS_WRAPPER={{justfile_directory()}}/result-batman/bin/bats PATH="{{justfile_directory()}}/result-batman/bin:$PATH" {{cmd_nix_dev}} just packages/batman/zz-tests_bats/test

test-sandcastle-bats: build-batman
    nix build .#sandcastle
    PATH="{{justfile_directory()}}/result-batman/bin:{{justfile_directory()}}/result/bin:$PATH" {{cmd_nix_dev}} just packages/sandcastle/zz-tests_bats/test

# RFC-0001 conformance tests (not in test: aggregate — PWD-default and stdout-mode tests will fail until go-mcp implements those modes)
test-grit-conformance: build-batman
    nix build .#grit
    PACKAGE_BIN={{justfile_directory()}}/result/bin/grit \
      PATH="{{justfile_directory()}}/result-batman/bin:$PATH" \
      {{cmd_nix_dev}} just zz-tests_bats/rfc-0001/test

test-get-hubbed-conformance: build-batman
    nix build .#get-hubbed
    PACKAGE_BIN={{justfile_directory()}}/result/bin/get-hubbed \
      PATH="{{justfile_directory()}}/result-batman/bin:$PATH" \
      {{cmd_nix_dev}} just zz-tests_bats/rfc-0001/test

test-mgp-conformance: build-batman
    nix build .#mgp
    PACKAGE_BIN={{justfile_directory()}}/result/bin/mgp \
      PATH="{{justfile_directory()}}/result-batman/bin:$PATH" \
      {{cmd_nix_dev}} just zz-tests_bats/rfc-0001/test

test: \
    test-batman-bats \
    test-chix \
    test-get-hubbed \
    test-go \
    test-go-mcp \
    test-grit \
    test-grit-bats \
    test-integration \
    test-lifecycle \
    test-lux \
    test-lux-service \
    test-rust-mcp \
    test-sandcastle-bats \
    test-spinclass \
    test-spinclass-bats \
    test-tap-dancer-go \
    test-tap-dancer-rust \
    test-template

# Build go-lib-mcp
build-go-mcp:
    {{cmd_nix_dev}} go build ./...

# Test go-lib-mcp
test-go-mcp:
    {{cmd_nix_dev}} {{tap-dancer-go-test}} -v ./libs/go-mcp/...

# Build rust-lib-mcp
build-rust-mcp:
    nix build ./libs/rust-mcp

# Test rust-lib-mcp
test-rust-mcp:
    cd libs/rust-mcp && {{cmd_nix_dev}} {{tap-dancer-cargo-test}} test

# Test command package specifically
test-command:
    cd libs/go-mcp && nix develop ../../ --command {{tap-dancer-go-test}} -v ./command/

# Verify mkMarketplace.nix parses
check-lib:
    nix-instantiate --parse lib/mkMarketplace.nix

# Verify template parses
check-template:
    nix-instantiate --parse templates/marketplace/flake.nix

# Run template tests
test-template: build-batman
    {{cmd_nix_dev}} {{cmd_batman_bats}} --jobs {{num_cpus()}} zz-tests_bats/marketplace_template.bats

# Build dummy Go MCP servers
build-dummies-go:
    {{cmd_nix_dev}} go build -o build/ ./dummies/go/cmd/...

# Test dummy Go MCP servers
test-dummies-go:
    {{cmd_nix_dev}} go vet ./dummies/go/...

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

# Build lux from source, start isolated daemon, drop into shell for nvim testing
dev-lux:
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(cd "{{justfile_directory()}}" && pwd)"
    build_dir="$root/build"

    # Build lux from source
    mkdir -p "$build_dir"
    nix develop "$root" --command go build -o "$build_dir/lux" ./packages/lux/cmd/lux

    # Create isolated temp environment
    dir=$(mktemp -d /tmp/lux-dev-XXXXXX)
    trap 'kill "$daemon_pid" 2>/dev/null; wait "$daemon_pid" 2>/dev/null; rm -rf "$dir"' EXIT

    socket="$dir/lux.sock"

    # Write .envrc that sources the repo's envrc + adds overrides
    cat > "$dir/.envrc" <<ENVRC
    source_env "$root"
    PATH_add "$build_dir"
    export LUX_SOCKET="$socket"
    export XDG_CONFIG_HOME="$dir/config"
    ENVRC
    direnv allow "$dir"

    # Create config dir: symlink real lux config, use minimal nvim config
    mkdir -p "$dir/config"
    ln -s "${XDG_CONFIG_HOME:-$HOME/.config}/lux" "$dir/config/lux"
    ln -s "$root/dev/lux-nvim/nvim" "$dir/config/nvim"

    # Start dev daemon in background
    LUX_SOCKET="$socket" "$build_dir/lux" service run &
    daemon_pid=$!

    # Wait for socket
    deadline=$((SECONDS + 5))
    while [[ ! -S "$socket" ]] && [[ $SECONDS -lt $deadline ]]; do sleep 0.05; done
    [[ -S "$socket" ]] || { echo "daemon failed to start" >&2; exit 1; }

    echo "dev lux daemon running (pid $daemon_pid)"
    echo "socket: $socket"
    echo "run 'nvim <file>' to test"
    echo ""

    # Drop into user's shell
    cd "$dir"
    "$SHELL"

# Build lux from source, start daemon with user's full config
dev-lux-open:
    #!/usr/bin/env bash
    set -euo pipefail
    root="$(cd "{{justfile_directory()}}" && pwd)"
    build_dir="$root/build"

    # Build lux from source
    mkdir -p "$build_dir"
    nix develop "$root" --command go build -o "$build_dir/lux" ./packages/lux/cmd/lux

    # Create temp dir for socket and logs
    dir=$(mktemp -d /tmp/lux-dev-XXXXXX)
    trap 'kill "$daemon_pid" 2>/dev/null; wait "$daemon_pid" 2>/dev/null; rm -rf "$dir"' EXIT

    socket="$dir/lux.sock"
    state_dir="$dir/state"

    # Start dev daemon in background
    LUX_SOCKET="$socket" XDG_STATE_HOME="$state_dir" "$build_dir/lux" service run &
    daemon_pid=$!

    # Wait for socket
    deadline=$((SECONDS + 5))
    while [[ ! -S "$socket" ]] && [[ $SECONDS -lt $deadline ]]; do sleep 0.05; done
    [[ -S "$socket" ]] || { echo "daemon failed to start" >&2; exit 1; }

    echo "dev lux daemon running (pid $daemon_pid)"
    echo "socket: $socket"
    echo "log: $state_dir/lux/lux.log"
    echo ""

    # Drop into user's shell with dev overrides
    PATH="$build_dir:$PATH" LUX_SOCKET="$socket" XDG_STATE_HOME="$state_dir" "$SHELL"

# Clean build artifacts
clean:
    rm -f purse-first
    rm -rf build/
    rm -rf result result-batman
