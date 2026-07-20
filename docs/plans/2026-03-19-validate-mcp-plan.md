# validate-mcp Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Add `purse-first validate-mcp <binary> [args...]` that spawns an MCP server, validates initialization + tools/list + resources/list + resources/templates/list over stdio JSON-RPC, and reports results.

**Architecture:** New `internal/validate/mcp.go` implements a lightweight MCP client using go-mcp's `jsonrpc.Conn` and `protocol` types. The CLI gains a `validate-mcp` subcommand and `validate --type mcp` alias. BATS integration tests verify against the repo's own MCP server binaries.

**Tech Stack:** Go, go-mcp (`jsonrpc`, `protocol`), cobra CLI, BATS

**Rollback:** Purely additive — remove the files and cobra registration to revert.

---

### Task 1: Core MCP validation logic

**Files:**
- Create: `internal/validate/mcp.go`
- Create: `internal/validate/mcp_test.go`

**Step 1: Write the failing test**

Create `internal/validate/mcp_test.go`:

```go
package validate

import (
	"context"
	"testing"
)

func TestValidateMCPBinaryNotFound(t *testing.T) {
	r, err := ValidateMCP(context.Background(), "/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
	if r != nil {
		t.Fatal("expected nil result on error")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -run TestValidateMCPBinaryNotFound ./internal/validate/`
Expected: FAIL — `ValidateMCP` undefined

**Step 3: Write the implementation**

Create `internal/validate/mcp.go`:

```go
package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
	"code.linenisgreat.com/purse-first/libs/go-mcp/protocol"
)

const mcpValidateTimeout = 10 * time.Second

// ValidateMCP spawns the binary as an MCP server over stdio and validates
// initialization, tools/list, resources/list, and resources/templates/list.
func ValidateMCP(ctx context.Context, binary string, args ...string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, mcpValidateTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", binary, err)
	}

	conn := jsonrpc.NewConn(stdout, stdin, nil)

	errCh := make(chan error, 1)
	go func() { errCh <- conn.Run(ctx) }()

	r := &Result{}

	// 1. Initialize
	initResult, err := mcpInitialize(ctx, conn)
	if err != nil {
		r.addError("initialize", err.Error())
		stdin.Close()
		cmd.Wait()
		return r, nil
	}
	r.addInfo("initialize", fmt.Sprintf("ok: %s %s (protocol %s)",
		initResult.ServerInfo.Name,
		initResult.ServerInfo.Version,
		initResult.ProtocolVersion))

	// 2. tools/list
	mcpValidateToolsList(ctx, conn, r)

	// 3. resources/list
	mcpValidateResourcesList(ctx, conn, r)

	// 4. resources/templates/list
	mcpValidateResourceTemplatesList(ctx, conn, r)

	// 5. Shutdown
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		r.addWarning("shutdown", fmt.Sprintf("process exited with: %v", err))
	}

	return r, nil
}

func mcpInitialize(ctx context.Context, conn *jsonrpc.Conn) (*protocol.InitializeResultV1, error) {
	params := protocol.InitializeParamsV1{
		ProtocolVersion: "2025-03-26",
		Capabilities:    protocol.ClientCapabilitiesV1{},
		ClientInfo: protocol.ImplementationV1{
			Name:    "purse-first-validate",
			Version: "0.1.0",
		},
	}

	raw, err := conn.Call(ctx, "initialize", params)
	if err != nil {
		return nil, fmt.Errorf("initialize call failed: %w", err)
	}

	var result protocol.InitializeResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("invalid initialize response: %w", err)
	}

	if result.ProtocolVersion == "" {
		return nil, fmt.Errorf("missing protocolVersion in response")
	}

	if err := conn.Notify("notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("initialized notification failed: %w", err)
	}

	return &result, nil
}

func mcpValidateToolsList(ctx context.Context, conn *jsonrpc.Conn, r *Result) {
	raw, err := conn.Call(ctx, "tools/list", nil)
	if err != nil {
		r.addError("tools/list", fmt.Sprintf("call failed: %v", err))
		return
	}

	var result protocol.ToolsListResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		r.addError("tools/list", fmt.Sprintf("invalid response: %v", err))
		return
	}

	if len(result.Tools) == 0 {
		r.addError("tools/list", "no tools returned")
		return
	}

	r.addInfo("tools/list", fmt.Sprintf("ok: %d tools", len(result.Tools)))

	for _, tool := range result.Tools {
		if tool.Annotations == nil {
			r.addWarning("tools/list", fmt.Sprintf("tool %q has no annotations", tool.Name))
		}
	}
}

func mcpValidateResourcesList(ctx context.Context, conn *jsonrpc.Conn, r *Result) {
	raw, err := conn.Call(ctx, "resources/list", nil)
	if err != nil {
		// Not all servers support resources — method not found is acceptable
		if isMethodNotFound(err) {
			r.addInfo("resources/list", "not supported (method not found)")
			return
		}
		r.addError("resources/list", fmt.Sprintf("call failed: %v", err))
		return
	}

	var result protocol.ResourcesListResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		r.addError("resources/list", fmt.Sprintf("invalid response: %v", err))
		return
	}

	r.addInfo("resources/list", fmt.Sprintf("ok: %d resources", len(result.Resources)))
}

func mcpValidateResourceTemplatesList(ctx context.Context, conn *jsonrpc.Conn, r *Result) {
	raw, err := conn.Call(ctx, "resources/templates/list", nil)
	if err != nil {
		if isMethodNotFound(err) {
			r.addInfo("resources/templates/list", "not supported (method not found)")
			return
		}
		r.addError("resources/templates/list", fmt.Sprintf("call failed: %v", err))
		return
	}

	var result protocol.ResourceTemplatesListResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		r.addError("resources/templates/list", fmt.Sprintf("invalid response: %v", err))
		return
	}

	r.addInfo("resources/templates/list", fmt.Sprintf("ok: %d templates", len(result.ResourceTemplates)))
}

func isMethodNotFound(err error) bool {
	if rpcErr, ok := err.(*jsonrpc.Error); ok {
		return rpcErr.Code == jsonrpc.MethodNotFound
	}
	return false
}
```

**Step 4: Add `addInfo` to Result type**

Modify `internal/validate/types.go` — add an `Info` severity and `addInfo` method:

```go
const (
	Error Severity = iota
	Warning
	Info
)

func (s Severity) String() string {
	switch s {
	case Info:
		return "info"
	case Warning:
		return "warning"
	default:
		return "error"
	}
}
```

Add to `Result`:

```go
func (r *Result) addInfo(path, msg string) {
	r.issues = append(r.issues, Issue{Severity: Info, Path: path, Message: msg})
}
```

**Step 5: Run test to verify it passes**

Run: `nix develop --command go test -run TestValidateMCPBinaryNotFound ./internal/validate/`
Expected: PASS

**Step 6: Commit**

```
feat: add MCP server validation via stdio JSON-RPC

Spawns an MCP binary, validates initialize handshake, tools/list
(non-empty + annotations), resources/list, and resources/templates/list
schema responses.
```

---

### Task 2: Wire up CLI subcommand

**Files:**
- Modify: `cmd/purse-first/main.go:182-270`

**Step 1: Add `validate-mcp` subcommand**

After the `validateCmd` definition (around line 227), add:

```go
validateMCPCmd := &cobra.Command{
	Use:   "validate-mcp <binary> [args...]",
	Short: "Validate a running MCP server over stdio",
	Long: `Spawn an MCP server binary and validate its protocol responses.

Checks: initialize handshake, tools/list (non-empty, annotations present),
resources/list (schema), and resources/templates/list (schema).`,
	Args:          cobra.MinimumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runValidateMCP(args[0], args[1:]...)
	},
}
```

Add to root.AddCommand (line 270):

```go
root.AddCommand(installCmd, installSelfCmd, genMarketplaceCmd, installLocalCmd, installDevMCPCmd, genPluginCmd, validateCmd, validateMCPCmd, packageCmd)
```

**Step 2: Extend `validate --type mcp`**

In `parseDocType` (line 277), add a case for `"mcp"` that returns a new `MCPDoc` constant. In the `validateCmd.RunE`, before the `os.Stat` call, add:

```go
if docType == validate.MCPDoc {
	if len(args) == 0 {
		return fmt.Errorf("validate --type mcp requires a binary path")
	}
	return runValidateMCP(args[0])
}
```

**Step 3: Add `runValidateMCP` helper**

```go
func runValidateMCP(binary string, args ...string) error {
	r, err := validate.ValidateMCP(context.Background(), binary, args...)
	if err != nil {
		return err
	}

	for _, issue := range r.Issues() {
		fmt.Fprintf(os.Stderr, "%s\n", issue)
	}

	if r.HasErrors() {
		return fmt.Errorf("MCP validation failed")
	}

	return nil
}
```

Add `"context"` to imports.

**Step 4: Add MCPDoc to detect.go**

In `internal/validate/detect.go`, add `MCPDoc` to the DocType constants:

```go
const (
	Unknown DocType = iota
	PluginDoc
	MappingDoc
	MarketplaceDoc
	MCPDoc
)
```

And in `DocType.String()`:

```go
case MCPDoc:
	return "mcp"
```

**Step 5: Verify it compiles**

Run: `nix develop --command go build ./cmd/purse-first`
Expected: success

**Step 6: Commit**

```
wire validate-mcp subcommand and validate --type mcp alias
```

---

### Task 3: BATS integration tests

**Files:**
- Create: `zz-tests_bats/validate_mcp.bats`

**Prerequisite:** `nix build` must succeed (need a working MCP binary in result/).

**Step 1: Write the test file**

Create `zz-tests_bats/validate_mcp.bats`:

```bash
#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  purse_first="$(purse_first_bin)"
  result="$(result_dir)"
}

function validate_mcp_nonexistent_binary_fails { # @test
  run "$purse_first" validate-mcp /nonexistent/binary
  assert_failure
}

function validate_mcp_grit_passes { # @test
  local grit_bin="$result/bin/grit"
  if [[ ! -x "$grit_bin" ]]; then
    skip "grit binary not in result"
  fi
  run "$purse_first" validate-mcp "$grit_bin"
  assert_success
  assert_output --partial "tools/list"
  assert_output --partial "ok:"
}

function validate_type_mcp_grit_passes { # @test
  local grit_bin="$result/bin/grit"
  if [[ ! -x "$grit_bin" ]]; then
    skip "grit binary not in result"
  fi
  run "$purse_first" validate --type mcp "$grit_bin"
  assert_success
  assert_output --partial "tools/list"
}

function validate_mcp_shows_tool_count { # @test
  local grit_bin="$result/bin/grit"
  if [[ ! -x "$grit_bin" ]]; then
    skip "grit binary not in result"
  fi
  run "$purse_first" validate-mcp "$grit_bin"
  assert_success
  # grit has 23 tools
  assert_output --partial "tools"
}
```

**Step 2: Run the tests**

Run: `nix build && PURSE_FIRST_BIN=result-cli/bin/purse-first nix develop --command bats --tap zz-tests_bats/validate_mcp.bats`
Expected: all tests pass (or skip if grit not available)

**Step 3: Add to justfile**

Add a new recipe and include in `test-integration`:

```just
# Run MCP validation tests
test-validate-mcp: build-purse-first-cli
    nix build
    PURSE_FIRST_BIN={{justfile_directory()}}/result-cli/bin/purse-first {{cmd_nix_dev}} bats --tap zz-tests_bats/validate_mcp.bats
```

Update the `test-integration` recipe to include `validate_mcp.bats`.

**Step 4: Commit**

```
add BATS integration tests for validate-mcp
```

---

### Task 4: Verify against multiple MCP servers

**Files:** None (manual verification step)

**Step 1: Run validate-mcp against each available server in result/**

```bash
for bin in result/bin/{grit,get-hubbed,lux,chix}; do
  echo "=== $bin ==="
  result-cli/bin/purse-first validate-mcp "$bin"
  echo
done
```

**Step 2: Review output**

Verify:
- All servers pass initialization
- tools/list returns tools with annotations
- resources/list and resources/templates/list either succeed or report "method not found" (not error)

**Step 3: Fix any issues found**

If any server fails unexpectedly, investigate and fix before completing.

**Step 4: Commit any fixes**

Only if needed.
