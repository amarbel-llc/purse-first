# Bats-in-Sandcastle: Remove Wrapper Script, Wrap Bats Directly

## Problem

The robin skill currently requires consumers to create a `bin/run-sandcastle-bats.bash` wrapper script that invokes sandcastle around bats. This adds per-project boilerplate (the wrapper script, the bin/ directory, the sandcastle flake input) and exposes sandcastle as a user-facing concern. Tests should get isolation transparently.

## Design

Changes scoped to the batman repo only. Sandcastle `allowRead` support deferred to a future effort (requires patching the upstream Anthropic sandbox-runtime).

### 1. Batman: Bats Wrapper with Built-in Sandcastle Isolation

Batman's `flake.nix` adds a new `bats` package — a shell script that generates a sandcastle config at runtime and execs sandcastle around the real bats binary.

**Read isolation (deny-list, current sandcastle model):**

- `denyRead`: `$HOME/.ssh`, `$HOME/.aws`, `$HOME/.gnupg`, `$HOME/.config`, `$HOME/.local`, `$HOME/.password-store`, `$HOME/.kube`

**Write isolation:**

- `allowWrite`: `/tmp` only

**Network:** unrestricted by default.

**Nix derivation sketch:**

```nix
bats = pkgs.writeShellApplication {
  name = "bats";
  runtimeInputs = [ pkgs.bats sandcastle ];
  text = ''
    config="$(mktemp)"
    trap 'rm -f "$config"' EXIT
    cat >"$config" <<EOF
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
        "allowWrite": ["/tmp"]
      }
    }
    EOF
    exec sandcastle --shell bash --config "$config" bats "$@"
  '';
};
```

Consumers add `batman.packages.${system}.bats` to their devShell. The wrapper shadows `pkgs.bats` by name. Inside the sandcastle, `$PATH` still resolves to the real bats.

### 2. Robin Skill Updates

**Removed:**

- `bin/run-sandcastle-bats.bash` wrapper script section and example
- "Sandcastle Environment Isolation" section explaining the wrapper pattern
- `sandcastle` as a direct consumer flake input
- Setup checklist step for creating the wrapper script
- `bin/` directory from directory convention

**Updated:**

- Nix flake integration: `batman.packages.${system}.bats` replaces `pkgs.bats` + `sandcastle`
- Justfile recipes call `bats` directly
- Directory convention loses `bin/` subdirectory
- Sandcastle reference doc updated to explain the bats wrapper approach

**New directory convention:**

```
project/
├── zz-tests_bats/
│   ├── justfile
│   ├── common.bash
│   ├── some_feature.bats
│   └── another_feature.bats
├── justfile
└── flake.nix
```

**New justfile template:**

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

**New setup checklist:**

1. Add `batman` flake input to `flake.nix`
2. Add `batman.packages.${system}.bats` and `batman.packages.${system}.bats-libs` to devShell
3. Create `zz-tests_bats/` directory
4. Create `common.bash` with `bats_load_library` calls
5. Add test justfile with `test-targets`, `test-tags`, and `test` recipes
6. Wire root justfile to delegate to `zz-tests_bats/test`
7. Create first `.bats` test file

## Future Work

- [ ] Add `allowRead` support to sandcastle (requires patching Anthropic sandbox-runtime's `FilesystemConfigSchema` in `sandbox-config.js` and `generateFilesystemArgs()` in `linux-sandbox-utils.js` to use selective `--ro-bind` instead of `--ro-bind / /`)
- [ ] Switch bats wrapper from deny-list to allow-list reads once `allowRead` is available (target: `allowRead: ["/nix/store", "/tmp"]`)

## Implementation Order

1. **Batman flake.nix:** Add sandcastle flake input, create bats wrapper derivation
2. **Robin skill:** Update SKILL.md, examples, references, remove wrapper script artifacts
3. **Verify:** Build and test the bats wrapper, run sandcastle's own tests
