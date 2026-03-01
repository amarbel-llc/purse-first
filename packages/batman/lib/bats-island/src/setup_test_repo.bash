setup_test_repo() {
  if [[ -z "${REAL_HOME:-}" ]]; then
    setup_test_home
  fi

  export TEST_REPO="${1:-$BATS_TEST_TMPDIR/repo}"
  mkdir -p "$TEST_REPO"
  git -C "$TEST_REPO" init
  echo "initial" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "initial commit"
}
