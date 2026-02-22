---
name: sandcastle
description: This skill should be used when the user asks to "sandbox a command", "isolate a process", "run in a sandbox", "restrict filesystem access", "restrict network access", "add sandcastle", "use sandcastle", "wrap with sandcastle", "add sandbox to tests", "isolate tests with sandcastle", "set up sandcastle config", "add sandcastle to flake", or mentions sandcastle, bubblewrap isolation, sandbox-runtime, command sandboxing in a Nix project, or preventing tests from accessing secrets.
---

# Sandcastle

## Overview

Sandcastle is a Nix-wrapped CLI around Anthropic's `sandbox-runtime` that uses bubblewrap (bwrap) to sandbox command execution with filesystem and network restrictions. It runs on Linux without requiring root privileges.

## CLI Interface

```
sandcastle [options] [command...]

Options:
  -d, --debug          Enable debug logging (prints bwrap command, config)
  --config <path>      Path to JSON config file (default: ~/.srt-settings.json)
  --shell <shell>      Shell to execute the command with
  --control-fd <fd>    Read config updates from file descriptor (JSON lines)
```

### Invocation Patterns

Pass commands as positional arguments. Each argument is automatically shell-quoted to preserve boundaries through bwrap's nested `bash -c` layers:

```bash
# Simple command
sandcastle echo hello

# Command with flags (no special handling needed)
sandcastle ls -la /tmp

# Complex command via --shell
sandcastle --shell bash echo hello world

# With config file
sandcastle --config /path/to/policy.json my-command --flag value
```

**Important**: Sandcastle cannot be nested. Running `sandcastle sandcastle ...` will fail with "Failed to create bridge sockets". When testing sandcastle itself, invoke bats directly without a sandcastle wrapper.

### Debug Mode

Enable `--debug` to see the constructed bwrap command, resolved config, and network restriction details. Useful for diagnosing permission denials or unexpected behavior.

## Configuration Format

The config is a JSON file with `filesystem` and `network` sections. See `references/config.md` for the complete field reference and policy examples.

Minimal config:

```json
{
  "filesystem": {
    "denyRead": [],
    "allowWrite": ["/tmp"],
    "denyWrite": []
  },
  "network": {
    "allowedDomains": [],
    "deniedDomains": []
  }
}
```

When no config file is found at the specified path (or the default `~/.srt-settings.json`), sandcastle uses a permissive default that denies nothing.

### Dynamic Config Updates

The `--control-fd` flag accepts a file descriptor number. Sandcastle reads JSON lines from this fd and applies config updates at runtime via `SandboxManager.updateConfig()`. Each line must be a complete JSON config object.

## Nix Flake Integration

Add sandcastle as a flake input. It bundles its own dependencies (bubblewrap, socat, ripgrep, Node.js) so consumers only need the single package:

```nix
inputs = {
  sandcastle.url = "github:amarbel-llc/sandcastle";
};
```

Include in a devShell:

```nix
devShells.default = pkgs.mkShell {
  packages = [
    sandcastle.packages.${system}.default
  ];
};
```

Or depend on it in a package build:

```nix
nativeBuildInputs = [
  sandcastle.packages.${system}.default
];
```

Follow the stable-first nixpkgs convention: pin `sandcastle.inputs.nixpkgs.follows = "nixpkgs";` to share the nixpkgs instance.

## Test Isolation Patterns

Sandcastle is commonly used to wrap integration test execution (e.g., bats) to prevent tests from accessing sensitive user data or writing outside `/tmp`.

### Wrapper Script Pattern

Create a script that generates a temporary config and execs sandcastle:

```bash
#!/usr/bin/env bash
set -euo pipefail

srt_config="$(mktemp)"
trap 'rm -f "$srt_config"' EXIT

cat >"$srt_config" <<SETTINGS
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
  },
  "network": {
    "allowedDomains": [],
    "deniedDomains": []
  }
}
SETTINGS

exec sandcastle --shell bash --config "$srt_config" "$@"
```

This script is available as `examples/run-sandcastle-bats.bash`.

Key details:
- `$HOME` expands at runtime (not when the config is written)
- Temp file cleaned up via trap
- `exec` avoids an extra process layer
- `"$@"` passes all arguments through

### Justfile Integration

Route test execution through the wrapper:

```makefile
test:
  ./bin/run-sandcastle-bats.bash bats --tap --jobs {{num_cpus()}} *.bats
```

### Self-Testing Caveat

When sandcastle is the binary under test, do NOT wrap bats with sandcastle. Nesting causes "Failed to create bridge sockets" errors because the inner sandcastle cannot create its socat bridge processes inside the outer sandbox. Run bats directly in this case.

## Additional Resources

### Reference Files

- **`references/config.md`** -- Complete config field reference, standard security policies, network restriction patterns, and troubleshooting guide

### Example Files

- **`examples/run-sandcastle-bats.bash`** -- Ready-to-use wrapper script for sandcastle-isolated bats execution
- **`examples/flake-snippet.nix`** -- Nix flake input and devShell integration example
