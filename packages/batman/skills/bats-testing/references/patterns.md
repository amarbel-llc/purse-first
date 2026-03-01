# BATS Testing Patterns Reference

## Command Wrappers

Wrap the binary under test with default flags to normalize output for assertions. This avoids repeating flag boilerplate in every test:

```bash
cmd_defaults=(
  -abbreviate-ids=false
  -abbreviate-shas=false
  -predictable-ids
  -print-types=false
  -print-time=false
  -print-colors=false
)

function run_my_command {
  cmd="$1"
  shift
  run timeout --preserve-status "2s" "$CMD_BIN" "$cmd" ${cmd_defaults[@]} "$@"
}
```

Use `timeout --preserve-status` to prevent hangs from blocking the test suite. The timeout value should be short (1-5 seconds) since integration tests should be fast.

## XDG and HOME Isolation

Use `bats-island` (loaded via `bats_load_library bats-island` in `common.bash`) for test isolation instead of defining helpers inline.

For most tests, call `setup_test_home` / `teardown_test_home` which handle HOME, XDG, and git config redirection:

```bash
setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  setup_test_home
  export output
}

teardown() {
  teardown_test_home
}
```

For tests that also need a git repo, use `setup_test_repo` instead (it calls `setup_test_home` internally):

```bash
setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  setup_test_repo
  export output
  cd "$TEST_REPO"
}
```

If you only need XDG redirection without HOME isolation, use `set_xdg "$BATS_TEST_TMPDIR"` directly. Note that `set_xdg` alone is not sufficient for tools that use their own config env vars (e.g. `GIT_CONFIG_GLOBAL` takes precedence over `XDG_CONFIG_HOME`). Prefer `setup_test_home` which handles these edge cases.

## Fixture Management

### Static Fixtures

Place fixture data in functions for easy reuse:

```bash
cat_fixture_data() (
  echo "line one"
  echo "line two"
  echo "line three"
)
```

Use subshells `()` instead of braces `{}` to avoid polluting the test environment.

### Version-Based Fixtures

For testing migrations or version-specific behavior, organize fixtures by version:

```
zz-tests_bats/
└── migration/
    ├── v1/
    │   └── .app/
    ├── v2/
    │   └── .app/
    └── generate_fixture.bash
```

Copy fixtures into `$BATS_TEST_TMPDIR` during setup:

```bash
function copy_from_version {
  DIR="$1"
  rm -rf "$BATS_TEST_TMPDIR/.app"
  cp -r "$DIR/migration/$APP_VERSION/.app" "$BATS_TEST_TMPDIR/.app"
}
```

### Fixture Generation

For fixtures that are expensive to create, generate them once and cache:

```makefile
# in zz-tests_bats/justfile
test-generate_fixtures:
  ./bin/generate_fixtures.bash
```

Wire fixture generation before test execution in the root justfile.

## Cleanup Patterns

### Basic Cleanup

```bash
teardown() {
  rm -rf "$BATS_TEST_TMPDIR"
}
```

## Custom Assertion: assert_output_unsorted

Sorts output line-by-line before comparing. Essential for commands that produce output in non-deterministic order.

### Implementation Pattern

```bash
assert_output_unsorted() {
  local sorted_output
  sorted_output="$(echo "$output" | sort)"

  local -a args=()
  local expected_from_stdin=false

  while (( $# > 0 )); do
    case "$1" in
      --regexp|-e) args+=("--regexp"); shift ;;
      --partial|-p) args+=("--partial"); shift ;;
      -) expected_from_stdin=true; shift ;;
      *) args+=("$1"); shift ;;
    esac
  done

  if $expected_from_stdin; then
    local expected
    expected="$(cat | sort)"
    output="$sorted_output" assert_output "${args[@]}" "$expected"
  else
    output="$sorted_output" assert_output "${args[@]}"
  fi
}
```

### Usage

```bash
function list_produces_all_items { # @test
  run_my_command list
  assert_success
  assert_output_unsorted - <<-EOM
    item-c
    item-a
    item-b
  EOM
}

function list_matches_pattern { # @test
  run_my_command list
  assert_success
  assert_output_unsorted --regexp - <<-EOM
    item-[0-9]+.*Type
    item-[0-9]+.*Type
  EOM
}
```

## Custom Assertion: assert_output_cut

Pipes output through `cut` before comparing, useful for field-based output:

### Usage

```bash
function show_displays_names { # @test
  run_my_command show --all
  assert_success
  assert_output_cut -d: -f1 - <<-EOM
    alice
    bob
    carol
  EOM
}

function show_displays_sorted_names { # @test
  run_my_command show --all
  assert_success
  assert_output_cut -d: -f1 -s - <<-EOM
    alice
    bob
    carol
  EOM
}
```

The `-s` flag sorts the cut output before comparison.

## Server/Service Testing

For testing CLI tools that start servers:

```bash
function start_server {
  dir="$1"

  coproc server {
    if [[ -n $dir ]]; then
      cd "$dir"
    fi
    my_command serve ${cmd_defaults[@]} tcp :0
  }

  # Read the port from server stdout
  read -r output <&"${server[0]}"

  if [[ $output =~ (starting HTTP server on port: \"([0-9]+)\") ]]; then
    export port="${BASH_REMATCH[2]}"
  fi
}

function stop_server {
  if [[ -n "${server_PID:-}" ]]; then
    kill "$server_PID" 2>/dev/null || true
    wait "$server_PID" 2>/dev/null || true
  fi
}
```

Key patterns:
- Use `coproc` for background processes with stdout/stderr capture
- Bind to port `:0` to let the OS assign a free port
- Parse the port from server output using regex
- Clean up in `teardown()` by killing the coproc

## Heredoc Output Matching

For multi-line expected output, use heredocs with `assert_output -`:

```bash
function command_produces_expected_output { # @test
  run_my_command status
  assert_success
  assert_output - <<-EOM
    status: active
    items: 3
    last-updated: today
  EOM
}
```

The `<<-` (with dash) allows tab indentation in the heredoc for readability while stripping leading tabs from the content.

## Test Tagging

BATS supports filtering tests by tags:

```bash
# bats test_tags=slow,network
function integration_with_remote { # @test
  # ...
}
```

Run specific tags: `bats --filter-tags slow *.bats`

## Parallel Execution Considerations

When running with `--jobs N`:
- Each test gets its own `$BATS_TEST_TMPDIR` (safe by default)
- Avoid shared state between tests (global temp files, ports, etc.)
- Use unique port allocation (`:0`) for server tests
- Ensure fixture generation is idempotent
