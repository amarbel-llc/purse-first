bats_load_library bats-support
bats_load_library bats-assert
bats_load_library bats-emo

require_bin PACKAGE_BIN

run_package_bin() {
  run "$PACKAGE_BIN" "$@"
}
