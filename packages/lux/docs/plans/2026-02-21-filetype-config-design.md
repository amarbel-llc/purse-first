# Filetype Config Design

Restructure lux config into three layers: tool declarations (LSPs,
formatters) and filetype configs that own routing and orchestration. Adds
formatter chaining, fallback, and LSP format control.

## Goals

- Separate tool declarations from file-type routing
- Support formatter chaining (run A then B sequentially)
- Support formatter fallback (try A, fall back to B)
- Control LSP formatting interaction per filetype
- Provide CLI bootstrapping for new configs

## Config File Structure

```
~/.config/lux/
├── lsps.toml           # LSP tool declarations
├── formatters.toml     # Formatter tool declarations
└── filetype/
    ├── go.toml
    ├── python.toml
    ├── javascript.toml
    └── ...

.lux/                   # per-project overrides (same structure)
├── lsps.toml
├── formatters.toml
└── filetype/
    └── go.toml
```

## Tool Declarations

LSPs and formatters become pure declarations with no routing fields.

### lsps.toml

```toml
[[lsp]]
name = "gopls"
flake = "nixpkgs#gopls"
args = []

[lsp.settings]
gofumpt = true
staticcheck = true

[[lsp]]
name = "nil"
flake = "nixpkgs#nil"

[[lsp]]
name = "rust-analyzer"
flake = "nixpkgs#rust-analyzer"
eager_start = true

[lsp.settings.imports]
granularity = { group = "crate" }
```

Fields: `name`, `flake`, `binary`, `args`, `env`, `init_options`, `settings`,
`settings_key`, `capabilities`, `wait_for_ready`, `ready_timeout`,
`activity_timeout`, `eager_start`.

Removed from current schema: `extensions`, `patterns`, `language_ids`.

### formatters.toml

```toml
[[formatter]]
name = "golines"
flake = "nixpkgs#golines"
args = ["--max-len=80", "--no-chain-split-dots", "--shorten-comments", "--base-formatter=gofumpt"]
mode = "filepath"

[[formatter]]
name = "shfmt"
flake = "nixpkgs#shfmt"
args = ["-s", "-i", "2"]
mode = "stdin"

[[formatter]]
name = "isort"
flake = "nixpkgs#isort"
args = ["-"]
mode = "stdin"

[[formatter]]
name = "black"
flake = "nixpkgs#black"
args = ["--quiet", "-"]
mode = "stdin"
```

Fields: `name`, `flake`, `path`, `binary`, `args`, `env`, `mode`, `disabled`.

Removed from current schema: `extensions`, `patterns`.

## Filetype Configs

Each file in `filetype/` owns all routing for one language.

### Schema

```toml
extensions = ["go"]
patterns = ["go.mod", "go.sum"]
language_ids = ["go"]

lsp = "gopls"
formatters = ["golines"]
formatter_mode = "chain"
lsp_format = "fallback"
```

Fields:

- `extensions`, `patterns`, `language_ids` --- at least one required
- `lsp` --- name reference to an `[[lsp]]` entry
- `formatters` --- ordered list of name references to `[[formatter]]` entries
- `formatter_mode` --- `"chain"` (default) or `"fallback"`
- `lsp_format` --- `"never"` (default when formatters set), `"fallback"`,
  or `"prefer"`

### Examples

**filetype/python.toml** --- chain isort then black:

```toml
extensions = ["py"]
language_ids = ["python"]

lsp = "pyright"
formatters = ["isort", "black"]
formatter_mode = "chain"
```

**filetype/javascript.toml** --- try prettierd, fall back to prettier:

```toml
extensions = ["js", "jsx"]
language_ids = ["javascript", "javascriptreact"]

lsp = "typescript-language-server"
formatters = ["prettierd", "prettier"]
formatter_mode = "fallback"
```

**filetype/rust.toml** --- external formatter with LSP fallback:

```toml
extensions = ["rs"]
language_ids = ["rust"]

lsp = "rust-analyzer"
formatters = ["rustfmt"]
lsp_format = "fallback"
```

## Routing & Execution Semantics

### File Matching

Priority (unchanged from current `pkg/filematch`):

1. Language ID (from LSP `textDocument/didOpen`)
2. Extension
3. Glob pattern

First filetype config that matches wins. Filetype configs are loaded
alphabetically by filename. Project overrides take precedence over global.

### Formatter Execution

**chain** (default): Run all formatters sequentially. Output of formatter N
becomes input of formatter N+1. If any formatter fails, stop and return the
error.

**fallback**: Try each formatter in order. Use the first one that is available
and succeeds. Skip to next on failure. If all fail, return the last error.

### LSP Format Control

| `lsp_format` | Behavior |
|---|---|
| `"never"` | Never use LSP formatting. Default when `formatters` is non-empty. |
| `"fallback"` | Use LSP formatting only if all external formatters fail. |
| `"prefer"` | Use LSP formatting instead of external formatters. |

When `formatters` is empty or absent, LSP formatting is used automatically.

### LSP Routing

Each filetype maps to exactly one LSP. No fallback chain for LSPs.

## Config Loading & Validation

### Load Order

1. Load `~/.config/lux/lsps.toml` --- map of LSP declarations by name
2. Load `~/.config/lux/formatters.toml` --- map of formatter declarations by
   name
3. Glob `~/.config/lux/filetype/*.toml` --- list of filetype configs
4. If project root has `.lux/`:
   - Deep-merge `.lux/lsps.toml` into global LSPs (settings, env,
     init_options merge; project wins)
   - Override `.lux/formatters.toml` by name into global formatters
   - For each `.lux/filetype/*.toml`, fully replace the matching global
     filetype config (same filename = same filetype)
5. Validate the assembled config

### Validation Rules

- Each filetype config must have at least one of `extensions`, `patterns`,
  or `language_ids`
- `lsp` must reference a name in the loaded LSP declarations
- Each entry in `formatters` must reference a name in the loaded formatter
  declarations
- `formatter_mode` must be `"chain"` or `"fallback"` (or absent)
- `lsp_format` must be `"never"`, `"fallback"`, or `"prefer"` (or absent)
- No two filetype configs may claim the same extension or language_id
- LSP and formatter names must be unique within their respective files

### Error Messages

Errors should be actionable:

```
filetype/python.toml: formatter "isrt" not found in formatters.toml (did you mean "isort"?)
filetype/go.toml: extension "go" also claimed by filetype/golang.toml
```

## CLI Commands

### `lux init`

Scaffold the config directory structure:

```sh
$ lux init
Created ~/.config/lux/lsps.toml
Created ~/.config/lux/formatters.toml
Created ~/.config/lux/filetype/
```

### `lux init --default`

Scaffold with curated defaults. Refuses to run if config files exist (use
`--force` to overwrite).

**Default LSPs:**

| Name | Flake |
|---|---|
| gopls | nixpkgs#gopls |
| nil | nixpkgs#nil |
| rust-analyzer | nixpkgs#rust-analyzer |
| typescript-language-server | nixpkgs#nodePackages.typescript-language-server |
| pyright | nixpkgs#pyright |
| bash-language-server | nixpkgs#nodePackages.bash-language-server |
| lua-language-server | nixpkgs#lua-language-server |

**Default formatters:**

| Name | Flake | Mode |
|---|---|---|
| golines | nixpkgs#golines | filepath |
| shfmt | nixpkgs#shfmt | stdin |
| nixfmt | nixpkgs#nixfmt-rfc-style | stdin |
| black | nixpkgs#black | stdin |
| isort | nixpkgs#isort | stdin |
| rustfmt | nixpkgs#rustfmt | stdin |
| prettierd | nixpkgs#prettierd | stdin |
| prettier | nixpkgs#nodePackages.prettier | stdin |
| stylua | nixpkgs#stylua | stdin |

**Default filetype configs:**

| File | Extensions | LSP | Formatters | Mode | lsp_format |
|---|---|---|---|---|---|
| go.toml | go | gopls | golines | chain | |
| sh.toml | sh, bash | bash-language-server | shfmt | chain | |
| nix.toml | nix | nil | nixfmt | chain | |
| python.toml | py | pyright | isort, black | chain | |
| rust.toml | rs | rust-analyzer | rustfmt | chain | fallback |
| javascript.toml | js, jsx | typescript-language-server | prettierd, prettier | fallback | |
| typescript.toml | ts, tsx | typescript-language-server | prettierd, prettier | fallback | |
| lua.toml | lua | lua-language-server | stylua | chain | |

### `lux add`

Updated to generate filetype configs alongside tool entries:

```sh
$ lux add nixpkgs#gopls
Probing gopls capabilities...
Added LSP "gopls" to ~/.config/lux/lsps.toml
Created ~/.config/lux/filetype/go.toml (extensions: go, language_ids: go)
```

### `lux add --formatter`

Add a formatter and wire it into a filetype config:

```sh
$ lux add --formatter nixpkgs#shfmt --extensions sh,bash
Added formatter "shfmt" to ~/.config/lux/formatters.toml
Updated ~/.config/lux/filetype/sh.toml (formatters: [shfmt])
```

### `lux add --filetype`

Create or edit a filetype config directly:

```sh
$ lux add --filetype python --extensions py --lsp pyright \
    --formatters isort,black --formatter-mode chain
Created ~/.config/lux/filetype/python.toml
```

### `lux list`

Shows the filetype-centric routing table:

```
Filetype  Extensions  LSP                  Formatters       Mode
go        .go         gopls                golines          chain
python    .py         pyright              isort, black     chain
js        .js, .jsx   typescript-ls        prettierd        fallback
nix       .nix        nil                  nixfmt           chain
sh        .sh, .bash  bash-language-server shfmt            chain
rust      .rs         rust-analyzer        rustfmt          chain (lsp_format: fallback)
lua       .lua        lua-language-server  stylua           chain
```

## Internal Architecture Changes

### New: `internal/config/filetype`

Filetype config struct and loading:

```go
type Config struct {
    Extensions    []string `toml:"extensions"`
    Patterns      []string `toml:"patterns"`
    LanguageIDs   []string `toml:"language_ids"`
    LSP           string   `toml:"lsp"`
    Formatters    []string `toml:"formatters"`
    FormatterMode string   `toml:"formatter_mode"`
    LSPFormat     string   `toml:"lsp_format"`
}
```

### Changes to `internal/config/`

- `config.go`: Remove `Extensions`, `Patterns`, `LanguageIDs` from `LSP`
  struct
- `formatter.go`: Remove `Extensions`, `Patterns` from `Formatter` struct
- New filetype loading, validation, and merging

### Changes to `internal/server/router.go`

Build `filematch.MatcherSet` from filetype configs instead of LSP entries.
Matcher returns a filetype config, router resolves to the LSP declaration.

### Changes to `internal/formatter/router.go`

Build `filematch.MatcherSet` from filetype configs instead of formatter
entries. Returns the filetype config with its ordered formatter list and mode.

### Changes to `internal/formatter/executor.go`

Two execution paths:

- `ExecuteChain(formatters []Formatter, content []byte) ([]byte, error)` ---
  pipe through all sequentially
- `ExecuteFallback(formatters []Formatter, content []byte) ([]byte, error)` ---
  try each, return first success

### Changes to `internal/server/handler.go`

Updated `tryExternalFormat` path:

1. Look up filetype config for the URI
2. If `lsp_format` is `"prefer"`, skip to LSP formatting
3. Resolve formatter names to declarations
4. Execute according to `formatter_mode`
5. If all fail and `lsp_format` is `"fallback"`, delegate to LSP
6. If `lsp_format` is `"never"`, return the error

### Changes to `cmd/lux/`

- `init.go`: New `lux init` and `lux init --default` commands
- `add.go`: Updated to generate filetype configs alongside tool entries
- `list.go`: Updated to show filetype-centric routing table

### Unchanged

- `pkg/filematch/` --- matcher infrastructure stays the same
- `internal/subprocess/` --- LSP process pool
- `internal/control/` --- Unix socket
- `internal/transport/` --- MCP transports
- `internal/capabilities/` --- LSP capability probing
