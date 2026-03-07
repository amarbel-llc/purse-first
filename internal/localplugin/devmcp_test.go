package localplugin

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeFakeBinary(t *testing.T, dir, name, script string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}

	return path
}

func readMCPJSON(t *testing.T, dir string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("reading .mcp.json: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parsing .mcp.json: %v", err)
	}

	return result
}

func TestInstallDevMCPWritesMCPJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on windows")
	}

	binDir := t.TempDir()
	outDir := t.TempDir()

	writeFakeBinary(t, binDir, "fake-mcp", `#!/bin/sh
cat <<'EOF'
{
  "name": "fake-mcp",
  "mcpServers": {
    "fake-mcp": {
      "type": "stdio",
      "command": "fake-mcp"
    }
  }
}
EOF
`)

	var buf bytes.Buffer
	err := InstallDevMCP(&buf, filepath.Join(binDir, "fake-mcp"), outDir)
	if err != nil {
		t.Fatalf("InstallDevMCP: %v", err)
	}

	result := readMCPJSON(t, outDir)

	mcpServers, ok := result["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers not found in .mcp.json")
	}

	if _, ok := mcpServers["fake-mcp"]; !ok {
		t.Fatal("fake-mcp server not found in .mcp.json")
	}
}

func TestInstallDevMCPRewritesCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on windows")
	}

	binDir := t.TempDir()
	outDir := t.TempDir()

	binaryPath := writeFakeBinary(t, binDir, "my-server", `#!/bin/sh
cat <<'EOF'
{
  "name": "my-server",
  "mcpServers": {
    "my-server": {
      "type": "stdio",
      "command": "my-server"
    }
  }
}
EOF
`)

	var buf bytes.Buffer
	err := InstallDevMCP(&buf, binaryPath, outDir)
	if err != nil {
		t.Fatalf("InstallDevMCP: %v", err)
	}

	result := readMCPJSON(t, outDir)
	mcpServers := result["mcpServers"].(map[string]any)
	server := mcpServers["my-server"].(map[string]any)

	absPath, _ := filepath.Abs(binaryPath)
	if server["command"] != absPath {
		t.Errorf("command = %q, want %q", server["command"], absPath)
	}
}

func TestInstallDevMCPPreservesArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on windows")
	}

	binDir := t.TempDir()
	outDir := t.TempDir()

	writeFakeBinary(t, binDir, "arg-server", `#!/bin/sh
cat <<'EOF'
{
  "name": "arg-server",
  "mcpServers": {
    "arg-server": {
      "type": "stdio",
      "command": "arg-server",
      "args": ["serve", "--port", "8080"]
    }
  }
}
EOF
`)

	var buf bytes.Buffer
	err := InstallDevMCP(&buf, filepath.Join(binDir, "arg-server"), outDir)
	if err != nil {
		t.Fatalf("InstallDevMCP: %v", err)
	}

	result := readMCPJSON(t, outDir)
	mcpServers := result["mcpServers"].(map[string]any)
	server := mcpServers["arg-server"].(map[string]any)

	args, ok := server["args"].([]any)
	if !ok {
		t.Fatal("args not found in server config")
	}

	expected := []string{"serve", "--port", "8080"}
	if len(args) != len(expected) {
		t.Fatalf("args length = %d, want %d", len(args), len(expected))
	}

	for i, want := range expected {
		if args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestInstallDevMCPFailsOnBadBinary(t *testing.T) {
	outDir := t.TempDir()

	var buf bytes.Buffer
	err := InstallDevMCP(&buf, "/nonexistent/binary", outDir)
	if err == nil {
		t.Fatal("expected error for nonexistent binary, got nil")
	}
}
