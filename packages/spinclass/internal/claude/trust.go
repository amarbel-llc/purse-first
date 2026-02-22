package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// TrustWorkspace ensures absPath is trusted in the Claude Code config file at
// claudeJSONPath (~/.claude.json). It sets
// projects.<absPath>.hasTrustDialogAccepted to true, preserving all other keys.
// The file is written atomically via a temp file + rename.
func TrustWorkspace(claudeJSONPath, absPath string) error {
	var doc map[string]any
	if data, err := os.ReadFile(claudeJSONPath); err == nil {
		json.Unmarshal(data, &doc)
	}
	if doc == nil {
		doc = make(map[string]any)
	}

	projects, _ := doc["projects"].(map[string]any)
	if projects == nil {
		projects = make(map[string]any)
	}

	entry, _ := projects[absPath].(map[string]any)
	if entry == nil {
		entry = make(map[string]any)
	}

	entry["hasTrustDialogAccepted"] = true
	projects[absPath] = entry
	doc["projects"] = projects

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(claudeJSONPath), 0o755); err != nil {
		return err
	}

	tmp := fmt.Sprintf("%s.tmp.%d", claudeJSONPath, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		os.Remove(tmp)
		return err
	}

	if err := os.Rename(tmp, claudeJSONPath); err != nil {
		os.Remove(tmp)
		return err
	}

	return nil
}
