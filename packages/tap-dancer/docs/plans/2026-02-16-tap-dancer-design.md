# tap-dancer Design

TAP-14 writer library (Go + Rust) and purse-first skill plugin.

## Problem

Three sibling repos (sweatshop, just-us, purse-first) each hand-roll TAP-14 writers. No shared library exists, leading to duplicated logic and potential spec drift. Claude also lacks a skill for producing spec-compliant TAP-14 output.

## Solution

A monorepo shipping:
1. Go package (`github.com/amarbel-llc/tap-dancer/go`) -- public TAP-14 writer
2. Rust crate (`tap-dancer`) -- public TAP-14 writer
3. Purse-first skill teaching Claude when and how to use TAP-14

## Repository Structure

```
tap-dancer/
├── flake.nix
├── flake.lock
├── justfile
├── LICENSE
├── CLAUDE.md
├── .envrc
├── .claude-plugin/
│   └── plugin.json
├── go/
│   ├── go.mod
│   ├── tap.go
│   └── tap_test.go
├── rust/
│   ├── Cargo.toml
│   └── src/
│       └── lib.rs
└── skills/
    └── tap14/
        ├── SKILL.md
        └── references/
            └── tap14-spec.md
```

## Go Library API

```go
package tap

type Writer struct { w io.Writer; n int }

func NewWriter(w io.Writer) *Writer       // Emits "TAP version 14\n"
func (tw *Writer) Ok(description string) int
func (tw *Writer) NotOk(description string, diagnostics map[string]string) int
func (tw *Writer) Skip(description, reason string) int
func (tw *Writer) Todo(description, reason string) int
func (tw *Writer) PlanAhead(n int)
func (tw *Writer) Plan()
func (tw *Writer) BailOut(reason string)
func (tw *Writer) Comment(text string)
```

Design decisions:
- Matches sweatshop's proven API shape (Ok/NotOk/Skip/PlanAhead/Plan)
- Adds Todo, BailOut, Comment for full TAP-14 coverage
- YAML diagnostics via sorted `map[string]string` with multiline `|` block support
- Auto-incrementing test numbering
- Returns test number from Ok/NotOk/Skip/Todo for caller reference

## Rust Library API

```rust
pub struct TapWriter { counter: usize }
pub struct TestResult {
    pub number: usize,
    pub name: String,
    pub ok: bool,
    pub error_message: Option<String>,
    pub exit_code: Option<i32>,
    pub output: Option<String>,
}

pub fn write_version(w: &mut impl Write) -> io::Result<()>
pub fn write_plan(w: &mut impl Write, count: usize) -> io::Result<()>
pub fn write_test_point(w: &mut impl Write, result: &TestResult) -> io::Result<()>
pub fn write_bail_out(w: &mut impl Write, reason: &str) -> io::Result<()>
pub fn write_comment(w: &mut impl Write, text: &str) -> io::Result<()>
pub fn write_skip(w: &mut impl Write, num: usize, desc: &str, reason: &str) -> io::Result<()>
pub fn write_todo(w: &mut impl Write, num: usize, desc: &str, reason: &str) -> io::Result<()>
```

Design decisions:
- Matches just-us's proven function-based approach
- TestResult struct with optional fields for YAML diagnostic blocks
- Separate functions for each TAP element (composable)
- `write_test_point` handles YAML diagnostic block generation

## Purse-First Skill

**SKILL.md** triggers on: "TAP output", "TAP-14", "format as TAP", "TAP version 14", "test output format", "TAP writer".

Content:
- When to use TAP-14 (CLI tools, test runners, recipe executors)
- Go usage examples with tap-dancer
- Rust usage examples with tap-dancer
- TAP-14 quick-reference (version, plan, test points, YAML diagnostics, directives)
- Full spec in `references/tap14-spec.md` (sourced from batman's copy)

**plugin.json:**
```json
{
  "name": "tap-dancer",
  "description": "TAP-14 writer libraries (Go + Rust) and skill for producing spec-compliant TAP output",
  "author": { "name": "friedenberg" }
}
```

## Nix Build

- Go: `buildGoModule` targeting `./go`
- Rust: `crane.buildPackage` targeting `./rust`
- Skill: Copy skills/ to `$out/share/purse-first/tap-dancer/skills/`
- DevShell: `devenvs/go` + `devenvs/shell` + rust toolchain
- Stable-first nixpkgs convention (nixpkgs stable, nixpkgs-master)

## Testing

- Go: `go test ./...` in `go/`
- Rust: `cargo test` in `rust/`
- Both verify spec compliance against shared TAP-14 examples
- Justfile: `test-go`, `test-rust`, `test` (both)

## Migration Path

After tap-dancer ships:
1. sweatshop replaces `internal/tap/` with `tap-dancer/go` import
2. purse-first replaces inline TAP writing with `tap-dancer/go`
3. just-us can optionally adopt `tap-dancer` Rust crate
