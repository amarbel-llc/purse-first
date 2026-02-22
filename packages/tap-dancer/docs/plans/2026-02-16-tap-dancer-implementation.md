# tap-dancer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ship a TAP-14 writer library in Go and Rust, plus a purse-first skill plugin, as a single Nix-backed monorepo.

**Architecture:** Monorepo with `go/`, `rust/`, and `skills/` directories. Nix flake builds both libraries and ships the skill to `$out/share/purse-first/tap-dancer/`. Go uses `buildGoApplication` with gomod2nix overlay; Rust uses crane. DevShell combines `devenvs/go`, `devenvs/rust`, and `devenvs/shell`.

**Tech Stack:** Go 1.23+, Rust stable, Nix flakes, gomod2nix, crane, purse-first skill format

---

### Task 1: Scaffold repo infrastructure

**Files:**
- Create: `.envrc`
- Create: `CLAUDE.md`
- Create: `justfile`
- Create: `.claude-plugin/plugin.json`

**Step 1: Create .envrc**

```
source_up
use flake .
```

**Step 2: Create CLAUDE.md**

```markdown
# CLAUDE.md

## Overview

TAP-14 writer library (Go + Rust) and purse-first skill plugin. Consolidates hand-rolled TAP writers from sweatshop, just-us, and purse-first into shared libraries.

## Build & Test

```sh
just build          # nix build
just test           # Run all tests (Go + Rust)
just test-go        # Go unit tests only
just test-rust      # Rust unit tests only
just fmt            # Format all code
just deps           # Update Go dependencies (go mod tidy + gomod2nix)
```

## Code Style

- Go: `gofumpt`, package name `tap`, module `github.com/amarbel-llc/tap-dancer/go`
- Rust: `cargo fmt` + `cargo clippy`, crate name `tap-dancer`
- Nix: `nixfmt-rfc-style`

## Testing

Both language implementations verify the same TAP-14 spec compliance: version line, plan, test points (ok/not ok), YAML diagnostics, directives (SKIP/TODO), bail out, comments.
```

**Step 3: Create justfile**

```makefile
default:
    @just --list

build:
    nix build

test: test-go test-rust

test-go:
    nix develop --command bash -c "cd go && go test ./..."

test-rust:
    nix develop --command bash -c "cd rust && cargo test"

fmt: fmt-go fmt-rust fmt-nix

fmt-go:
    nix develop --command bash -c "cd go && gofumpt -w ."

fmt-rust:
    nix develop --command bash -c "cd rust && cargo fmt"

fmt-nix:
    nix run ~/eng/devenvs/nix#fmt -- flake.nix

deps:
    nix develop --command bash -c "cd go && go mod tidy && gomod2nix"

clean:
    rm -rf result
```

**Step 4: Create .claude-plugin/plugin.json**

```json
{
  "name": "tap-dancer",
  "description": "TAP-14 writer libraries (Go + Rust) and skill for producing spec-compliant TAP output",
  "author": {
    "name": "friedenberg"
  }
}
```

**Step 5: Commit**

```bash
git add .envrc CLAUDE.md justfile .claude-plugin/plugin.json
git commit -m "Scaffold repo infrastructure with justfile, CLAUDE.md, envrc, and plugin manifest"
```

---

### Task 2: Go library - failing tests

**Files:**
- Create: `go/go.mod`
- Create: `go/tap_test.go`

**Step 1: Create go.mod**

```
module github.com/amarbel-llc/tap-dancer/go

go 1.23.0
```

**Step 2: Write failing tests**

Write `go/tap_test.go` with comprehensive tests covering all API methods. Tests should verify:

1. `NewWriter` emits `TAP version 14\n`
2. `Ok` emits `ok N - description\n` with auto-incrementing N
3. `NotOk` without diagnostics emits `not ok N - description\n`
4. `NotOk` with diagnostics emits YAML block (`---`, sorted keys, `...`)
5. `NotOk` with multiline diagnostic values uses YAML `|` block scalar
6. `Skip` emits `ok N - description # SKIP reason\n`
7. `Todo` emits `not ok N - description # TODO reason\n`
8. `PlanAhead` emits `1..N\n`
9. `Plan` emits `1..N\n` based on tests emitted so far
10. `Plan` with zero tests emits `1..0\n`
11. `BailOut` emits `Bail out! reason\n`
12. `Comment` emits `# text\n`
13. Sequential numbering across mixed Ok/NotOk/Skip/Todo calls
14. All methods returning test numbers return correct sequential values

```go
package tap

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewWriterEmitsVersionHeader(t *testing.T) {
	var buf bytes.Buffer
	NewWriter(&buf)
	if buf.String() != "TAP version 14\n" {
		t.Errorf("expected TAP version 14 header, got: %q", buf.String())
	}
}

func TestOkEmitsLine(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n := tw.Ok("first test")
	if n != 1 {
		t.Errorf("expected test number 1, got %d", n)
	}
	if !strings.Contains(buf.String(), "ok 1 - first test\n") {
		t.Errorf("expected ok line, got: %q", buf.String())
	}
}

func TestNotOkWithoutDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n := tw.NotOk("failing test", nil)
	if n != 1 {
		t.Errorf("expected test number 1, got %d", n)
	}
	if !strings.Contains(buf.String(), "not ok 1 - failing test\n") {
		t.Errorf("expected not ok line, got: %q", buf.String())
	}
	if strings.Contains(buf.String(), "---") {
		t.Error("should not contain YAML block without diagnostics")
	}
}

func TestNotOkWithDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.NotOk("error case", map[string]string{
		"message":  "something broke",
		"severity": "fail",
	})
	out := buf.String()
	if !strings.Contains(out, "  ---\n") {
		t.Errorf("expected YAML start, got: %q", out)
	}
	if !strings.Contains(out, "  message: something broke\n") {
		t.Errorf("expected message diagnostic, got: %q", out)
	}
	if !strings.Contains(out, "  severity: fail\n") {
		t.Errorf("expected severity diagnostic, got: %q", out)
	}
	if !strings.Contains(out, "  ...\n") {
		t.Errorf("expected YAML end, got: %q", out)
	}
}

func TestNotOkWithMultilineDiagnostic(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.NotOk("multiline", map[string]string{
		"output": "line one\nline two",
	})
	out := buf.String()
	if !strings.Contains(out, "output: |\n") {
		t.Errorf("expected YAML block scalar, got: %q", out)
	}
	if !strings.Contains(out, "    line one\n") {
		t.Errorf("expected indented line one, got: %q", out)
	}
	if !strings.Contains(out, "    line two\n") {
		t.Errorf("expected indented line two, got: %q", out)
	}
}

func TestDiagnosticKeysAreSorted(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.NotOk("sorted", map[string]string{
		"zebra": "last",
		"alpha": "first",
	})
	out := buf.String()
	alphaIdx := strings.Index(out, "alpha:")
	zebraIdx := strings.Index(out, "zebra:")
	if alphaIdx >= zebraIdx {
		t.Errorf("expected alpha before zebra in YAML block")
	}
}

func TestSkipEmitsDirective(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n := tw.Skip("skipped test", "not applicable")
	if n != 1 {
		t.Errorf("expected test number 1, got %d", n)
	}
	if !strings.Contains(buf.String(), "ok 1 - skipped test # SKIP not applicable\n") {
		t.Errorf("expected skip line, got: %q", buf.String())
	}
}

func TestTodoEmitsDirective(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n := tw.Todo("unfinished", "not implemented yet")
	if n != 1 {
		t.Errorf("expected test number 1, got %d", n)
	}
	if !strings.Contains(buf.String(), "not ok 1 - unfinished # TODO not implemented yet\n") {
		t.Errorf("expected todo line, got: %q", buf.String())
	}
}

func TestPlanAhead(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.PlanAhead(5)
	if !strings.Contains(buf.String(), "1..5\n") {
		t.Errorf("expected plan line 1..5, got: %q", buf.String())
	}
	_ = tw // suppress unused
}

func TestPlanAfterTests(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Ok("a")
	tw.Ok("b")
	tw.Plan()
	if !strings.HasSuffix(buf.String(), "1..2\n") {
		t.Errorf("expected plan line 1..2, got: %q", buf.String())
	}
}

func TestPlanWithZeroTests(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Plan()
	if !strings.HasSuffix(buf.String(), "1..0\n") {
		t.Errorf("expected plan line 1..0, got: %q", buf.String())
	}
}

func TestBailOut(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.BailOut("database unavailable")
	if !strings.Contains(buf.String(), "Bail out! database unavailable\n") {
		t.Errorf("expected bail out line, got: %q", buf.String())
	}
}

func TestComment(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Comment("this is a comment")
	if !strings.Contains(buf.String(), "# this is a comment\n") {
		t.Errorf("expected comment line, got: %q", buf.String())
	}
}

func TestSequentialNumbering(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n1 := tw.Ok("pass")
	n2 := tw.NotOk("fail", nil)
	n3 := tw.Skip("skip", "lazy")
	n4 := tw.Todo("todo", "later")
	tw.Plan()

	if n1 != 1 || n2 != 2 || n3 != 3 || n4 != 4 {
		t.Errorf("expected 1,2,3,4 got %d,%d,%d,%d", n1, n2, n3, n4)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// lines[0] = "TAP version 14"
	if lines[1] != "ok 1 - pass" {
		t.Errorf("line 1: %q", lines[1])
	}
	if lines[2] != "not ok 2 - fail" {
		t.Errorf("line 2: %q", lines[2])
	}
	if lines[3] != "ok 3 - skip # SKIP lazy" {
		t.Errorf("line 3: %q", lines[3])
	}
	if lines[4] != "not ok 4 - todo # TODO later" {
		t.Errorf("line 4: %q", lines[4])
	}
	if lines[5] != "1..4" {
		t.Errorf("plan line: %q", lines[5])
	}
}
```

**Step 3: Run tests to verify they fail**

Run: `cd go && go test ./...`
Expected: FAIL — `NewWriter` and all methods are undefined

**Step 4: Commit**

```bash
git add go/go.mod go/tap_test.go
git commit -m "Add Go TAP-14 writer tests (red)"
```

---

### Task 3: Go library - implementation

**Files:**
- Create: `go/tap.go`

**Step 1: Implement the writer**

```go
package tap

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type Writer struct {
	w io.Writer
	n int
}

func NewWriter(w io.Writer) *Writer {
	fmt.Fprintln(w, "TAP version 14")
	return &Writer{w: w}
}

func (tw *Writer) Ok(description string) int {
	tw.n++
	fmt.Fprintf(tw.w, "ok %d - %s\n", tw.n, description)
	return tw.n
}

func (tw *Writer) NotOk(description string, diagnostics map[string]string) int {
	tw.n++
	fmt.Fprintf(tw.w, "not ok %d - %s\n", tw.n, description)
	if len(diagnostics) > 0 {
		fmt.Fprintln(tw.w, "  ---")
		keys := make([]string, 0, len(diagnostics))
		for k := range diagnostics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := diagnostics[k]
			if strings.Contains(v, "\n") {
				fmt.Fprintf(tw.w, "  %s: |\n", k)
				lines := strings.Split(v, "\n")
				for len(lines) > 0 && lines[len(lines)-1] == "" {
					lines = lines[:len(lines)-1]
				}
				for _, line := range lines {
					fmt.Fprintf(tw.w, "    %s\n", line)
				}
			} else {
				fmt.Fprintf(tw.w, "  %s: %s\n", k, v)
			}
		}
		fmt.Fprintln(tw.w, "  ...")
	}
	return tw.n
}

func (tw *Writer) Skip(description, reason string) int {
	tw.n++
	fmt.Fprintf(tw.w, "ok %d - %s # SKIP %s\n", tw.n, description, reason)
	return tw.n
}

func (tw *Writer) Todo(description, reason string) int {
	tw.n++
	fmt.Fprintf(tw.w, "not ok %d - %s # TODO %s\n", tw.n, description, reason)
	return tw.n
}

func (tw *Writer) PlanAhead(n int) {
	fmt.Fprintf(tw.w, "1..%d\n", n)
}

func (tw *Writer) Plan() {
	fmt.Fprintf(tw.w, "1..%d\n", tw.n)
}

func (tw *Writer) BailOut(reason string) {
	fmt.Fprintf(tw.w, "Bail out! %s\n", reason)
}

func (tw *Writer) Comment(text string) {
	fmt.Fprintf(tw.w, "# %s\n", text)
}
```

**Step 2: Run tests to verify they pass**

Run: `cd go && go test ./...`
Expected: PASS (all 14 tests)

**Step 3: Format**

Run: `just fmt-go`

**Step 4: Commit**

```bash
git add go/tap.go
git commit -m "Implement Go TAP-14 writer"
```

---

### Task 4: Rust library - failing tests

**Files:**
- Create: `rust/Cargo.toml`
- Create: `rust/src/lib.rs`

**Step 1: Create Cargo.toml**

```toml
[package]
name = "tap-dancer"
version = "0.1.0"
edition = "2021"
description = "TAP-14 writer library"

[dependencies]
```

**Step 2: Write lib.rs with tests and stub types**

Write `rust/src/lib.rs` with the public API structs and function signatures returning `todo!()`, plus comprehensive tests:

```rust
use std::io::{self, Write};

pub struct TestResult {
    pub number: usize,
    pub name: String,
    pub ok: bool,
    pub error_message: Option<String>,
    pub exit_code: Option<i32>,
    pub output: Option<String>,
}

pub struct TapWriter {
    counter: usize,
}

impl TapWriter {
    pub fn new() -> Self {
        Self { counter: 0 }
    }

    pub fn next(&mut self) -> usize {
        self.counter += 1;
        self.counter
    }

    pub fn count(&self) -> usize {
        self.counter
    }
}

pub fn write_version(w: &mut impl Write) -> io::Result<()> {
    todo!()
}

pub fn write_plan(w: &mut impl Write, count: usize) -> io::Result<()> {
    todo!()
}

pub fn write_test_point(w: &mut impl Write, result: &TestResult) -> io::Result<()> {
    todo!()
}

pub fn write_bail_out(w: &mut impl Write, reason: &str) -> io::Result<()> {
    todo!()
}

pub fn write_comment(w: &mut impl Write, text: &str) -> io::Result<()> {
    todo!()
}

pub fn write_skip(w: &mut impl Write, num: usize, desc: &str, reason: &str) -> io::Result<()> {
    todo!()
}

pub fn write_todo(w: &mut impl Write, num: usize, desc: &str, reason: &str) -> io::Result<()> {
    todo!()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn version_line() {
        let mut buf = Vec::new();
        write_version(&mut buf).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "TAP version 14\n");
    }

    #[test]
    fn plan_line() {
        let mut buf = Vec::new();
        write_plan(&mut buf, 3).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "1..3\n");
    }

    #[test]
    fn plan_zero() {
        let mut buf = Vec::new();
        write_plan(&mut buf, 0).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "1..0\n");
    }

    #[test]
    fn passing_test_point() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 1,
            name: "build".into(),
            ok: true,
            error_message: None,
            exit_code: None,
            output: None,
        };
        write_test_point(&mut buf, &result).unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "ok 1 - build\n");
    }

    #[test]
    fn passing_test_point_with_output() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 1,
            name: "build".into(),
            ok: true,
            error_message: None,
            exit_code: None,
            output: Some("building\n".into()),
        };
        write_test_point(&mut buf, &result).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("ok 1 - build\n"));
        assert!(out.contains("  ---\n"));
        assert!(out.contains("  output: |\n"));
        assert!(out.contains("    building\n"));
        assert!(out.contains("  ...\n"));
    }

    #[test]
    fn failing_test_point() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 2,
            name: "test".into(),
            ok: false,
            error_message: Some("something failed".into()),
            exit_code: Some(1),
            output: None,
        };
        write_test_point(&mut buf, &result).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("not ok 2 - test\n"));
        assert!(out.contains("  ---\n"));
        assert!(out.contains("  message: \"something failed\"\n"));
        assert!(out.contains("  severity: fail\n"));
        assert!(out.contains("  exitcode: 1\n"));
        assert!(out.contains("  ...\n"));
    }

    #[test]
    fn failing_test_point_with_multiline_output() {
        let mut buf = Vec::new();
        let result = TestResult {
            number: 1,
            name: "multi".into(),
            ok: false,
            error_message: None,
            exit_code: None,
            output: Some("line one\nline two".into()),
        };
        write_test_point(&mut buf, &result).unwrap();
        let out = String::from_utf8(buf).unwrap();
        assert!(out.contains("  output: |\n"));
        assert!(out.contains("    line one\n"));
        assert!(out.contains("    line two\n"));
    }

    #[test]
    fn bail_out() {
        let mut buf = Vec::new();
        write_bail_out(&mut buf, "database down").unwrap();
        assert_eq!(
            String::from_utf8(buf).unwrap(),
            "Bail out! database down\n"
        );
    }

    #[test]
    fn comment() {
        let mut buf = Vec::new();
        write_comment(&mut buf, "a note").unwrap();
        assert_eq!(String::from_utf8(buf).unwrap(), "# a note\n");
    }

    #[test]
    fn skip_directive() {
        let mut buf = Vec::new();
        write_skip(&mut buf, 3, "optional feature", "not supported").unwrap();
        assert_eq!(
            String::from_utf8(buf).unwrap(),
            "ok 3 - optional feature # SKIP not supported\n"
        );
    }

    #[test]
    fn todo_directive() {
        let mut buf = Vec::new();
        write_todo(&mut buf, 4, "future work", "not implemented").unwrap();
        assert_eq!(
            String::from_utf8(buf).unwrap(),
            "not ok 4 - future work # TODO not implemented\n"
        );
    }

    #[test]
    fn tap_writer_counter() {
        let mut tw = TapWriter::new();
        assert_eq!(tw.count(), 0);
        assert_eq!(tw.next(), 1);
        assert_eq!(tw.next(), 2);
        assert_eq!(tw.count(), 2);
    }
}
```

**Step 3: Run tests to verify they fail**

Run: `cd rust && cargo test`
Expected: FAIL — `todo!()` panics

**Step 4: Commit**

```bash
git add rust/Cargo.toml rust/src/lib.rs
git commit -m "Add Rust TAP-14 writer tests (red)"
```

---

### Task 5: Rust library - implementation

**Files:**
- Modify: `rust/src/lib.rs` (replace `todo!()` stubs with implementations)

**Step 1: Replace all `todo!()` calls with implementations**

```rust
pub fn write_version(w: &mut impl Write) -> io::Result<()> {
    writeln!(w, "TAP version 14")
}

pub fn write_plan(w: &mut impl Write, count: usize) -> io::Result<()> {
    writeln!(w, "1..{count}")
}

fn write_yaml_field(w: &mut impl Write, key: &str, value: &str) -> io::Result<()> {
    if value.contains('\n') {
        writeln!(w, "  {key}: |")?;
        for line in value.lines() {
            writeln!(w, "    {line}")?;
        }
    } else {
        writeln!(w, "  {key}: \"{value}\"")?;
    }
    Ok(())
}

fn has_yaml_block(result: &TestResult) -> bool {
    !result.ok || result.output.is_some()
}

pub fn write_test_point(w: &mut impl Write, result: &TestResult) -> io::Result<()> {
    let status = if result.ok { "ok" } else { "not ok" };
    writeln!(w, "{status} {} - {}", result.number, result.name)?;

    if has_yaml_block(result) {
        writeln!(w, "  ---")?;
        if let Some(ref message) = result.error_message {
            write_yaml_field(w, "message", message)?;
        }
        if !result.ok {
            writeln!(w, "  severity: fail")?;
        }
        if let Some(code) = result.exit_code {
            writeln!(w, "  exitcode: {code}")?;
        }
        if let Some(ref output) = result.output {
            write_yaml_field(w, "output", output)?;
        }
        writeln!(w, "  ...")?;
    }

    Ok(())
}

pub fn write_bail_out(w: &mut impl Write, reason: &str) -> io::Result<()> {
    writeln!(w, "Bail out! {reason}")
}

pub fn write_comment(w: &mut impl Write, text: &str) -> io::Result<()> {
    writeln!(w, "# {text}")
}

pub fn write_skip(w: &mut impl Write, num: usize, desc: &str, reason: &str) -> io::Result<()> {
    writeln!(w, "ok {num} - {desc} # SKIP {reason}")
}

pub fn write_todo(w: &mut impl Write, num: usize, desc: &str, reason: &str) -> io::Result<()> {
    writeln!(w, "not ok {num} - {desc} # TODO {reason}")
}
```

**Step 2: Run tests to verify they pass**

Run: `cd rust && cargo test`
Expected: PASS (all 12 tests)

**Step 3: Format**

Run: `cd rust && cargo fmt`

**Step 4: Commit**

```bash
git add rust/src/lib.rs
git commit -m "Implement Rust TAP-14 writer"
```

---

### Task 6: Purse-first skill

**Files:**
- Create: `skills/tap14/SKILL.md`
- Create: `skills/tap14/references/tap14-spec.md`

**Step 1: Create SKILL.md**

Write `skills/tap14/SKILL.md` with YAML frontmatter and instructional content covering:
- When to use TAP-14 output
- Go library usage (import path, NewWriter, Ok/NotOk/Skip/Todo, Plan, YAML diagnostics)
- Rust library usage (write_version, write_plan, write_test_point, TestResult struct)
- Quick-reference of TAP-14 format elements
- Reference to the full spec

Frontmatter:
```yaml
---
name: TAP-14 Output
description: This skill should be used when the user asks to "format output as TAP", "add TAP output", "produce TAP-14", "write TAP test results", "use tap-dancer", or mentions TAP-14, TAP version 14, TAP output format, TAP writer, or test result formatting.
version: 0.1.0
---
```

**Step 2: Copy TAP-14 spec to references**

Copy from batman: `batman/skills/bats-testing/references/tap14.md` -> `skills/tap14/references/tap14-spec.md`

**Step 3: Commit**

```bash
git add skills/
git commit -m "Add purse-first TAP-14 skill with spec reference"
```

---

### Task 7: Nix flake

**Files:**
- Create: `flake.nix`

**Step 1: Write flake.nix**

The flake needs to:
- Follow stable-first nixpkgs convention (nixpkgs stable + nixpkgs-master)
- Import devenvs: go, rust, shell
- Import crane for Rust builds
- Build Go library with `buildGoApplication` using gomod2nix overlay
- Build Rust library with crane
- Combine both into a default package via `symlinkJoin`
- Ship skill to `$out/share/purse-first/tap-dancer/`
- DevShell combining go + rust + shell devenvs

```nix
{
  description = "TAP-14 writer libraries (Go + Rust) and purse-first skill plugin";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    go.url = "github:friedenberg/eng?dir=devenvs/go";
    rust.url = "github:friedenberg/eng?dir=devenvs/rust";
    shell.url = "github:friedenberg/eng?dir=devenvs/shell";
    crane.url = "github:ipetkov/crane";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, nixpkgs-master, utils, go, rust, shell, crane, rust-overlay }:
    utils.lib.eachDefaultSystem (system:
      let
        overlays = [ (import rust-overlay) ];
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ go.overlays.default ] ++ overlays;
        };

        rustToolchain = pkgs.rust-bin.stable.latest.default;
        craneLib = (crane.mkLib pkgs).overrideToolchain rustToolchain;

        # Go library
        tap-dancer-go = pkgs.buildGoApplication {
          pname = "tap-dancer-go";
          version = "0.1.0";
          src = ./go;
          modules = ./go/gomod2nix.toml;
        };

        # Rust library
        rustSrc = craneLib.cleanCargoSource ./rust;
        tap-dancer-rust = craneLib.buildPackage {
          src = rustSrc;
          strictDeps = true;
        };

        # Skill-only package with purse-first manifest
        tap-dancer-skill = pkgs.runCommand "tap-dancer-skill" {} ''
          mkdir -p $out/share/purse-first/tap-dancer/skills
          cp -r ${./skills}/* $out/share/purse-first/tap-dancer/skills/
          cp ${./.claude-plugin/plugin.json} $out/share/purse-first/tap-dancer/plugin.json
        '';
      in
      {
        packages = {
          default = pkgs.symlinkJoin {
            name = "tap-dancer";
            paths = [ tap-dancer-go tap-dancer-rust tap-dancer-skill ];
          };
          go = tap-dancer-go;
          rust = tap-dancer-rust;
          skill = tap-dancer-skill;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [ just gum ];
          inputsFrom = [
            go.devShells.${system}.default
            rust.devShells.${system}.default
            shell.devShells.${system}.default
          ];
          shellHook = ''
            echo "tap-dancer - dev environment"
          '';
        };
      }
    );
}
```

Use the exact nixpkgs SHAs from the sibling repos:
- `nixpkgs`: `23d72dabcb3b12469f57b37170fcbc1789bd7457`
- `nixpkgs-master`: `b28c4999ed71543e71552ccfd0d7e68c581ba7e9`

**Step 2: Generate gomod2nix.toml**

Since the Go library has no external dependencies, create a minimal `go/gomod2nix.toml`:

```toml
[mod]
```

Run: `nix develop --command bash -c "cd go && gomod2nix"` to verify/regenerate.

**Step 3: Lock flake inputs**

Run: `nix flake update`

**Step 4: Build and verify**

Run: `nix build`
Then: `ls ./result/share/purse-first/tap-dancer/`
Expected: `plugin.json` and `skills/` directory present

**Step 5: Commit**

```bash
git add flake.nix flake.lock go/gomod2nix.toml
git commit -m "Add Nix flake building Go + Rust libraries and shipping purse-first skill"
```

---

### Task 8: Verify full build and test

**Step 1: Run full build**

Run: `just build`
Expected: Build succeeds, `./result/` contains Go and Rust outputs plus skill

**Step 2: Run all tests**

Run: `just test`
Expected: Both Go and Rust tests pass

**Step 3: Verify skill output**

Run: `ls -R ./result/share/purse-first/tap-dancer/`
Expected:
```
plugin.json
skills/tap14/SKILL.md
skills/tap14/references/tap14-spec.md
```

**Step 4: Format all code**

Run: `just fmt`

**Step 5: Commit any formatting changes**

```bash
git add -A
git commit -m "Format all code" || echo "nothing to format"
```

---

### Task 9: Update plugin.json with skills array

**Files:**
- Modify: `.claude-plugin/plugin.json`

**Step 1: Add skills array**

```json
{
  "name": "tap-dancer",
  "description": "TAP-14 writer libraries (Go + Rust) and skill for producing spec-compliant TAP output",
  "author": {
    "name": "friedenberg"
  },
  "skills": [
    "./skills/tap14"
  ]
}
```

**Step 2: Commit**

```bash
git add .claude-plugin/plugin.json
git commit -m "Add skills array to plugin manifest"
```
