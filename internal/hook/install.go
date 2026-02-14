package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type hookMatcher struct {
	Matcher string      `json:"matcher"`
	Hooks   []hookEntry `json:"hooks"`
}

func Install(binaryPath string, project bool) error {
	settingsPath, err := settingsFilePath(project)
	if err != nil {
		return err
	}

	settings, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	entry := hookMatcher{
		Matcher: "Read|Edit|Write|Grep|Glob|Bash",
		Hooks: []hookEntry{
			{
				Type:    "command",
				Command: binaryPath + " hook",
				Timeout: 5,
			},
		},
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	preToolUse, _ := hooks["PreToolUse"].([]any)

	// Check if purse-first hook already exists
	entryJSON, _ := json.Marshal(entry)
	var entryMap map[string]any
	json.Unmarshal(entryJSON, &entryMap)

	found := false
	for i, existing := range preToolUse {
		existingMap, ok := existing.(map[string]any)
		if !ok {
			continue
		}

		existingHooks, _ := existingMap["hooks"].([]any)
		for _, h := range existingHooks {
			hMap, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hMap["command"].(string)
			if len(cmd) > 0 && cmd == binaryPath+" hook" {
				// Update existing entry
				preToolUse[i] = entryMap
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		preToolUse = append(preToolUse, entryMap)
	}

	hooks["PreToolUse"] = preToolUse
	settings["hooks"] = hooks

	return writeSettings(settingsPath, settings)
}

func settingsFilePath(project bool) (string, error) {
	if project {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getting working directory: %w", err)
		}
		return filepath.Join(cwd, ".claude", "settings.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func readSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return settings, nil
}

func writeSettings(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", path, err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
