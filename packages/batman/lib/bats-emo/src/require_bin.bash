# Validate that a binary under test is available.
#
# Usage:
#   require_bin GRIT_BIN grit     # env var or PATH fallback
#   require_bin BATS_WRAPPER      # env var only (no PATH fallback)
#
# If the env var is set, verifies the path is executable.
# If the env var is unset and a command name is provided, checks PATH.
# Fails with a clear message if the binary is unavailable.
require_bin() {
  local var_name="$1"
  local cmd_name="${2:-}"
  local var_value="${!var_name:-}"

  if [[ -n "$var_value" ]]; then
    if [[ ! -x "$var_value" ]]; then
      echo "error: $var_name=$var_value is not executable" >&2
      exit 1
    fi
  elif [[ -n "$cmd_name" ]]; then
    if ! command -v "$cmd_name" &>/dev/null; then
      echo "error: $cmd_name not found. Set $var_name or use --bin-dir" >&2
      exit 1
    fi
  else
    echo "error: $var_name not set" >&2
    exit 1
  fi
}
