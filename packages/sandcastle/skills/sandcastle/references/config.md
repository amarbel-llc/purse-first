# Sandcastle Configuration Reference

## Config File Format

Sandcastle reads a JSON configuration file with two top-level sections: `filesystem` and `network`. Pass via `--config <path>` or place at the default location `~/.srt-settings.json`.

## Filesystem Section

Controls which paths the sandboxed process can read and write.

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `denyRead` | `string[]` | `[]` | Paths blocked from reading |
| `denyWrite` | `string[]` | `[]` | Paths blocked from writing |
| `allowWrite` | `string[]` | `[]` | Paths explicitly allowed for writing |

Path values support environment variable expansion at the shell level when the config is generated via heredoc (e.g., `$HOME` expands to the user's home directory).

## Network Section

Controls which domains the sandboxed process can access. Sandcastle uses `--unshare-net` in bwrap and routes traffic through socat bridges to enforce restrictions.

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `allowedDomains` | `string[]` | `[]` | Domains that may be accessed (allowlist) |
| `deniedDomains` | `string[]` | `[]` | Domains that are blocked (denylist) |

When both lists are empty, no network restrictions are applied beyond the default bwrap network namespace isolation. Traffic is proxied through localhost socat bridges on ports 3128 (HTTP) and 1080 (SOCKS).

## Standard Security Policies

### Integration Test Isolation

Block sensitive user directories, allow writing to `/tmp`:

```json
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
```

This policy:
- Blocks access to SSH keys, AWS credentials, GPG keys, Kubernetes configs
- Blocks general config/local directories that may contain tokens or secrets
- Allows writing only to `/tmp` (where `$BATS_TEST_TMPDIR` resides)
- Leaves network unrestricted (empty lists = no domain filtering)

### Network-Restricted Tests

For tests that should not make external network calls:

```json
{
  "filesystem": {
    "denyRead": ["$HOME/.ssh", "$HOME/.aws", "$HOME/.gnupg"],
    "denyWrite": [],
    "allowWrite": ["/tmp"]
  },
  "network": {
    "allowedDomains": ["localhost", "127.0.0.1"],
    "deniedDomains": []
  }
}
```

### Tests Requiring Specific Services

Allow access to specific external APIs while blocking everything else:

```json
{
  "network": {
    "allowedDomains": ["api.example.com", "cache.nixos.org", "localhost"],
    "deniedDomains": []
  }
}
```

### Minimal Permissive Config

For cases where only the process isolation (separate PID/mount namespace) is needed without filesystem or network restrictions:

```json
{
  "filesystem": {
    "denyRead": [],
    "denyWrite": [],
    "allowWrite": []
  },
  "network": {
    "allowedDomains": [],
    "deniedDomains": []
  }
}
```

## Troubleshooting

### Permission Denied Errors

- Verify `allowWrite` includes the directories the process needs to write to
- For bats tests, ensure `/tmp` is in `allowWrite` (that's where `$BATS_TEST_TMPDIR` lives)
- Use `sandcastle --debug` to see which paths are mounted and what bwrap args are used
- Nix store paths (`/nix/store/...`) are always readable by default

### Binary Not Found Inside Sandbox

- The sandboxed process inherits `$PATH` from the parent environment
- Ensure the binary's directory is not under a `denyRead` path
- Nix store paths are always accessible
- Use absolute paths when in doubt

### Failed to Create Bridge Sockets

This error occurs when:
- Sandcastle is nested (running sandcastle inside sandcastle)
- The socat bridge processes cannot bind their listen ports
- There are leftover socket files from a previous run

Resolution:
- Never nest sandcastle invocations
- Check for stale files under `/tmp/nix-shell.*/claude-*`
- Use `--debug` to see which socket paths are being used

### Slow Startup

- Sandcastle has minimal per-invocation overhead (~200ms)
- Wrap the entire test runner in a single sandcastle invocation, not each individual test
- The socat bridges start once per sandcastle invocation and are shared across all commands within

### Tests Pass Locally but Fail in Sandbox

- The sandbox has a separate network namespace; `localhost` services may not be reachable
- Environment variables from the parent shell are inherited, but filesystem mounts differ
- Check if the test depends on files in a `denyRead` path
