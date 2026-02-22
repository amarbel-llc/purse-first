# rust-mcp Shell Completion & MCP Protocol Completion Design

## Goal

Add two independent features to `libs/rust-mcp`:

1. **CLI shell completion generation** via a new command framework (`command` feature)
2. **MCP protocol `completion/complete` handler** (`completions` feature)

## Feature 1: Command Framework (`command` feature)

### New Module: `src/command/`

**Types:**

- `App` — name, version, description, global params, registered commands
- `Command` — name, aliases, description (short/long), params, hidden flag
- `Param` — name, short char, param type, description, required, default
- `Description` — short + long strings
- `ParamType` — enum: String, Int, Bool, Float, Array

These mirror go-mcp's `command.App`/`Command`/`Param` exactly.

**Shell Completion Generation:**

`App::generate_completions(dir: &str) -> Result<()>` writes scripts to:

- `{dir}/share/bash-completion/completions/{name}` — bash
- `{dir}/share/zsh/site-functions/_{name}` — zsh
- `{dir}/share/fish/vendor_completions.d/{name}.fish` — fish

Generation logic matches go-mcp:

- **Bash**: completion function with `compgen -W` for subcommands at position 1, case statement per subcommand for flags (long `--name` and short `-x`)
- **Zsh**: `_describe` with command:description pairs
- **Fish**: `complete -c` with `__fish_use_subcommand` / `__fish_seen_subcommand_from` guards, long `-l` and short `-s` flags

Hidden commands are excluded. Commands are sorted alphabetically.

### Cargo Feature

```toml
[features]
command = []
```

Feature-gated in `src/lib.rs`:

```rust
#[cfg(feature = "command")]
pub mod command;
```

## Feature 2: MCP Protocol Completions (`completions` feature)

### Protocol Types: `src/protocol/completions.rs`

```rust
pub struct CompletionReference {
    pub ref_type: String,  // "ref/prompt" or "ref/resource"
    pub name: Option<String>,
    pub uri: Option<String>,
}

pub struct CompletionArgument {
    pub name: String,
    pub value: String,  // current partial value
}

pub struct CompletionCompleteParams {
    pub r#ref: CompletionReference,
    pub argument: CompletionArgument,
}

pub struct CompletionResult {
    pub completion: CompletionValues,
}

pub struct CompletionValues {
    pub values: Vec<String>,
    pub total: Option<usize>,
    pub has_more: bool,
}
```

### Handler Trait: `src/completions/handler.rs`

```rust
#[async_trait]
pub trait CompletionProvider: Send + Sync {
    async fn complete(
        &self,
        params: CompletionCompleteParams,
        ctx: &Context,
    ) -> Result<CompletionResult, CompletionError>;
}
```

### Registry: `src/completions/registry.rs`

Holds a `Vec<Arc<dyn CompletionProvider>>` (or keyed by reference type). Routes `completion/complete` requests to the appropriate provider.

### Integration

- `McpServerBuilder::with_completion_provider(provider)` registers a provider and enables `CompletionsCapability` in V1 capability negotiation
- Dispatcher handles `completion/complete` by delegating to the registry
- Feature-gated behind the existing `completions` Cargo feature flag

## Consumer Migration: chix

1. Switch from clap to rust-mcp's `command::App` for CLI parsing
2. Register commands with params matching current tool schemas
3. Call `app.generate_completions(out_dir)` from a `generate-completions` subcommand
4. Nix build invokes `chix generate-completions $out` to produce shell completion scripts alongside plugin.json
5. Optionally implement `CompletionProvider` for MCP argument completions

## Dependencies Between Features

The two features are independent:

- `command` = CLI framework + shell completion file generation
- `completions` = MCP protocol `completion/complete` JSON-RPC handler

A package can use either or both. They share no types or code paths.
