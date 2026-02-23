package localplugin

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// runGenerate execs "go run ./cmd/<binary> _generate <outDir>" and returns the
// path to the generated plugin.json found via glob.
func runGenerate(root, binary string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolving root: %w", err)
	}

	outDir := filepath.Join(absRoot, ".claude-plugin")

	cmd := exec.Command("go", "run", "./cmd/"+binary, "_generate", outDir)
	cmd.Dir = absRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("running _generate: %w\n%s", err, output)
	}

	return findGeneratedPlugin(outDir)
}

// findGeneratedPlugin globs for plugin.json under the _generate output tree.
func findGeneratedPlugin(outDir string) (string, error) {
	pattern := filepath.Join(outDir, "share", "purse-first", "*", "plugin.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("globbing for plugin.json: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no plugin.json found under %s", outDir)
	}

	if len(matches) > 1 {
		return "", fmt.Errorf("multiple plugin.json found under %s: %v", outDir, matches)
	}

	return matches[0], nil
}
