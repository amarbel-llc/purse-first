# Bash Library Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move the TAP-14 bash writer library from batman into tap-dancer and expose it as a nix package with a setup-hook exporting `TAP_DANCER_LIB`.

**Architecture:** New `bash/` directory with `load.bash` entry point and `src/*.bash` modules (moved from batman's `lib/tap-writer/`). Nix derivation `tap-dancer-bash` installs to `$out/share/tap-dancer/lib/` and exports `TAP_DANCER_LIB` via `nix-support/setup-hook`. Exposed as `packages.bash-lib` and included in the default `symlinkJoin`.

**Tech Stack:** Bash, Nix flakes, stdenvNoCC

---

### Task 1: Create bash library files

**Files:**
- Create: `bash/load.bash`
- Create: `bash/src/plan.bash`
- Create: `bash/src/run.bash`
- Create: `bash/src/skip.bash`
- Create: `bash/src/comment.bash`
- Create: `bash/src/bail_out.bash`

**Step 1: Create `bash/src/plan.bash`**

```bash
tap_plan() {
  _tap_plan_declared=1
  echo "1..$1"
}
```

**Step 2: Create `bash/src/run.bash`**

```bash
tap_run() {
  local bail=1
  if [[ $1 == "--no-bail" ]]; then
    bail=0
    shift
  fi

  local desc="$1"
  shift

  _tap_test_num=$((_tap_test_num + 1))

  local output
  if output="$("$@" 2>&1)"; then
    echo "ok ${_tap_test_num} - ${desc}"
  else
    echo "not ok ${_tap_test_num} - ${desc}"
    echo "  ---"
    echo "  output: |"
    echo "${output}" | sed 's/^/    /'
    echo "  ..."
    if [[ $bail -eq 1 ]]; then
      tap_bail_out "${desc} failed"
    fi
  fi
}
```

**Step 3: Create `bash/src/skip.bash`**

```bash
tap_skip() {
  _tap_test_num=$((_tap_test_num + 1))
  echo "ok ${_tap_test_num} - $1 # SKIP $2"
}
```

**Step 4: Create `bash/src/comment.bash`**

```bash
tap_comment() {
  echo "# $1"
}
```

**Step 5: Create `bash/src/bail_out.bash`**

```bash
tap_bail_out() {
  echo "Bail out! $1"
  exit 1
}
```

**Step 6: Create `bash/load.bash`**

```bash
# tap-dancer - TAP version 14 output helpers for bash scripts

# shellcheck disable=1090
source "$(dirname "${BASH_SOURCE[0]}")/src/plan.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/run.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/skip.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/comment.bash"
source "$(dirname "${BASH_SOURCE[0]}")/src/bail_out.bash"

_tap_test_num=0
_tap_plan_declared=0

_tap_trailing_plan() {
  if [[ $_tap_plan_declared -eq 0 ]]; then
    echo "1..${_tap_test_num}"
  fi
}

trap _tap_trailing_plan EXIT

echo "TAP version 14"
```

**Step 7: Commit**

```bash
git add bash/
git commit -m "feat: add TAP-14 bash writer library"
```

---

### Task 2: Add nix derivation and expose as package

**Files:**
- Modify: `flake.nix`

**Step 1: Add `tap-dancer-bash` derivation to `flake.nix`**

Add after the `tap-dancer-skill` definition (around line 94):

```nix
tap-dancer-bash = pkgs.stdenvNoCC.mkDerivation {
  pname = "tap-dancer-bash";
  inherit version;
  src = ./bash;
  dontBuild = true;
  installPhase = ''
    mkdir -p $out/share/tap-dancer/lib/src
    cp load.bash $out/share/tap-dancer/lib/
    cp src/*.bash $out/share/tap-dancer/lib/src/
    mkdir -p $out/nix-support
    echo 'export TAP_DANCER_LIB="'"$out"'/share/tap-dancer/lib"' > $out/nix-support/setup-hook
  '';
};
```

**Step 2: Add `tap-dancer-bash` to the default symlinkJoin**

Update the `paths` list in the default package (around line 100) to include `tap-dancer-bash`.

**Step 3: Expose as `packages.bash-lib`**

Add `bash-lib = tap-dancer-bash;` to the `packages` attrset (around line 109).

**Step 4: Format flake.nix**

Run: `nix run ~/eng/devenvs/nix#fmt -- flake.nix`

**Step 5: Commit**

```bash
git add flake.nix
git commit -m "feat: add tap-dancer-bash nix package with TAP_DANCER_LIB setup-hook"
```

---

### Task 3: Build and verify

**Step 1: Build the full package**

Run: `just build`
Expected: Successful build with no errors.

**Step 2: Verify the bash-lib package output**

Run: `nix build .#bash-lib && ls -R result/share/tap-dancer/lib/`
Expected: `load.bash` and `src/` directory with all 5 source files.

**Step 3: Verify setup-hook exports TAP_DANCER_LIB**

Run: `cat result/nix-support/setup-hook`
Expected: `export TAP_DANCER_LIB="/nix/store/...-tap-dancer-bash-0.1.0/share/tap-dancer/lib"`

**Step 4: Verify load.bash works standalone**

Run: `bash -c 'source result/share/tap-dancer/lib/load.bash && tap_plan 1 && tap_run "echo test" echo hello'`
Expected: TAP-14 output with version line, plan, and passing test point.

**Step 5: Commit (if any fixes were needed)**

```bash
git add -A
git commit -m "fix: address build issues in bash lib packaging"
```
