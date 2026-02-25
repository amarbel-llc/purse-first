---
status: accepted
date: 2026-02-25
---

# Adopt unified plugin generation from single source of truth

## Context and Problem Statement

Plugin manifests (`.claude-plugin/plugin.json`) were maintained manually, duplicating information already present in `command.App` registrations for Go packages or scattered across config files for non-Go packages. This caused drift between the binary's actual capabilities and its declared manifest -- for example, lux's manual `plugin.json` had `["mcp", "stdio"]` while the actual subcommand was `mcp-stdio`.

## Considered Options

* Keep manual `plugin.json` files
* Generate from `command.App` (Go) and `package.toml` (non-Go) as single sources of truth
* Use a schema validator to catch drift in manual files

## Decision Outcome

Chosen option: "Generate from `command.App` (Go) and `package.toml` (non-Go) as single sources of truth", because it eliminates an entire class of drift bugs by making the binary's own registrations the authoritative source for plugin metadata, and a validator would still require manual maintenance of a second file.

### Consequences

* Good, because plugin manifests can never drift from the binary's actual command structure.
* Good, because skills are auto-discovered from `{skills-dir}/*/SKILL.md` rather than manually listed.
* Bad, because non-Go packages must adopt the `package.toml` convention and depend on the `purse-first` CLI for generation.

## Pros and Cons of the Options

### Keep manual `plugin.json` files

* Good, because no tooling changes required.
* Bad, because manifests drift from actual binary behavior, causing silent breakage.
* Bad, because the same information is maintained in two places.

### Generate from `command.App` (Go) and `package.toml` (non-Go)

* Good, because Go packages already have all needed metadata in `command.App` struct fields (`PluginDescription`, `PluginAuthor`, `MCPArgs`).
* Good, because `generate-plugin` replaces the deprecated `generate-local-plugin` with a cleaner interface.
* Neutral, because nix `postInstall` must call the appropriate generator for each package.
* Bad, because non-Go packages introduce a build-time dependency on the `purse-first` binary.

### Use a schema validator to catch drift

* Good, because existing manual files are preserved.
* Bad, because validation only catches structural errors, not semantic drift (wrong command args, missing tools).
* Bad, because two sources of truth still exist; one is just checked more often.

## More Information

See `docs/plans/2026-02-23-unified-plugin-generation-design.md` for the source-of-truth matrix and per-package generator details.
