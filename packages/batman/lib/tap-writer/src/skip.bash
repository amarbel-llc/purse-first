tap_skip() {
  _tap_test_num=$((_tap_test_num + 1))
  echo "ok ${_tap_test_num} - $1 # SKIP $2"
}
