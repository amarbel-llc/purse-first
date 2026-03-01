setup_test_home() {
  export REAL_HOME="$HOME"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"

  set_xdg "$BATS_TEST_TMPDIR"

  mkdir -p "$XDG_CONFIG_HOME/git"
  export GIT_CONFIG_GLOBAL="$XDG_CONFIG_HOME/git/config"
  export GIT_CONFIG_SYSTEM=/dev/null
  export GIT_CEILING_DIRECTORIES="$BATS_TEST_TMPDIR"
  export GIT_EDITOR=true

  git config --global user.name "Test User"
  git config --global user.email "test@example.com"
  git config --global init.defaultBranch main
}
