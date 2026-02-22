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
