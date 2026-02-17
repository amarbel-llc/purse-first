#!/bin/bash -e

if [[ -z $BATS_TEST_TMPDIR ]]; then
  echo "BATS_TEST_TMPDIR is not set" >&2
  exit 1
fi

bats_load_library "bats-support"
bats_load_library "bats-assert"

marketplace_result() {
  local result_path="${PURSE_FIRST_RESULT:-$BATS_CWD/result}"
  echo "${result_path}/.claude-plugin/marketplace.json"
}

purse_first_bin() {
  local result_path="${PURSE_FIRST_RESULT:-$BATS_CWD/result}"
  echo "${result_path}/bin/purse-first"
}

hook_payload() {
  local tool_name="$1"
  shift
  # Remaining args are key=value pairs for tool_input
  local tool_input="{"
  local first=true
  for kv in "$@"; do
    local key="${kv%%=*}"
    local val="${kv#*=}"
    if [ "$first" = true ]; then
      first=false
    else
      tool_input+=","
    fi
    tool_input+="\"$key\":\"$val\""
  done
  tool_input+="}"

  cat <<EOF
{"session_id":"bats-test","tool_name":"$tool_name","tool_input":$tool_input,"hook_event_name":"PreToolUse"}
EOF
}
