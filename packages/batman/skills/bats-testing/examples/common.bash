bats_load_library bats-support
bats_load_library bats-assert
bats_load_library bats-assert-additions
bats_load_library bats-island

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
