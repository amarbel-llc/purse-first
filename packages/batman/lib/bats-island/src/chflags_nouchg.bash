
chflags_nouchg() {
  chflags -R nouchg "$BATS_TEST_TMPDIR" 2>/dev/null || true
}
