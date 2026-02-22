# Bats-in-Sandcastle Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace per-project sandcastle wrapper scripts with a batman-provided `bats` wrapper that transparently enforces sandcastle isolation.

**Architecture:** Batman's flake.nix gains a sandcastle input and a `bats` wrapper derivation (shell script). The wrapper generates a sandcastle config at runtime and execs sandcastle around the real bats. The robin skill docs are updated to remove all wrapper script references and simplify the consumer experience.

**Tech Stack:** Nix flakes, bash, sandcastle, bats-core

---

### Task 1: Add sandcastle flake input to batman

**Files:**
- Modify: `flake.nix:4-14` (inputs section)

**Step 1: Add sandcastle input**

Add the sandcastle input after the existing `purse-first` input in `flake.nix`:

```nix
    sandcastle = {
      url = "github:amarbel-llc/sandcastle";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };
```

Also add `sandcastle` to the outputs function parameters at line 26:

```nix
    {
      self,
      nixpkgs,
      nixpkgs-master,
      utils,
      shell,
      purse-first,
      sandcastle,
    }:
```

**Step 2: Lock the new input**

Run: `nix flake lock --update-input sandcastle` (from the and-robin worktree)

Expected: `flake.lock` updates with sandcastle's locked rev and hash.

**Step 3: Verify the flake still evaluates**

Run: `nix flake show` (from the and-robin worktree)

Expected: All existing outputs still appear (packages.x86_64-linux.default, bats-libs, etc.)

**Step 4: Commit**

```
feat: add sandcastle flake input to batman
```

---

### Task 2: Create bats wrapper derivation

**Files:**
- Modify: `flake.nix:68-120` (after bats-libs, before robin)

**Step 1: Write bats wrapper test**

Create `zz-tests_bats/bats_wrapper.bats` to test the wrapper:

```bash
#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
}

function bats_wrapper_runs_tests { # @test
  # The wrapper should be able to run a simple inline test
  run bats --tap <(cat <<'EOF'
#! /usr/bin/env bats
function truth { # @test
  true
}
EOF
)
  assert_success
  assert_output --partial "ok 1"
}

function bats_wrapper_blocks_ssh_read { # @test
  # Create a canary file in a location that should be denied
  # Note: this test only works if $HOME/.ssh exists
  if [[ ! -d "$HOME/.ssh" ]]; then
    skip "no ~/.ssh directory"
  fi
  run bats --tap <(cat <<'EOF'
#! /usr/bin/env bats
function read_ssh { # @test
  ls "$HOME/.ssh"
}
EOF
)
  assert_failure
}

function bats_wrapper_allows_tmp_write { # @test
  run bats --tap <(cat <<'EOF'
#! /usr/bin/env bats
function write_tmp { # @test
  echo "test" > /tmp/bats-wrapper-test-$$
  rm -f /tmp/bats-wrapper-test-$$
}
EOF
)
  assert_success
}
```

Create `zz-tests_bats/common.bash`:

```bash
bats_load_library bats-support
bats_load_library bats-assert
```

**Step 2: Add the bats wrapper derivation to flake.nix**

Insert after the `bats-libs` definition (after line 79) and before the `robin` definition:

```nix
        sandcastle-pkg = sandcastle.packages.${system}.default;

        bats = pkgs.writeShellApplication {
          name = "bats";
          runtimeInputs = [
            pkgs.bats
            sandcastle-pkg
          ];
          text = ''
            config="$(mktemp)"
            trap 'rm -f "$config"' EXIT

            cat >"$config" <<SANDCASTLE_CONFIG
            {
              "filesystem": {
                "denyRead": [
                  "$HOME/.ssh",
                  "$HOME/.aws",
                  "$HOME/.gnupg",
                  "$HOME/.config",
                  "$HOME/.local",
                  "$HOME/.password-store",
                  "$HOME/.kube"
                ],
                "denyWrite": [],
                "allowWrite": [
                  "/tmp"
                ]
              },
              "network": {
                "allowedDomains": [],
                "deniedDomains": []
              }
            }
            SANDCASTLE_CONFIG

            exec sandcastle --shell bash --config "$config" bats "$@"
          '';
        };
```

**Step 3: Export bats in packages output**

Add `bats` to the packages set (around line 114):

```nix
        packages = {
          default = pkgs.symlinkJoin {
            name = "batman";
            paths = [
              bats-libs
              bats
              robin
            ];
          };
          inherit
            bats-support
            bats-assert
            bats-assert-additions
            bats-libs
            bats
            robin
            ;
        };
```

**Step 4: Add bats and sandcastle to devShell**

Update the devShell packages to include bats wrapper and sandcastle (for running the wrapper's own tests):

```nix
        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.just
            pkgs.gum
            bats-libs
            bats
          ];

          inputsFrom = [
            shell.devShells.${system}.default
          ];

          shellHook = ''
            echo "batman - dev environment"
          '';
        };
```

**Step 5: Build the flake**

Run: `nix build` (from the and-robin worktree)

Expected: Build succeeds. The result includes `bin/bats` (the wrapper).

**Step 6: Verify the wrapper script content**

Run: `cat result/bin/bats`

Expected: The wrapper script with sandcastle invocation.

**Step 7: Run the wrapper tests**

Run: `just zz-tests_bats/test` or `bats --tap zz-tests_bats/bats_wrapper.bats`

Expected: All 3 tests pass (wrapper runs, blocks ssh reads, allows tmp writes).

**Step 8: Commit**

```
feat: add bats wrapper with built-in sandcastle isolation
```

---

### Task 3: Update SKILL.md — remove wrapper script references

**Files:**
- Modify: `skills/bats-testing/SKILL.md`

**Step 1: Update directory convention (lines 15-26)**

Replace the directory tree to remove `bin/` directory:

```
project/
├── zz-tests_bats/
│   ├── justfile
│   ├── common.bash
│   ├── some_feature.bats
│   └── another_feature.bats
├── justfile                  (root justfile delegates to zz-tests_bats/)
└── flake.nix
```

**Step 2: Update Nix Flake Integration section (lines 104-128)**

Replace the entire section with:

```markdown
## Nix Flake Integration

Add `batman` to flake inputs, then include `bats` and `bats-libs` in the devShell packages. The `bats` package wraps bats-core with sandcastle isolation — tests are automatically sandboxed without any additional setup. The `bats-libs` package includes a setup hook that automatically exports `BATS_LIB_PATH`, so `bats_load_library` works without any manual environment configuration.

\`\`\`nix
inputs = {
  batman.url = "github:amarbel-llc/batman";
};
\`\`\`

\`\`\`nix
devShells.default = pkgs.mkShell {
  packages = (with pkgs; [
    just
    gum
  ]) ++ [
    batman.packages.${system}.bats
    batman.packages.${system}.bats-libs
  ];
};
\`\`\`

With this setup:
- `bats` automatically runs tests inside a sandcastle sandbox (blocks reads to `~/.ssh`, `~/.aws`, `~/.gnupg`, etc.; writes restricted to `/tmp`)
- `bats_load_library bats-support`, `bats_load_library bats-assert`, and `bats_load_library bats-assert-additions` all resolve automatically via `BATS_LIB_PATH`
```

**Step 3: Remove Sandcastle Environment Isolation section (lines 130-171)**

Delete the entire "## Sandcastle Environment Isolation" section.

**Step 4: Update Justfile Integration section (lines 173-211)**

Replace the test-suite justfile example to remove sandcastle wrapper references:

```makefile
export CMD_BIN := "my-command"

bats_timeout := "5"

test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --tap --jobs {{num_cpus()}} {{targets}}

test-tags *tags:
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --tap --jobs {{num_cpus()}} --filter-tags {{tags}} *.bats

test: (test-targets "*.bats")
```

Update the "Key patterns" list to replace "All execution routed through sandcastle wrapper" with "Sandcastle isolation provided transparently by batman's bats wrapper".

**Step 5: Update Setup Checklist (lines 213-224)**

Replace with:

```markdown
## Setup Checklist

When setting up BATS in a new repo:

1. Add `batman` flake input to `flake.nix`
2. Add `batman.packages.${system}.bats` and `batman.packages.${system}.bats-libs` to devShell packages
3. Create `zz-tests_bats/` directory
4. Create `common.bash` using `bats_load_library` to load assertion libraries
5. Add test justfile with `test-targets`, `test-tags`, and `test` recipes
6. Wire root justfile to delegate to `zz-tests_bats/test`
7. Create first `.bats` test file following the function-name pattern
```

**Step 6: Update Example Files list (lines 242-248)**

Remove the `examples/run-sandcastle-bats.bash` entry.

**Step 7: Update skill description frontmatter (line 3)**

Remove "sandcastle test isolation" from the trigger list, replace with "sandcastle" (it's still relevant, just transparent now).

**Step 8: Commit**

```
docs: update SKILL.md to remove wrapper script, simplify consumer experience
```

---

### Task 4: Update example files

**Files:**
- Delete: `skills/bats-testing/examples/run-sandcastle-bats.bash`
- Modify: `skills/bats-testing/examples/justfile`

**Step 1: Delete the wrapper script example**

Remove `skills/bats-testing/examples/run-sandcastle-bats.bash`.

**Step 2: Update example justfile**

Replace `skills/bats-testing/examples/justfile` with:

```makefile
# zz-tests_bats/justfile
# Test-suite justfile for BATS integration tests

export CMD_BIN := "my-command"

bats_timeout := "5"

# Run specific test files (default: all .bats files)
test-targets *targets="*.bats":
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --tap --jobs {{num_cpus()}} {{targets}}

# Run tests matching specific tags
test-tags *tags:
  BATS_TEST_TIMEOUT="{{bats_timeout}}" \
    bats --tap --jobs {{num_cpus()}} --filter-tags {{tags}} *.bats

# Run all tests
test: (test-targets "*.bats")
```

**Step 3: Commit**

```
refactor: remove sandcastle wrapper example, simplify justfile template
```

---

### Task 5: Update sandcastle reference doc

**Files:**
- Modify: `skills/bats-testing/references/sandcastle.md`

**Step 1: Update the reference**

Replace the "Runner Script Pattern" section (lines 101-140) with a new section explaining that batman's bats wrapper handles sandcastle invocation automatically:

```markdown
## Batman Bats Wrapper

Batman provides a `bats` wrapper that automatically invokes sandcastle. When you use `batman.packages.${system}.bats` in your devShell, every `bats` invocation is sandboxed with:

- **Read denied:** `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.config`, `~/.local`, `~/.password-store`, `~/.kube`
- **Write allowed:** `/tmp` only
- **Network:** unrestricted

No wrapper script or manual sandcastle configuration is needed. Just run `bats` normally.

For custom sandcastle policies beyond the defaults (e.g., network restrictions, additional deny paths), you can invoke sandcastle directly — see the CLI interface section above.
```

Remove the "Standard Security Policy for BATS Tests" section (lines 67-99) since the policy is now baked into the wrapper. Keep the config format docs for users who want custom policies.

**Step 2: Commit**

```
docs: update sandcastle reference for bats wrapper approach
```

---

### Task 6: Build and verify

**Step 1: Build the full flake**

Run: `nix build` (from the and-robin worktree)

Expected: Build succeeds.

**Step 2: Verify bats wrapper exists and works**

Run: `./result/bin/bats --version`

Expected: Prints bats version.

**Step 3: Run batman's own bats tests**

Run: `bats --tap zz-tests_bats/bats_wrapper.bats`

Expected: All tests pass.

**Step 4: Verify skill content is correct**

Run: `ls result/share/purse-first/robin/skills/bats-testing/examples/`

Expected: `run-sandcastle-bats.bash` is NOT present. `common.bash`, `example.bats`, `justfile` are present.

**Step 5: Run nix flake check**

Run: `nix flake check`

Expected: No errors.

**Step 6: Final commit if any fixups needed**

```
chore: fixups from verification
```
