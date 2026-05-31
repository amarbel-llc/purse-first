#!/bin/bash -e

if [[ -z $BATS_TEST_TMPDIR ]]; then
  echo "BATS_TEST_TMPDIR is not set" >&2
  exit 1
fi

bats_load_library "bats-support"
bats_load_library "bats-assert"

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

# Create an isolated HOME with XDG dirs and minimal git config.
setup_test_home() {
  export REAL_HOME="$HOME"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"

  local loc
  loc="$(realpath "$BATS_TEST_TMPDIR")"
  export XDG_DATA_HOME="$loc/.xdg/data"
  export XDG_CONFIG_HOME="$loc/.xdg/config"
  export XDG_STATE_HOME="$loc/.xdg/state"
  export XDG_CACHE_HOME="$loc/.xdg/cache"
  export XDG_RUNTIME_HOME="$loc/.xdg/runtime"
  mkdir -p "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "$XDG_STATE_HOME" \
    "$XDG_CACHE_HOME" "$XDG_RUNTIME_HOME"

  mkdir -p "$XDG_CONFIG_HOME/git"
  export GIT_CONFIG_GLOBAL="$XDG_CONFIG_HOME/git/config"
  export GIT_CONFIG_SYSTEM=/dev/null
  export GIT_CEILING_DIRECTORIES="$BATS_TEST_TMPDIR"
  export GIT_EDITOR=true

  git config --global user.name "Test User"
  git config --global user.email "test@example.com"
  git config --global init.defaultBranch main
}

teardown_test_home() {
  chflags -R nouchg "$BATS_TEST_TMPDIR" 2>/dev/null || true
}

require_bin PURSE_FIRST_BIN purse-first

result_dir() {
  local result_path="${PURSE_FIRST_RESULT:-$BATS_CWD/result}"
  echo "${result_path}"
}

marketplace_result() {
  echo "$(result_dir)/.claude-plugin/marketplace.json"
}

purse_first_bin() {
  echo "${PURSE_FIRST_BIN:-purse-first}"
}

plugin_share_dir() {
  echo "$(result_dir)/share/purse-first/$1"
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
