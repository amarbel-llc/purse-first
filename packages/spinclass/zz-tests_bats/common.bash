bats_load_library bats-support
bats_load_library bats-assert
bats_load_library bats-assert-additions
bats_load_library bats-emo

require_bin SPINCLASS_BIN spinclass

set_xdg() {
  loc="$(realpath "$1" 2>/dev/null)"
  export XDG_DATA_HOME="$loc/.xdg/data"
  export XDG_CONFIG_HOME="$loc/.xdg/config"
  export XDG_STATE_HOME="$loc/.xdg/state"
  export XDG_CACHE_HOME="$loc/.xdg/cache"
  export XDG_RUNTIME_HOME="$loc/.xdg/runtime"
  mkdir -p "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "$XDG_STATE_HOME" \
    "$XDG_CACHE_HOME" "$XDG_RUNTIME_HOME"
}

setup_test_home() {
  export REAL_HOME="$HOME"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
  set_xdg "$BATS_TEST_TMPDIR"
  mkdir -p "$XDG_CONFIG_HOME/git"
  export GIT_CONFIG_GLOBAL="$XDG_CONFIG_HOME/git/config"
  git config --global user.name "Test User"
  git config --global user.email "test@example.com"
  git config --global init.defaultBranch main
  export GIT_EDITOR=true
  export GIT_CEILING_DIRECTORIES="$BATS_TEST_TMPDIR"
}

setup_stubs() {
  local stub_dir="$BATS_TEST_TMPDIR/stubs"
  mkdir -p "$stub_dir"

  for cmd in zmx claude direnv; do
    cat > "$stub_dir/$cmd" <<'STUB'
#!/usr/bin/env bash
printf '%s' "$@" >> "$BATS_TEST_TMPDIR/stubs/CMDNAME.log"
printf '\n' >> "$BATS_TEST_TMPDIR/stubs/CMDNAME.log"
exit 0
STUB
    sed -i "s/CMDNAME/$cmd/g" "$stub_dir/$cmd"
    chmod +x "$stub_dir/$cmd"
  done

  export PATH="$stub_dir:$PATH"
}

# Create a git repo with an initial commit.
# Sets TEST_REPO to the repo path.
create_repo() {
  export TEST_REPO="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$TEST_REPO"
  git -C "$TEST_REPO" init
  echo "initial" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "initial commit"
}

# Create a worktree in the standard .worktrees/ location.
# Usage: create_worktree <branch-name>
# Sets WT_PATH to the worktree path.
create_worktree() {
  local branch="$1"
  local wt_dir="$TEST_REPO/.worktrees"
  mkdir -p "$wt_dir"
  export WT_PATH="$wt_dir/$branch"
  git -C "$TEST_REPO" worktree add -b "$branch" "$WT_PATH"
}

# Run spinclass with timeout.
# Usage: run_sc <subcommand> [args...]
run_sc() {
  local bin="${SPINCLASS_BIN:-spinclass}"
  run timeout --preserve-status 5s "$bin" --format tap "$@"
}
