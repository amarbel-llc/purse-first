#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  setup_test_home
  export output
  # lux uses XDG_RUNTIME_DIR for socket path; setup_test_home only sets XDG_RUNTIME_HOME.
  # Use /tmp to keep paths short (108-byte sun_path limit) and avoid sandcastle restrictions.
  runtime_dir="$(mktemp -d /tmp/lux-bats-XXXXXX)"
  export XDG_RUNTIME_DIR="$runtime_dir"
  lux="$(result_dir)/bin/lux"
}

teardown() {
  if [[ -n "${daemon_pid:-}" ]]; then
    kill "$daemon_pid" 2>/dev/null || true
    wait "$daemon_pid" 2>/dev/null || true
  fi
  [[ -n "${runtime_dir:-}" ]] && rm -rf "$runtime_dir"
  teardown_test_home
}

start_daemon() {
  "$lux" service-run &
  daemon_pid=$!
  local socket="$XDG_RUNTIME_DIR/lux.sock"
  local deadline=$((SECONDS + 5))
  while [[ ! -S "$socket" ]] && [[ $SECONDS -lt $deadline ]]; do
    sleep 0.05
  done
  [[ -S "$socket" ]]
}

function service_run_creates_socket { # @test
  start_daemon
  [[ -S "$XDG_RUNTIME_DIR/lux.sock" ]]
}

function service_status_returns_json { # @test
  start_daemon
  run "$lux" service-status
  assert_success
  assert_output --partial '"session_count"'
}

function service_cleans_up_socket_on_shutdown { # @test
  start_daemon
  local socket="$XDG_RUNTIME_DIR/lux.sock"
  [[ -S "$socket" ]]
  # SIGTERM triggers graceful shutdown via signal handler
  kill "$daemon_pid"
  wait "$daemon_pid" 2>/dev/null || true
  daemon_pid=""
  [[ ! -e "$socket" ]]
}

function service_removes_stale_socket_on_restart { # @test
  start_daemon
  local socket="$XDG_RUNTIME_DIR/lux.sock"
  [[ -S "$socket" ]]
  # SIGKILL bypasses signal handler — socket left behind
  kill -9 "$daemon_pid"
  wait "$daemon_pid" 2>/dev/null || true
  daemon_pid=""
  [[ -e "$socket" ]]
  # A new daemon should remove the stale socket and start successfully
  start_daemon
  [[ -S "$socket" ]]
}
