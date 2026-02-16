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
