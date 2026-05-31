bats_load_library bats-support
bats_load_library bats-assert

# Validate that a binary under test is available.
# Usage: require_bin VAR_NAME [command-name]
require_bin() {
  local var_name="$1"
  local cmd_name="${2:-}"
  local var_value="${!var_name:-}"

  if [[ -n $var_value ]]; then
    if [[ ! -x $var_value ]]; then
      echo "error: $var_name=$var_value is not executable" >&2
      exit 1
    fi
  elif [[ -n $cmd_name ]]; then
    if ! command -v "$cmd_name" &>/dev/null; then
      echo "error: $cmd_name not found. Set $var_name or add to PATH" >&2
      exit 1
    fi
  else
    echo "error: $var_name not set" >&2
    exit 1
  fi
}

require_bin PACKAGE_BIN

run_package_bin() {
  run "$PACKAGE_BIN" "$@"
}
