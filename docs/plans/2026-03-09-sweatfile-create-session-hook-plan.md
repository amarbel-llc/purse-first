# Sweatfile Create-Session Hook Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `envrc-directives`, `[env]` table, and `[hooks]` table to the sweatfile format, and rename all snake_case TOML keys to snob-case.

**Architecture:** Four incremental changes to the sweatfile struct and its consumers: (1) rename TOML tags, (2) replace `StopHook` with `[hooks]` table, (3) add `envrc-directives`, (4) add `[env]` table. Each change is independently testable and committable.

**Tech Stack:** Go, TOML (`github.com/BurntSushi/toml`), `os.Expand` for variable interpolation, BATS for integration tests.

**Design doc:** `docs/plans/2026-03-09-sweatfile-create-session-hook-design.md`

---

### Task 1: Rename TOML Tags to Snob-Case

**Files:**
- Modify: `packages/spinclass/internal/sweatfile/sweatfile.go:23,26`
- Modify: `packages/spinclass/internal/sweatfile/sweatfile_test.go` (all TOML string literals)
- Modify: `packages/spinclass/internal/validate/validate.go` (string references to field names)
- Modify: `packages/spinclass/internal/validate/validate_test.go` (TOML string literals)
- Modify: `packages/spinclass/internal/shop/shop.go:73-85` (log key strings)
- Modify: `packages/spinclass/internal/hooks/hooks.go:52,66` (comments)
- Modify: `packages/spinclass/internal/hooks/hooks_test.go:148,184,213` (TOML string literals)
- Modify: `packages/spinclass/zz-tests_bats/sweatfile.bats:15,36,41` (TOML string literals)
- Modify: `packages/spinclass/zz-tests_bats/validate.bats:12-13` (TOML string literals)
- Modify: `sweatfile` (repo root sweatfile)

**Step 1: Update TOML tags on struct**

In `packages/spinclass/internal/sweatfile/sweatfile.go`, change:
```go
GitSkipIndex       []string `toml:"git-excludes"`
ClaudeAllow        []string `toml:"claude-allow"`
StopHook           *string  `toml:"stop-hook"`
```
Remove the TODO comments on those lines.

**Step 2: Update all TOML string literals in unit tests**

In `packages/spinclass/internal/sweatfile/sweatfile_test.go`, replace every occurrence of:
- `git_excludes` → `git-excludes` (in TOML string literals)
- `claude_allow` → `claude-allow` (in TOML string literals)
- `stop_hook` → `stop-hook` (in TOML string literals)

Keep Go field names (`GitSkipIndex`, `ClaudeAllow`, `StopHook`) unchanged.

**Step 3: Update validate field name strings**

In `packages/spinclass/internal/validate/validate.go`, replace:
- `"git_excludes"` → `"git-excludes"` (in Issue.Field, error messages, and sub.Ok/sub.NotOk strings)
- `"claude_allow"` → `"claude-allow"` (same)

In `packages/spinclass/internal/validate/validate_test.go`, replace:
- `git_excludes` → `git-excludes` in TOML string literals
- `claude_allow` → `claude-allow` in TOML string literals
- `stop_hook` → `stop-hook` in TOML string literals

**Step 4: Update shop.go log key strings**

In `packages/spinclass/internal/shop/shop.go`, replace:
- `"git_excludes"` → `"git-excludes"` (lines 73, 84)
- `"claude_allow"` → `"claude-allow"` (lines 75, 85)

**Step 5: Update hooks.go comments and hooks_test.go TOML literals**

In `packages/spinclass/internal/hooks/hooks.go`:
- Line 52 comment: `stop_hook` → `stop-hook`
- Line 66: `"stop_hook failed: %s"` → `"stop-hook failed: %s"`

In `packages/spinclass/internal/hooks/hooks_test.go`:
- `stop_hook` → `stop-hook` in TOML string literals (lines 148, 184, 213)
- Update comments referencing `stop_hook`

**Step 6: Update BATS test TOML literals**

In `packages/spinclass/zz-tests_bats/sweatfile.bats`:
- `claude_allow` → `claude-allow` in TOML string literals

In `packages/spinclass/zz-tests_bats/validate.bats`:
- `claude_allow` → `claude-allow` in TOML string literals
- `git_excludes` → `git-excludes` in TOML string literals

**Step 7: Update repo root sweatfile**

In `sweatfile`:
- `stop_hook` → `stop-hook` (currently commented out)

**Step 8: Run tests**

Run: `nix develop --command go test ./packages/spinclass/...`
Expected: all tests pass

**Step 9: Commit**

```
feat(spinclass): rename sweatfile TOML keys from snake_case to snob-case

git_excludes → git-excludes, claude_allow → claude-allow,
stop_hook → stop-hook. No backward compatibility — old keys are
flagged as unknown by the validator.
```

---

### Task 2: Replace StopHook with [hooks] Table

**Files:**
- Modify: `packages/spinclass/internal/sweatfile/sweatfile.go:15-29` (struct)
- Modify: `packages/spinclass/internal/sweatfile/hierarchy.go:133-135` (merge)
- Modify: `packages/spinclass/internal/sweatfile/apply.go:157` (StopHook reference)
- Modify: `packages/spinclass/internal/hooks/hooks.go:51,55,66` (StopHook references)
- Modify: `packages/spinclass/internal/sweatfile/sweatfile_test.go` (stop hook tests)
- Modify: `packages/spinclass/internal/sweatfile/apply_test.go:225-273` (stop hook tests)
- Modify: `packages/spinclass/internal/hooks/hooks_test.go` (stop hook tests)

**Step 1: Write failing tests for Hooks struct parsing**

Add to `packages/spinclass/internal/sweatfile/sweatfile_test.go`:

```go
func TestParseHooksCreate(t *testing.T) {
	input := `
[hooks]
create = "composer install"
`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Hooks == nil || sf.Hooks.Create == nil || *sf.Hooks.Create != "composer install" {
		t.Errorf("hooks.create: got %v", sf.Hooks)
	}
}

func TestParseHooksStop(t *testing.T) {
	input := `
[hooks]
stop = "just test"
`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Hooks == nil || sf.Hooks.Stop == nil || *sf.Hooks.Stop != "just test" {
		t.Errorf("hooks.stop: got %v", sf.Hooks)
	}
}

func TestParseHooksBoth(t *testing.T) {
	input := `
[hooks]
create = "npm install"
stop = "just lint"
`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Hooks == nil {
		t.Fatal("expected non-nil hooks")
	}
	if sf.Hooks.Create == nil || *sf.Hooks.Create != "npm install" {
		t.Errorf("hooks.create: got %v", sf.Hooks.Create)
	}
	if sf.Hooks.Stop == nil || *sf.Hooks.Stop != "just lint" {
		t.Errorf("hooks.stop: got %v", sf.Hooks.Stop)
	}
}

func TestParseHooksAbsent(t *testing.T) {
	sf, err := Parse([]byte(`git-excludes = [".claude/"]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Hooks != nil {
		t.Errorf("expected nil hooks, got %v", sf.Hooks)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test -run TestParseHooks ./packages/spinclass/internal/sweatfile/`
Expected: FAIL (Hooks field doesn't exist yet)

**Step 3: Update struct — add Hooks, remove StopHook**

In `packages/spinclass/internal/sweatfile/sweatfile.go`:

Replace:
```go
type Experimental struct {
	BoundaryNotify *bool `toml:"boundary-notify"`
}

type Sweatfile struct {
	SystemPrompt       *string  `toml:"system-prompt"`
	SystemPromptAppend *string  `toml:"system-prompt-append"`
	BranchNameCommand  string   `toml:"branch-name-command"`
	GitSkipIndex       []string `toml:"git-excludes"`
	ClaudeAllow        []string `toml:"claude-allow"`
	StopHook           *string  `toml:"stop-hook"`
	Experimental       *Experimental `toml:"experimental"`
}
```

With:
```go
type Hooks struct {
	Create *string `toml:"create"`
	Stop   *string `toml:"stop"`
}

type Experimental struct {
	BoundaryNotify *bool `toml:"boundary-notify"`
}

type Sweatfile struct {
	SystemPrompt       *string       `toml:"system-prompt"`
	SystemPromptAppend *string       `toml:"system-prompt-append"`
	BranchNameCommand  string        `toml:"branch-name-command"`
	GitSkipIndex       []string      `toml:"git-excludes"`
	ClaudeAllow        []string      `toml:"claude-allow"`
	Hooks              *Hooks        `toml:"hooks"`
	Experimental       *Experimental `toml:"experimental"`
}
```

**Step 4: Add StopHook helper method**

Add to `packages/spinclass/internal/sweatfile/sweatfile.go`:

```go
func (sf Sweatfile) StopHookCommand() *string {
	if sf.Hooks == nil {
		return nil
	}
	return sf.Hooks.Stop
}

func (sf Sweatfile) CreateHookCommand() *string {
	if sf.Hooks == nil {
		return nil
	}
	return sf.Hooks.Create
}
```

**Step 5: Update merge logic**

In `packages/spinclass/internal/sweatfile/hierarchy.go`, replace the `StopHook` merge block:

```go
if repo.StopHook != nil {
	merged.StopHook = repo.StopHook
}
```

With:
```go
if repo.Hooks != nil {
	if merged.Hooks == nil {
		merged.Hooks = &Hooks{}
	}
	if repo.Hooks.Create != nil {
		merged.Hooks.Create = repo.Hooks.Create
	}
	if repo.Hooks.Stop != nil {
		merged.Hooks.Stop = repo.Hooks.Stop
	}
}
```

**Step 6: Update apply.go — use helper method**

In `packages/spinclass/internal/sweatfile/apply.go`, replace:
```go
if sweatfile.StopHook != nil && *sweatfile.StopHook != "" {
```
With:
```go
if cmd := sweatfile.StopHookCommand(); cmd != nil && *cmd != "" {
```

**Step 7: Update hooks.go — use helper method**

In `packages/spinclass/internal/hooks/hooks.go`, replace:
```go
if err != nil || result.Merged.StopHook == nil || *result.Merged.StopHook == "" {
	return nil // no stop-hook -> approve
}

cmd := exec.Command("sh", "-c", *result.Merged.StopHook)
```
With:
```go
stopCmd := result.Merged.StopHookCommand()
if err != nil || stopCmd == nil || *stopCmd == "" {
	return nil // no stop hook configured -> approve
}

cmd := exec.Command("sh", "-c", *stopCmd)
```

And replace:
```go
reason := fmt.Sprintf("stop-hook failed: %s", *result.Merged.StopHook)
```
With:
```go
reason := fmt.Sprintf("stop hook failed: %s", *stopCmd)
```

**Step 8: Update existing stop hook tests to use [hooks] table syntax**

In `packages/spinclass/internal/sweatfile/sweatfile_test.go`, update all stop hook test TOML strings from:
```toml
stop-hook = "just test"
```
To:
```toml
[hooks]
stop = "just test"
```

And update Go struct literals from `Sweatfile{StopHook: &cmd}` to `Sweatfile{Hooks: &Hooks{Stop: &cmd}}`.

In `packages/spinclass/internal/sweatfile/apply_test.go`, update:
- `TestApplyClaudeSettingsWritesStopHookWhenConfigured`: `Sweatfile{StopHook: &cmd}` → `Sweatfile{Hooks: &Hooks{Stop: &cmd}}`
- `TestApplyClaudeSettingsNoStopHookWhenNotConfigured`: already uses `Sweatfile{}`, no change needed.

In `packages/spinclass/internal/hooks/hooks_test.go`, update TOML literals from:
```toml
stop_hook = "false"
```
To:
```toml
[hooks]
stop = "false"
```
(Note: the rename in Task 1 already changed `stop_hook` to `stop-hook`; this task changes the format to `[hooks]\nstop = ...`)

**Step 9: Add merge tests for hooks**

Add to `packages/spinclass/internal/sweatfile/sweatfile_test.go`:

```go
func TestMergeHooksCreateInherit(t *testing.T) {
	cmd := "npm install"
	base := Sweatfile{Hooks: &Hooks{Create: &cmd}}
	repo := Sweatfile{}
	merged := Merge(base, repo)
	if merged.Hooks == nil || merged.Hooks.Create == nil || *merged.Hooks.Create != "npm install" {
		t.Errorf("expected inherited hooks.create, got %v", merged.Hooks)
	}
}

func TestMergeHooksCreateOverride(t *testing.T) {
	baseCmd := "npm install"
	repoCmd := "composer install"
	base := Sweatfile{Hooks: &Hooks{Create: &baseCmd}}
	repo := Sweatfile{Hooks: &Hooks{Create: &repoCmd}}
	merged := Merge(base, repo)
	if merged.Hooks == nil || merged.Hooks.Create == nil || *merged.Hooks.Create != "composer install" {
		t.Errorf("expected overridden hooks.create, got %v", merged.Hooks)
	}
}

func TestMergeHooksCreateClear(t *testing.T) {
	baseCmd := "npm install"
	empty := ""
	base := Sweatfile{Hooks: &Hooks{Create: &baseCmd}}
	repo := Sweatfile{Hooks: &Hooks{Create: &empty}}
	merged := Merge(base, repo)
	if merged.Hooks == nil || merged.Hooks.Create == nil || *merged.Hooks.Create != "" {
		t.Errorf("expected cleared hooks.create, got %v", merged.Hooks)
	}
}

func TestMergeHooksIndependentFields(t *testing.T) {
	createCmd := "npm install"
	stopCmd := "just test"
	base := Sweatfile{Hooks: &Hooks{Create: &createCmd}}
	repo := Sweatfile{Hooks: &Hooks{Stop: &stopCmd}}
	merged := Merge(base, repo)
	if merged.Hooks == nil {
		t.Fatal("expected non-nil hooks")
	}
	if merged.Hooks.Create == nil || *merged.Hooks.Create != "npm install" {
		t.Errorf("expected inherited hooks.create, got %v", merged.Hooks.Create)
	}
	if merged.Hooks.Stop == nil || *merged.Hooks.Stop != "just test" {
		t.Errorf("expected overridden hooks.stop, got %v", merged.Hooks.Stop)
	}
}
```

**Step 10: Run tests**

Run: `nix develop --command go test ./packages/spinclass/...`
Expected: all tests pass

**Step 11: Update repo root sweatfile**

In `sweatfile`, change:
```toml
# stop_hook = "just build test"
```
To:
```toml
# [hooks]
# stop = "just build test"
```

**Step 12: Commit**

```
feat(spinclass): replace stop-hook with [hooks] table

Introduces Hooks struct with create and stop fields. stop-hook is
removed as a top-level field. Both hooks merge independently using
scalar override semantics.
```

---

### Task 3: Add envrc-directives

**Files:**
- Modify: `packages/spinclass/internal/sweatfile/sweatfile.go` (add field)
- Modify: `packages/spinclass/internal/sweatfile/hierarchy.go` (merge logic)
- Modify: `packages/spinclass/internal/sweatfile/apply.go` (rewrite writeEnvrc)
- Modify: `packages/spinclass/internal/sweatfile/sweatfile_test.go` (add tests)
- Modify: `packages/spinclass/internal/sweatfile/apply_test.go` (update envrc tests)
- Modify: `packages/spinclass/zz-tests_bats/sweatfile.bats` (update BATS tests)

**Step 1: Write failing tests for envrc-directives parsing**

Add to `packages/spinclass/internal/sweatfile/sweatfile_test.go`:

```go
func TestParseEnvrcDirectives(t *testing.T) {
	input := `envrc-directives = ["source_up", "dotenv_if_exists"]`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sf.EnvrcDirectives) != 2 {
		t.Fatalf("expected 2 envrc-directives, got %v", sf.EnvrcDirectives)
	}
	if sf.EnvrcDirectives[0] != "source_up" || sf.EnvrcDirectives[1] != "dotenv_if_exists" {
		t.Errorf("envrc-directives: got %v", sf.EnvrcDirectives)
	}
}

func TestParseEnvrcDirectivesAbsent(t *testing.T) {
	sf, err := Parse([]byte(`git-excludes = [".claude/"]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.EnvrcDirectives != nil {
		t.Errorf("expected nil envrc-directives, got %v", sf.EnvrcDirectives)
	}
}

func TestMergeEnvrcDirectivesAppend(t *testing.T) {
	base := Sweatfile{EnvrcDirectives: []string{"source_up"}}
	repo := Sweatfile{EnvrcDirectives: []string{"dotenv_if_exists"}}
	merged := Merge(base, repo)
	if len(merged.EnvrcDirectives) != 2 {
		t.Fatalf("expected 2 envrc-directives, got %v", merged.EnvrcDirectives)
	}
}

func TestMergeEnvrcDirectivesClear(t *testing.T) {
	base := Sweatfile{EnvrcDirectives: []string{"source_up"}}
	repo := Sweatfile{EnvrcDirectives: []string{}}
	merged := Merge(base, repo)
	if len(merged.EnvrcDirectives) != 0 {
		t.Errorf("expected cleared envrc-directives, got %v", merged.EnvrcDirectives)
	}
}

func TestMergeEnvrcDirectivesInherit(t *testing.T) {
	base := Sweatfile{EnvrcDirectives: []string{"source_up"}}
	merged := Merge(base, Sweatfile{})
	if len(merged.EnvrcDirectives) != 1 || merged.EnvrcDirectives[0] != "source_up" {
		t.Errorf("expected inherited envrc-directives, got %v", merged.EnvrcDirectives)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test -run TestParseEnvrcDirectives ./packages/spinclass/internal/sweatfile/`
Expected: FAIL

**Step 3: Add EnvrcDirectives field to struct**

In `packages/spinclass/internal/sweatfile/sweatfile.go`, add to the Sweatfile struct:
```go
EnvrcDirectives    []string      `toml:"envrc-directives"`
```

**Step 4: Add merge logic for EnvrcDirectives**

In `packages/spinclass/internal/sweatfile/hierarchy.go`, add after the `ClaudeAllow` merge block:
```go
if repo.EnvrcDirectives != nil {
	if len(repo.EnvrcDirectives) == 0 {
		merged.EnvrcDirectives = []string{}
	} else {
		merged.EnvrcDirectives = append(base.EnvrcDirectives, repo.EnvrcDirectives...)
	}
}
```

**Step 5: Run parse/merge tests**

Run: `nix develop --command go test -run "TestParseEnvrcDirectives|TestMergeEnvrcDirectives" ./packages/spinclass/internal/sweatfile/`
Expected: PASS

**Step 6: Write failing tests for envrc rendering**

Add to `packages/spinclass/internal/sweatfile/apply_test.go`:

```go
func TestWriteEnvrcWithDirectives(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{EnvrcDirectives: []string{"source_up", "dotenv_if_exists"}}
	err := sf.prepareDirenv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	content := string(data)

	binAbs, _ := filepath.Abs(".git/spinclass/bin")
	wantPathAdd := fmt.Sprintf("PATH_add \"%s\"\n", binAbs)
	want := "source_up\ndotenv_if_exists\n" + wantPathAdd
	if content != want {
		t.Errorf(".envrc content:\ngot  %q\nwant %q", content, want)
	}
}

func TestWriteEnvrcDefaultFallbackWithFlake(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "flake.nix"), []byte("{}"), 0o644)

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{} // nil EnvrcDirectives = use default
	err := sf.prepareDirenv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	content := string(data)

	binAbs, _ := filepath.Abs(".git/spinclass/bin")
	wantPathAdd := fmt.Sprintf("PATH_add \"%s\"\n", binAbs)
	want := "source_up\nuse flake\n" + wantPathAdd
	if content != want {
		t.Errorf(".envrc content:\ngot  %q\nwant %q", content, want)
	}
}

func TestWriteEnvrcDefaultFallbackWithoutFlake(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{} // nil EnvrcDirectives, no flake.nix
	err := sf.prepareDirenv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	content := string(data)

	binAbs, _ := filepath.Abs(".git/spinclass/bin")
	wantPathAdd := fmt.Sprintf("PATH_add \"%s\"\n", binAbs)
	want := "source_up\n" + wantPathAdd
	if content != want {
		t.Errorf(".envrc content:\ngot  %q\nwant %q", content, want)
	}
}
```

**Step 7: Rewrite writeEnvrc and prepareDirenv to be methods on Sweatfile**

In `packages/spinclass/internal/sweatfile/apply.go`, replace `writeEnvrc` and `prepareDirenv` with:

```go
func (sf Sweatfile) writeEnvrc(worktreePath string) error {
	file, err := os.OpenFile(
		filepath.Join(worktreePath, ".envrc"),
		os.O_TRUNC|os.O_CREATE|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return err
	}
	defer file.Close()

	bufferedWriter := bufio.NewWriter(file)

	directives := sf.EnvrcDirectives
	if directives == nil {
		// Default fallback: source_up + use flake (if flake.nix exists)
		directives = []string{"source_up"}
		if _, ok := fileExists(filepath.Join(worktreePath, "flake.nix")); ok {
			directives = append(directives, "use flake")
		}
	}

	for _, directive := range directives {
		if _, err := fmt.Fprintln(bufferedWriter, directive); err != nil {
			return err
		}
	}

	dirSpinclassBinAbs, err := filepath.Abs(".git/spinclass/bin")
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(
		bufferedWriter,
		"PATH_add \"%s\"\n",
		dirSpinclassBinAbs,
	); err != nil {
		return err
	}

	return bufferedWriter.Flush()
}

func (sf Sweatfile) prepareDirenv(worktreePath string) error {
	direnvPath, err := exec.LookPath("direnv")
	if err != nil {
		return nil
	}

	if err := sf.writeEnvrc(worktreePath); err != nil {
		return err
	}

	cmd := exec.Command(direnvPath, "allow")
	cmd.Dir = worktreePath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
```

**Step 8: Update Apply to call method**

In `packages/spinclass/internal/sweatfile/apply.go`, change:
```go
if err := prepareDirenv(worktreePath); err != nil {
```
To:
```go
if err := sweatfile.prepareDirenv(worktreePath); err != nil {
```

(The receiver variable `sweatfile` is already the name used in the `Apply` method.)

**Step 9: Update existing envrc tests**

The existing `prepareDirenv` tests in `apply_test.go` call the package-level function `prepareDirenv(dir)`. Since it's now a method on `Sweatfile`, update them to call `Sweatfile{}.prepareDirenv(dir)` for default behavior tests. Same for the ones that create a `flake.nix`.

**Step 10: Update BATS envrc tests**

In `packages/spinclass/zz-tests_bats/sweatfile.bats`, the existing tests `apply_writes_envrc_when_flake_exists` and `apply_skips_use_flake_without_flake_nix` test default behavior (no `envrc-directives` set), so they should continue to pass without changes since `nil` falls back to the old behavior.

**Step 11: Run all tests**

Run: `nix develop --command go test ./packages/spinclass/...`
Expected: all tests pass

**Step 12: Commit**

```
feat(spinclass): add envrc-directives to sweatfile

Allows repos to specify custom direnv directives instead of the
default source_up + use flake. Merge semantics: nil = inherit,
[] = clear, non-empty = append.
```

---

### Task 4: Add [env] Table

**Files:**
- Modify: `packages/spinclass/internal/sweatfile/sweatfile.go` (add field)
- Modify: `packages/spinclass/internal/sweatfile/hierarchy.go` (merge logic)
- Modify: `packages/spinclass/internal/sweatfile/apply.go` (write .spinclass.env, auto-directive)
- Modify: `packages/spinclass/internal/sweatfile/sweatfile_test.go` (add tests)
- Modify: `packages/spinclass/internal/sweatfile/apply_test.go` (add tests)

**Step 1: Write failing tests for [env] parsing**

Add to `packages/spinclass/internal/sweatfile/sweatfile_test.go`:

```go
func TestParseEnv(t *testing.T) {
	input := `
[env]
FOO = "bar"
BAZ = "qux"
`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sf.Env) != 2 {
		t.Fatalf("expected 2 env vars, got %v", sf.Env)
	}
	if sf.Env["FOO"] != "bar" || sf.Env["BAZ"] != "qux" {
		t.Errorf("env: got %v", sf.Env)
	}
}

func TestParseEnvAbsent(t *testing.T) {
	sf, err := Parse([]byte(`git-excludes = [".claude/"]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.Env != nil {
		t.Errorf("expected nil env, got %v", sf.Env)
	}
}

func TestMergeEnvInherit(t *testing.T) {
	base := Sweatfile{Env: map[string]string{"FOO": "bar"}}
	repo := Sweatfile{}
	merged := Merge(base, repo)
	if merged.Env["FOO"] != "bar" {
		t.Errorf("expected inherited env, got %v", merged.Env)
	}
}

func TestMergeEnvOverrideKey(t *testing.T) {
	base := Sweatfile{Env: map[string]string{"FOO": "bar", "BAZ": "qux"}}
	repo := Sweatfile{Env: map[string]string{"FOO": "override"}}
	merged := Merge(base, repo)
	if merged.Env["FOO"] != "override" {
		t.Errorf("expected overridden FOO, got %v", merged.Env["FOO"])
	}
	if merged.Env["BAZ"] != "qux" {
		t.Errorf("expected inherited BAZ, got %v", merged.Env["BAZ"])
	}
}

func TestMergeEnvAddKey(t *testing.T) {
	base := Sweatfile{Env: map[string]string{"FOO": "bar"}}
	repo := Sweatfile{Env: map[string]string{"BAZ": "qux"}}
	merged := Merge(base, repo)
	if len(merged.Env) != 2 {
		t.Fatalf("expected 2 env vars, got %v", merged.Env)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test -run TestParseEnv ./packages/spinclass/internal/sweatfile/`
Expected: FAIL

**Step 3: Add Env field to struct**

In `packages/spinclass/internal/sweatfile/sweatfile.go`, add to the Sweatfile struct:
```go
Env                map[string]string `toml:"env"`
```

**Step 4: Add merge logic for Env**

In `packages/spinclass/internal/sweatfile/hierarchy.go`, add after the `EnvrcDirectives` merge block:
```go
if repo.Env != nil {
	if merged.Env == nil {
		merged.Env = make(map[string]string)
	}
	for k, v := range repo.Env {
		merged.Env[k] = v
	}
}
```

**Step 5: Run parse/merge tests**

Run: `nix develop --command go test -run "TestParseEnv|TestMergeEnv" ./packages/spinclass/internal/sweatfile/`
Expected: PASS

**Step 6: Write failing tests for .spinclass.env rendering and auto-directive**

Add to `packages/spinclass/internal/sweatfile/apply_test.go`:

```go
func TestWriteSpinclassEnv(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{
		Env: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
	}
	err := sf.Apply(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".spinclass.env"))
	if err != nil {
		t.Fatalf("reading .spinclass.env: %v", err)
	}

	content := string(data)
	// Sorted by key
	if content != "BAZ=qux\nFOO=bar\n" {
		t.Errorf(".spinclass.env content: got %q", content)
	}
}

func TestWriteSpinclassEnvInterpolatesWorktree(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{
		Env: map[string]string{
			"INCLUDE_PATH": "$WORKTREE/lib:.",
		},
	}
	err := sf.Apply(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".spinclass.env"))
	want := fmt.Sprintf("INCLUDE_PATH=%s/lib:.\n", dir)
	if string(data) != want {
		t.Errorf(".spinclass.env content:\ngot  %q\nwant %q", string(data), want)
	}
}

func TestEnvAutoDotenvDirective(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{
		Env: map[string]string{"FOO": "bar"},
	}
	err := sf.Apply(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	content := string(data)
	if !strings.Contains(content, "dotenv .spinclass.env") {
		t.Errorf("expected dotenv .spinclass.env in .envrc, got %q", content)
	}
}

func TestNoEnvNoDotenvDirective(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{}
	err := sf.Apply(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	content := string(data)
	if strings.Contains(content, "dotenv") {
		t.Errorf("expected no dotenv in .envrc when env is empty, got %q", content)
	}
}
```

**Step 7: Add writeSpinclassEnv method**

In `packages/spinclass/internal/sweatfile/apply.go`, add:

```go
func (sf Sweatfile) writeSpinclassEnv(worktreePath string) error {
	if len(sf.Env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(sf.Env))
	for k := range sf.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	file, err := os.OpenFile(
		filepath.Join(worktreePath, ".spinclass.env"),
		os.O_TRUNC|os.O_CREATE|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return err
	}
	defer file.Close()

	expand := func(key string) string {
		if key == "WORKTREE" {
			return worktreePath
		}
		return os.Getenv(key)
	}

	for _, k := range keys {
		expanded := os.Expand(sf.Env[k], expand)
		if _, err := fmt.Fprintf(file, "%s=%s\n", k, expanded); err != nil {
			return err
		}
	}

	return nil
}
```

Add `"sort"` to imports.

**Step 8: Update writeEnvrc to append auto-directive**

In the `writeEnvrc` method (from Task 3), after writing the user directives but before `PATH_add`, add:

```go
if len(sf.Env) > 0 {
	if _, err := fmt.Fprintln(bufferedWriter, "dotenv .spinclass.env"); err != nil {
		return err
	}
}
```

**Step 9: Update Apply to call writeSpinclassEnv**

In `packages/spinclass/internal/sweatfile/apply.go`, in the `Apply` method, add before `prepareDirenv`:

```go
if err := sweatfile.writeSpinclassEnv(worktreePath); err != nil {
	return fmt.Errorf("writing .spinclass.env: %w", err)
}
```

Note: `writeSpinclassEnv` must be called before `prepareDirenv` because `writeEnvrc` (called inside `prepareDirenv`) checks `len(sf.Env)` to decide whether to add the `dotenv` directive.

**Step 10: Add `strings` import to apply_test.go if not already present**

Check and add if needed.

**Step 11: Run all tests**

Run: `nix develop --command go test ./packages/spinclass/...`
Expected: all tests pass

**Step 12: Commit**

```
feat(spinclass): add [env] table to sweatfile

Declares environment variables written to .spinclass.env with
os.Expand interpolation ($WORKTREE plus process env). Auto-adds
dotenv .spinclass.env to envrc when env is non-empty.
```

---

### Task 5: Add Create Hook Execution

**Files:**
- Modify: `packages/spinclass/internal/worktree/worktree.go:139-161` (call create hook)
- Modify: `packages/spinclass/internal/sweatfile/apply.go` (add runCreateHook)
- Test: `packages/spinclass/internal/sweatfile/apply_test.go` (add tests)

**Step 1: Write failing tests for create hook execution**

Add to `packages/spinclass/internal/sweatfile/apply_test.go`:

```go
func TestRunCreateHookExecutes(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "hook-ran")

	cmd := fmt.Sprintf("touch %s", marker)
	sf := Sweatfile{Hooks: &Hooks{Create: &cmd}}

	err := sf.RunCreateHook(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); os.IsNotExist(err) {
		t.Error("expected create hook to run and create marker file")
	}
}

func TestRunCreateHookReceivesWorktreeEnv(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "worktree-path")

	cmd := fmt.Sprintf("echo $WORKTREE > %s", output)
	sf := Sweatfile{Hooks: &Hooks{Create: &cmd}}

	err := sf.RunCreateHook(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(output)
	got := strings.TrimSpace(string(data))
	if got != dir {
		t.Errorf("WORKTREE env: got %q, want %q", got, dir)
	}
}

func TestRunCreateHookFailureReturnsError(t *testing.T) {
	dir := t.TempDir()

	cmd := "exit 1"
	sf := Sweatfile{Hooks: &Hooks{Create: &cmd}}

	err := sf.RunCreateHook(dir)
	if err == nil {
		t.Error("expected error from failing create hook")
	}
}

func TestRunCreateHookNilIsNoop(t *testing.T) {
	dir := t.TempDir()
	sf := Sweatfile{}

	err := sf.RunCreateHook(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCreateHookEmptyStringIsNoop(t *testing.T) {
	dir := t.TempDir()
	empty := ""
	sf := Sweatfile{Hooks: &Hooks{Create: &empty}}

	err := sf.RunCreateHook(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test -run TestRunCreateHook ./packages/spinclass/internal/sweatfile/`
Expected: FAIL

**Step 3: Implement RunCreateHook**

In `packages/spinclass/internal/sweatfile/apply.go`, add:

```go
func (sf Sweatfile) RunCreateHook(worktreePath string) error {
	cmd := sf.CreateHookCommand()
	if cmd == nil || *cmd == "" {
		return nil
	}

	c := exec.Command("sh", "-c", *cmd)
	c.Dir = worktreePath
	c.Env = append(os.Environ(), "WORKTREE="+worktreePath)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	return c.Run()
}
```

**Step 4: Run create hook tests**

Run: `nix develop --command go test -run TestRunCreateHook ./packages/spinclass/internal/sweatfile/`
Expected: PASS

**Step 5: Integrate into worktree creation**

In `packages/spinclass/internal/worktree/worktree.go`, in `applyWorktreeConfig`, add after `claude.TrustWorkspace`:

```go
if err := sweetfile.Merged.RunCreateHook(worktreePath); err != nil {
	// Clean up the worktree on hook failure
	git.RunPassthrough(repoPath, "worktree", "remove", "--force", worktreePath)
	return fmt.Errorf("create hook failed: %w", err)
}
```

**Step 6: Run all tests**

Run: `nix develop --command go test ./packages/spinclass/...`
Expected: all tests pass

**Step 7: Commit**

```
feat(spinclass): add create hook execution on worktree setup

The [hooks] create command runs after worktree config is applied.
Receives $WORKTREE env var. Failure removes the worktree.
```

---

### Task 6: Update Global Sweatfile and Documentation

**Files:**
- Modify: `rcm/config/spinclass/sweatfile` (in eng repo — note for user)
- Modify: `packages/spinclass/CLAUDE.md` (update docs)

**Step 1: Note for user about global sweatfile**

The global sweatfile at `~/.config/spinclass/sweatfile` (managed via rcm in the eng repo) needs to be updated from `git_excludes`/`claude_allow` to `git-excludes`/`claude-allow`. This is in a different repo (`eng/rcm/config/spinclass/sweatfile`) — capture as a TODO item rather than modifying it here.

**Step 2: Update spinclass CLAUDE.md**

In `packages/spinclass/CLAUDE.md`, update any references to `git_excludes`, `claude_allow`, `stop_hook` to their new snob-case names. Update the sweatfile description to mention `envrc-directives`, `[env]`, and `[hooks]`.

**Step 3: Commit**

```
docs(spinclass): update CLAUDE.md for new sweatfile format
```

---

### Task 7: Run Full Test Suite and BATS Integration Tests

**Step 1: Run Go tests**

Run: `nix develop --command go test ./packages/spinclass/...`
Expected: all pass

**Step 2: Run BATS tests (if buildable)**

Run: `nix develop --command bats --tap packages/spinclass/zz-tests_bats/sweatfile.bats packages/spinclass/zz-tests_bats/validate.bats`
Expected: all pass (BATS tests use the old key names in their TOML literals — these were updated in Task 1)

**Step 3: Verify no unknown-field regressions**

Run: `nix develop --command go test -run TestCheckUnknownFields ./packages/spinclass/internal/validate/`
Expected: PASS (the `TestCheckUnknownFieldsClean` test now uses snob-case keys)
