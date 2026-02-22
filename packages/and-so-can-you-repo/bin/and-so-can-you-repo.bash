#!/usr/bin/env bash

set -euo pipefail

# and-so-can-you-repo — scaffold a new nix-flake-backed repo

REPOS_DIR="${AND_SO_CAN_YOU_REPO_DIR:-$(pwd)}"

# --- Helpers ---

nix_safe_name() {
  echo "$1" | tr '-' '_'
}

generate_file_if_missing() {
  local filepath="$1"
  local content="$2"

  if [[ -f "$filepath" ]]; then
    gum log --level warn "Skipping $filepath (already exists)"
    return 1
  fi

  mkdir -p "$(dirname "$filepath")"
  echo "$content" >"$filepath"
  gum log --level info "Created $filepath"
  return 0
}

apply_placeholders() {
  local content="$1"
  local name="$2"
  local description="$3"
  local nix_name
  nix_name="$(nix_safe_name "$name")"

  echo "$content" |
    sed "s/__NAME__/$name/g" |
    sed "s/__DESCRIPTION__/$description/g" |
    sed "s/__NIX_NAME__/$nix_name/g"
}

# --- Templates ---

template_envrc() {
  cat <<'TMPL'
source_up
use flake .
TMPL
}

template_gitignore_go() {
  cat <<'TMPL'
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
coverage.*
*.coverprofile
profile.cov
go.work
go.work.sum
.env
result
result-*
.direnv/
/__NAME__
TMPL
}

template_gitignore_rust() {
  cat <<'TMPL'
/target
.env
result
result-*
.direnv/
TMPL
}

template_gitignore_zig() {
  cat <<'TMPL'
.zig-cache/
zig-cache/
zig-out/
*.log
.env
result
result-*
.direnv/
TMPL
}

template_gitignore_shell() {
  cat <<'TMPL'
.env
result
result-*
.direnv/
TMPL
}

template_flake_go() {
  cat <<'TMPL'
{
  description = "__DESCRIPTION__";

  inputs = {
    nixpkgs-master.url = "github:NixOS/nixpkgs/b28c4999ed71543e71552ccfd0d7e68c581ba7e9";
    nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    go.url = "github:friedenberg/eng?dir=devenvs/go";
    shell.url = "github:friedenberg/eng?dir=devenvs/shell";
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      go,
      shell,
      nixpkgs-master,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [
            go.overlays.default
          ];
        };

        version = "0.1.0";

        __NIX_NAME__ = pkgs.buildGoApplication {
          pname = "__NAME__";
          inherit version;
          src = ./.;
          modules = ./gomod2nix.toml;
          subPackages = [ "cmd/__NAME__" ];

          meta = with pkgs.lib; {
            description = "__DESCRIPTION__";
            homepage = "https://github.com/friedenberg/__NAME__";
            license = licenses.mit;
          };
        };
      in
      {
        packages = {
          default = __NIX_NAME__;
          inherit __NIX_NAME__;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            just
          ];

          inputsFrom = [
            go.devShells.${system}.default
            shell.devShells.${system}.default
          ];

          shellHook = ''
            echo "__NAME__ - dev environment"
          '';
        };

        apps.default = {
          type = "app";
          program = "${__NIX_NAME__}/bin/__NAME__";
        };
      }
    );
}
TMPL
}

template_flake_rust() {
  cat <<'TMPL'
{
  description = "__DESCRIPTION__";

  inputs = {
    devenv-rust.url = "github:friedenberg/eng?dir=devenvs/rust";
    nixpkgs.follows = "devenv-rust/nixpkgs";
    utils.follows = "devenv-rust/utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      devenv-rust,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };
      in
      {
        packages.default = pkgs.rustPlatform.buildRustPackage {
          pname = "__NAME__";
          version = "0.1.0";

          src = ./.;

          cargoLock = {
            lockFile = ./Cargo.lock;
          };

          meta = with pkgs.lib; {
            description = "__DESCRIPTION__";
            homepage = "https://github.com/friedenberg/__NAME__";
            license = licenses.mit;
          };
        };

        devShells.default = devenv-rust.devShells.${system}.default;
      }
    );
}
TMPL
}

template_flake_zig() {
  cat <<'TMPL'
{
  description = "__DESCRIPTION__";

  inputs = {
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    nixpkgs-master.url = "github:NixOS/nixpkgs/b28c4999ed71543e71552ccfd0d7e68c581ba7e9";
    nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
    zig2nix.url = "github:Cloudef/zig2nix";
  };

  outputs =
    {
      zig2nix,
      nixpkgs,
      nixpkgs-master,
      utils,
      ...
    }:
    (utils.lib.eachDefaultSystem (
      system:
      let
        env = zig2nix.outputs.zig-env.${system} {
          zig = zig2nix.outputs.packages.${system}.zig-0_15_2;
        };
        pkgs = env.pkgs;
      in
      {
        packages.default = env.package {
          src = pkgs.lib.cleanSource ./.;
          zigBuildFlags = [ "-Doptimize=ReleaseSafe" ];
        };

        apps = {
          default = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/__NAME__";
          };
          build = env.app [ ] "zig build \"$@\"";
          test = env.app [ ] "zig build test -- \"$@\"";
        };

        devShells.default = env.mkShell {};
      }
    ));
}
TMPL
}

template_flake_shell() {
  cat <<'TMPL'
{
  description = "__DESCRIPTION__";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
    nixpkgs-master.url = "github:NixOS/nixpkgs/b28c4999ed71543e71552ccfd0d7e68c581ba7e9";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    shell.url = "github:friedenberg/eng?dir=devenvs/shell";
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-master,
      utils,
      shell,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };
        name = "__NAME__";
        script = (
          pkgs.writeScriptBin name (builtins.readFile ./bin/__NAME__.bash)
        ).overrideAttrs (old: {
          buildCommand = "${old.buildCommand}\n patchShebangs $out";
        });
        buildInputs = with pkgs; [
          gum
        ];
      in
      {
        packages.default = pkgs.symlinkJoin {
          inherit name;
          paths = [ script ] ++ buildInputs;
          buildInputs = [ pkgs.makeWrapper ];
          postBuild = "wrapProgram $out/bin/${name} --prefix PATH : $out/bin";
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            just
            gum
          ];

          inputsFrom = [
            shell.devShells.${system}.default
          ];

          shellHook = ''
            echo "${name} - dev environment"
          '';
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/${name}";
        };
      }
    );
}
TMPL
}

template_justfile_go() {
  cat <<'TMPL'
# __NAME__

default:
    @just --list

# Build the binary
build:
    nix build

build-gomod2nix:
    nix develop --command gomod2nix

build-go: build-gomod2nix
    nix develop --command go build -o __NAME__ ./cmd/__NAME__

# Run tests
test:
    nix develop --command go test ./...

# Run tests with verbose output
test-v:
    nix develop --command go test -v ./...

# Format code
fmt:
    nix develop --command go fmt ./...

# Lint code
lint:
    go vet ./...

# Update go dependencies and regenerate gomod2nix.toml
deps:
    nix develop --command go mod tidy
    nix develop --command gomod2nix

# Clean build artifacts
clean:
    rm -f __NAME__
    rm -rf result
TMPL
}

template_justfile_rust() {
  cat <<'TMPL'
# __NAME__

default:
    @just --list

# Build the binary
build:
    nix build

# Build with cargo
build-cargo:
    nix develop --command cargo build

# Run tests
test:
    nix develop --command cargo test

# Run tests with verbose output
test-v:
    nix develop --command cargo test -- --nocapture

# Check with clippy
check:
    nix develop --command cargo clippy

# Format code
fmt:
    nix develop --command cargo fmt

# Clean build artifacts
clean:
    rm -rf target
    rm -rf result
TMPL
}

template_justfile_zig() {
  cat <<'TMPL'
# __NAME__

default:
    @just --list

# Build the binary
build:
    nix build

# Check compilation
check:
    nix run .#build -- check

# Run tests
test:
    nix run .#test

# Clean build artifacts
clean:
    rm -rf zig-out .zig-cache
    rm -rf result
TMPL
}

template_justfile_shell() {
  cat <<'TMPL'
# __NAME__

default:
    @just --list

# Build the package
build:
    nix build

# Run the script
run *ARGS:
    nix run . -- {{ARGS}}

# Run tests
test:
    nix develop --command bats tests/

# Check with shellcheck
check:
    nix develop --command shellcheck bin/__NAME__.bash

# Format with shfmt
fmt:
    nix develop --command shfmt -w -i 2 -ci bin/__NAME__.bash

# Clean build artifacts
clean:
    rm -rf result
TMPL
}

template_go_main() {
  cat <<'TMPL'
package main

import "fmt"

func main() {
	fmt.Println("Hello from __NAME__")
}
TMPL
}

template_rust_cargo_toml() {
  cat <<'TMPL'
[package]
name = "__NAME__"
version = "0.1.0"
edition = "2021"
description = "__DESCRIPTION__"

[dependencies]
TMPL
}

template_rust_main() {
  cat <<'TMPL'
fn main() {
    println!("Hello from __NAME__");
}
TMPL
}

template_zig_build() {
  cat <<'TMPL'
const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const exe = b.addExecutable(.{
        .name = "__NAME__",
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
    });

    b.installArtifact(exe);

    const run_cmd = b.addRunArtifact(exe);
    run_cmd.step.dependOn(b.getInstallStep());
    if (b.args) |args| {
        run_cmd.addArgs(args);
    }

    const run_step = b.step("run", "Run the application");
    run_step.dependOn(&run_cmd.step);

    const unit_tests = b.addTest(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
    });

    const run_unit_tests = b.addRunArtifact(unit_tests);
    const test_step = b.step("test", "Run unit tests");
    test_step.dependOn(&run_unit_tests.step);
}
TMPL
}

template_zig_build_zon() {
  cat <<'TMPL'
.{
    .name = "__NAME__",
    .version = "0.1.0",
    .paths = .{
        "build.zig",
        "build.zig.zon",
        "src",
    },
}
TMPL
}

template_zig_main() {
  cat <<'TMPL'
const std = @import("std");

pub fn main() !void {
    const stdout = std.io.getStdOut().writer();
    try stdout.print("Hello from __NAME__\n", .{});
}
TMPL
}

template_shell_main() {
  cat <<'TMPL'
#!/usr/bin/env bash

set -euo pipefail

gum style --foreground 212 "Hello from __NAME__"
TMPL
}

# --- Main ---

main() {
  gum style \
    --border double \
    --padding "1 2" \
    --foreground 212 \
    "and-so-can-you-repo"

  # Step 1: Clone or init
  local repo_dir name
  local create_github=false
  local github_owner=""
  local github_visibility=""

  if gum confirm "Clone an existing repository?"; then
    local url
    url="$(gum input --placeholder "Repository URL")"

    if [[ -z "$url" ]]; then
      gum log --level error "URL cannot be empty"
      exit 1
    fi

    name="$(basename "$url" .git)"
    repo_dir="$REPOS_DIR/$name"

    if [[ -d "$repo_dir" ]]; then
      gum log --level error "$repo_dir already exists"
      exit 1
    fi

    gum spin --title "Cloning $url..." -- git clone "$url" "$repo_dir"
    gum log --level info "Cloned into $repo_dir"
  else
    name="$(gum input --placeholder "Project name (e.g. my-tool)")"

    if [[ -z "$name" ]]; then
      gum log --level error "Name cannot be empty"
      exit 1
    fi

    repo_dir="$REPOS_DIR/$name"

    if [[ -d "$repo_dir" ]]; then
      gum log --level error "$repo_dir already exists"
      exit 1
    fi

    if gum confirm "Create a GitHub repository?"; then
      create_github=true

      local orgs
      orgs="$(gh api user/orgs --jq '.[].login' 2>/dev/null || true)"

      local gh_user
      gh_user="$(gh api user --jq '.login' 2>/dev/null || true)"

      if [[ -z "$gh_user" ]]; then
        gum log --level error "Not authenticated with gh. Run 'gh auth login' first."
        exit 1
      fi

      local choices="$gh_user (personal)"
      if [[ -n "$orgs" ]]; then
        while IFS= read -r org; do
          choices="$choices"$'\n'"$org"
        done <<<"$orgs"
      fi

      github_owner="$(echo "$choices" | gum choose --header "GitHub owner:")"
      github_owner="${github_owner% (personal)}"

      github_visibility="$(gum choose --header "Visibility:" "private" "public")"
    fi

    mkdir -p "$repo_dir"
    git init "$repo_dir" --quiet
    gum log --level info "Initialized $repo_dir"
  fi

  # Step 2: Language
  local lang
  lang="$(gum choose --header "Language:" "go" "rust" "zig" "shell")"

  # Step 3: Description
  local description
  description="$(gum input --placeholder "Short project description")"

  if [[ -z "$description" ]]; then
    description="$name"
  fi

  # Escape slashes and ampersands for sed
  local sed_description
  sed_description="$(echo "$description" | sed 's/[&/\]/\\&/g')"

  cd "$repo_dir"

  # Step 4: Generate files
  gum log --level info "Generating files for $lang project..."

  local flake_content justfile_content gitignore_content

  case "$lang" in
    go)
      flake_content="$(apply_placeholders "$(template_flake_go)" "$name" "$sed_description")"
      justfile_content="$(apply_placeholders "$(template_justfile_go)" "$name" "$sed_description")"
      gitignore_content="$(apply_placeholders "$(template_gitignore_go)" "$name" "$sed_description")"
      ;;
    rust)
      flake_content="$(apply_placeholders "$(template_flake_rust)" "$name" "$sed_description")"
      justfile_content="$(apply_placeholders "$(template_justfile_rust)" "$name" "$sed_description")"
      gitignore_content="$(apply_placeholders "$(template_gitignore_rust)" "$name" "$sed_description")"
      ;;
    zig)
      flake_content="$(apply_placeholders "$(template_flake_zig)" "$name" "$sed_description")"
      justfile_content="$(apply_placeholders "$(template_justfile_zig)" "$name" "$sed_description")"
      gitignore_content="$(apply_placeholders "$(template_gitignore_zig)" "$name" "$sed_description")"
      ;;
    shell)
      flake_content="$(apply_placeholders "$(template_flake_shell)" "$name" "$sed_description")"
      justfile_content="$(apply_placeholders "$(template_justfile_shell)" "$name" "$sed_description")"
      gitignore_content="$(apply_placeholders "$(template_gitignore_shell)" "$name" "$sed_description")"
      ;;
  esac

  local envrc_content
  envrc_content="$(template_envrc)"

  generate_file_if_missing "flake.nix" "$flake_content"
  generate_file_if_missing "justfile" "$justfile_content"
  generate_file_if_missing ".envrc" "$envrc_content"
  generate_file_if_missing ".gitignore" "$gitignore_content"

  # Language-specific scaffolding
  case "$lang" in
    go)
      local go_main
      go_main="$(apply_placeholders "$(template_go_main)" "$name" "$sed_description")"
      generate_file_if_missing "cmd/$name/main.go" "$go_main"
      generate_file_if_missing "gomod2nix.toml" ""
      ;;
    rust)
      local cargo_toml rust_main
      cargo_toml="$(apply_placeholders "$(template_rust_cargo_toml)" "$name" "$sed_description")"
      rust_main="$(apply_placeholders "$(template_rust_main)" "$name" "$sed_description")"
      generate_file_if_missing "Cargo.toml" "$cargo_toml"
      generate_file_if_missing "src/main.rs" "$rust_main"
      ;;
    zig)
      local zig_build zig_zon zig_main
      zig_build="$(apply_placeholders "$(template_zig_build)" "$name" "$sed_description")"
      zig_zon="$(apply_placeholders "$(template_zig_build_zon)" "$name" "$sed_description")"
      zig_main="$(apply_placeholders "$(template_zig_main)" "$name" "$sed_description")"
      generate_file_if_missing "build.zig" "$zig_build"
      generate_file_if_missing "build.zig.zon" "$zig_zon"
      generate_file_if_missing "src/main.zig" "$zig_main"
      ;;
    shell)
      local shell_main
      shell_main="$(apply_placeholders "$(template_shell_main)" "$name" "$sed_description")"
      generate_file_if_missing "bin/$name.bash" "$shell_main"
      mkdir -p tests
      gum log --level info "Created tests/"
      ;;
  esac

  # Step 5: Post-generation
  gum log --level info "Running post-generation steps..."

  git add -A

  gum spin --title "Locking flake inputs..." -- \
    nix flake lock

  git add flake.lock

  direnv allow . 2>/dev/null || true

  case "$lang" in
    go)
      local go_module_owner="$github_owner"
      if [[ -z "$go_module_owner" ]]; then
        go_module_owner="friedenberg"
      fi
      gum log --level info "Initializing Go module..."
      nix develop --command bash -c "go mod init github.com/$go_module_owner/$name && gomod2nix" 2>&1 |
        while IFS= read -r line; do gum log --level debug "$line"; done
      git add -A
      ;;
    rust)
      gum log --level info "Generating Cargo.lock..."
      nix develop --command cargo generate-lockfile 2>&1 |
        while IFS= read -r line; do gum log --level debug "$line"; done
      git add -A
      ;;
    shell)
      chmod +x "bin/$name.bash"
      ;;
  esac

  # Step 6: Create GitHub repo
  if [[ "$create_github" == "true" ]]; then
    gum log --level info "Creating GitHub repository $github_owner/$name..."

    local gh_args=(
      "$github_owner/$name"
      "--$github_visibility"
      "--source" "$repo_dir"
      "--description" "$description"
    )

    gum spin --title "Creating GitHub repo..." -- \
      gh repo create "${gh_args[@]}"

    gum log --level info "Created GitHub repo: $github_owner/$name"
  fi

  # Step 7: Summary
  local summary_lines=(
    "Project '$name' bootstrapped successfully!"
    ""
    "Location: $repo_dir"
    "Language: $lang"
    "Description: $description"
  )

  if [[ "$create_github" == "true" ]]; then
    summary_lines+=("GitHub: $github_owner/$name ($github_visibility)")
  fi

  summary_lines+=(
    ""
    "Next steps:"
    "  cd $repo_dir"
    "  just build"
  )

  echo ""
  gum style \
    --border rounded \
    --padding "1 2" \
    --foreground 82 \
    "${summary_lines[@]}"
}

main "$@"
