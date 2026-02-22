tap_bail_out() {
  _tap_bailed=1
  echo "Bail out! $1"
  exit 1
}
