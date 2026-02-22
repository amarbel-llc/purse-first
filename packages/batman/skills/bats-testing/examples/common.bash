bats_load_library bats-support
bats_load_library bats-assert
bats_load_library bats-assert-additions

# XDG isolation: route all XDG directories into test temp dir
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

# Test home isolation: fake HOME + XDG + git config
# Call in setup() for any test that touches HOME, git, or config files.
setup_test_home() {
  export REAL_HOME="$HOME"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
  set_xdg "$BATS_TEST_TMPDIR"
  # GIT_CONFIG_GLOBAL takes precedence over XDG_CONFIG_HOME — override it
  mkdir -p "$XDG_CONFIG_HOME/git"
  export GIT_CONFIG_GLOBAL="$XDG_CONFIG_HOME/git/config"
  git config --global user.name "Test User"
  git config --global user.email "test@example.com"
  git config --global init.defaultBranch main
}

# Command wrapper: runs the binary under test with normalized defaults.
# The binary must be on PATH — use `bats --bin-dir <dir>` to inject it.
cmd_defaults=(
  # Add project-specific default flags here to normalize output
  # -print-colors=false
  # -print-time=false
)

run_cmd() {
  subcmd="$1"
  shift
  run timeout --preserve-status "2s" my-command "$subcmd" ${cmd_defaults[@]} "$@"
}
