bats_load_library bats-support
bats_load_library bats-assert
bats_load_library bats-assert-additions

set_xdg() {
  loc="$(realpath "$1" 2>/dev/null)"
  export XDG_DATA_HOME="$loc/.xdg/data"
  export XDG_CONFIG_HOME="$loc/.xdg/config"
  export XDG_STATE_HOME="$loc/.xdg/state"
  export XDG_CACHE_HOME="$loc/.xdg/cache"
  export XDG_RUNTIME_HOME="$loc/.xdg/runtime"
  mkdir -p "$XDG_DATA_HOME" "$XDG_CONFIG_HOME" "$XDG_STATE_HOME" \
    "$XDG_CACHE_HOME" "$XDG_RUNTIME_HOME"
}

setup_test_home() {
  export REAL_HOME="$HOME"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
  set_xdg "$BATS_TEST_TMPDIR"
  mkdir -p "$XDG_CONFIG_HOME/git"
  export GIT_CONFIG_GLOBAL="$XDG_CONFIG_HOME/git/config"
  git config --global user.name "Test User"
  git config --global user.email "test@example.com"
  git config --global init.defaultBranch main
  export GIT_EDITOR=true
}

chflags_and_rm() {
  chflags -R nouchg "$BATS_TEST_TMPDIR" 2>/dev/null || true
  rm -rf "$BATS_TEST_TMPDIR"
}

# Create a git repo with an initial commit
setup_test_repo() {
  setup_test_home
  export TEST_REPO="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$TEST_REPO"
  git -C "$TEST_REPO" init
  echo "initial" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "initial commit"
}

# Create a conflict scenario:
# - main branch has one change to file.txt
# - feature branch has a different change to file.txt
# After calling, you're on the "feature" branch ready to rebase onto main.
setup_conflict_scenario() {
  setup_test_repo

  # Create divergent change on main
  echo "main change" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "main: modify file"

  # Create feature branch from initial commit with conflicting change
  git -C "$TEST_REPO" checkout -b feature HEAD~1
  echo "feature change" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "feature: modify file"
}

# Create a clean rebase scenario:
# - main branch has changes to file_a.txt
# - feature branch has changes to file_b.txt (no overlap)
# After calling, you're on the "feature" branch ready to rebase onto main.
setup_clean_rebase_scenario() {
  setup_test_repo

  # Create change on main to a different file
  echo "main addition" > "$TEST_REPO/file_a.txt"
  git -C "$TEST_REPO" add file_a.txt
  git -C "$TEST_REPO" commit -m "main: add file_a"

  # Create feature branch from initial commit with non-conflicting change
  git -C "$TEST_REPO" checkout -b feature HEAD~1
  echo "feature addition" > "$TEST_REPO/file_b.txt"
  git -C "$TEST_REPO" add file_b.txt
  git -C "$TEST_REPO" commit -m "feature: add file_b"
}

# Send a JSON-RPC tools/call request to grit and capture the response.
# Usage: run_grit_mcp <tool_name> <json_args>
# Sets $output to the parsed result content text (JSON), and $status to exit code.
run_grit_mcp() {
  local tool_name="$1"
  local tool_args="$2"
  local grit_bin="${GRIT_BIN:-grit}"

  local init_request='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}'
  local initialized_notification='{"jsonrpc":"2.0","method":"notifications/initialized"}'
  local call_request
  call_request=$(printf '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"%s","arguments":%s}}' "$tool_name" "$tool_args")

  local response
  response=$(printf '%s\n%s\n%s\n' "$init_request" "$initialized_notification" "$call_request" \
    | timeout --preserve-status 5s "$grit_bin" 2>/dev/null \
    | grep -F '"id":2' \
    | head -1)

  if [ -z "$response" ]; then
    echo "no response from grit"
    return 1
  fi

  # Extract the text content from the result
  echo "$response" | jq -r '.result.content[0].text'
}
