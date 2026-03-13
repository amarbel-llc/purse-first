package localplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// InstallDevMCP runs `<binaryPath> generate-plugin -` to get a plugin manifest
// from stdout, rewrites the mcpServers command fields to the absolute binary
// path, and writes the result as .mcp.json in outputDir.
func InstallDevMCP(w io.Writer, binaryPath, outputDir string) error {
	tw := newTAPWriter(w)
	tw.PlanAhead(3)

	// 1. Resolve binary to absolute path and verify it exists
	absPath, err := filepath.Abs(binaryPath)
	if err != nil {
		tw.NotOk("resolve binary path", map[string]string{
			"error": err.Error(),
		})
		return fmt.Errorf("resolving binary path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		tw.NotOk("resolve binary path", map[string]string{
			"error": err.Error(),
			"path":  absPath,
		})
		return fmt.Errorf("binary not found: %w", err)
	}

	if info.IsDir() {
		tw.NotOk("resolve binary path", map[string]string{
			"error": "path is a directory, not a file",
			"path":  absPath,
		})
		return fmt.Errorf("binary path is a directory: %s", absPath)
	}

	if info.Mode()&0o111 == 0 {
		tw.NotOk("resolve binary path", map[string]string{
			"error": "file is not executable",
			"path":  absPath,
		})
		return fmt.Errorf("binary is not executable: %s", absPath)
	}

	tw.Ok("resolve binary path")

	// 2. Run `<binary> generate-plugin -` and capture stdout
	var stdout bytes.Buffer
	cmd := exec.Command(absPath, "generate-plugin", "-")
	cmd.Stdout = &stdout
	cmd.Stderr = w

	if err := cmd.Run(); err != nil {
		tw.NotOk("run generate-plugin", map[string]string{
			"error":   err.Error(),
			"command": absPath + " generate-plugin -",
		})
		return fmt.Errorf("running generate-plugin: %w", err)
	}

	var plugin map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &plugin); err != nil {
		tw.NotOk("run generate-plugin", map[string]string{
			"error":  err.Error(),
			"output": stdout.String(),
		})
		return fmt.Errorf("parsing plugin JSON: %w", err)
	}

	tw.Ok("run generate-plugin")

	// 3. Extract mcpServers, rewrite command fields, write .mcp.json
	mcpServers, ok := plugin["mcpServers"].(map[string]any)
	if !ok || len(mcpServers) == 0 {
		tw.NotOk("write .mcp.json", map[string]string{
			"error": "no mcpServers found in plugin manifest",
		})
		return fmt.Errorf("no mcpServers found in plugin manifest")
	}

	for name, serverRaw := range mcpServers {
		serverMap, ok := serverRaw.(map[string]any)
		if !ok {
			continue
		}
		serverMap["command"] = absPath
		mcpServers[name] = serverMap
	}

	mcpJSON := map[string]any{
		"mcpServers": mcpServers,
	}

	data, err := json.MarshalIndent(mcpJSON, "", "  ")
	if err != nil {
		tw.NotOk("write .mcp.json", map[string]string{
			"error": err.Error(),
		})
		return fmt.Errorf("marshaling .mcp.json: %w", err)
	}
	data = append(data, '\n')

	mcpPath := filepath.Join(outputDir, ".mcp.json")
	if err := os.WriteFile(mcpPath, data, 0o644); err != nil {
		tw.NotOk("write .mcp.json", map[string]string{
			"error": err.Error(),
			"path":  mcpPath,
		})
		return fmt.Errorf("writing .mcp.json: %w", err)
	}

	count := len(mcpServers)
	tw.Ok(fmt.Sprintf("write .mcp.json (%d server%s)", count, plural(count)))

	return nil
}
