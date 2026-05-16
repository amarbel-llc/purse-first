package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type preToolUseEntry struct {
	Matcher string      `json:"matcher"`
	Hooks   []hookEntry `json:"hooks"`
}

type hooksManifest struct {
	Hooks struct {
		PreToolUse []preToolUseEntry `json:"PreToolUse"`
	} `json:"hooks"`
}

// GenerateHooks writes hooks/hooks.json and hooks/pre-tool-use to
// {dir}/{u.Name}/hooks/. The hooks.json matcher is built from the unique
// set of Replaces values across all ToolMappings. If no commands have tool
// mappings, no files are written.
func (u *Utility) GenerateHooks(dir string) error {
	replacesSet := make(map[string]bool)

	for _, cmd := range u.AllCommands() {
		if cmd.Hidden {
			continue
		}
		for _, tm := range cmd.MapsTools {
			replacesSet[tm.Replaces] = true
		}
	}

	if len(replacesSet) == 0 {
		return nil
	}

	sorted := make([]string, 0, len(replacesSet))
	for r := range replacesSet {
		sorted = append(sorted, r)
	}
	sort.Strings(sorted)
	matcher := strings.Join(sorted, "|")

	hooksDir := filepath.Join(dir, u.Name, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks directory: %w", err)
	}

	var manifest hooksManifest
	manifest.Hooks.PreToolUse = []preToolUseEntry{
		{
			Matcher: matcher,
			Hooks: []hookEntry{
				{
					Type:    "command",
					Command: "${CLAUDE_PLUGIN_ROOT}/hooks/pre-tool-use",
					Timeout: 5,
				},
			},
		},
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling hooks.json: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), data, 0o644); err != nil {
		return fmt.Errorf("writing hooks.json: %w", err)
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}
	script := fmt.Sprintf("#!/bin/sh\nexec '%s' hook\n", self)
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-tool-use"), []byte(script), 0o755); err != nil {
		return fmt.Errorf("writing pre-tool-use: %w", err)
	}

	return nil
}
