package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type hookMatcher struct {
	Matcher string      `json:"matcher,omitempty"`
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

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	toolMatcher := "Read|Edit|Write|Grep|Glob|Bash"

	hookDefs := []struct {
		event   string
		entry   hookMatcher
	}{
		{
			event: "PreToolUse",
			entry: hookMatcher{
				Matcher: toolMatcher,
				Hooks: []hookEntry{{
					Type:    "command",
					Command: binaryPath + " hook",
					Timeout: 5,
				}},
			},
		},
		{
			event: "PostToolUse",
			entry: hookMatcher{
				Matcher: toolMatcher,
				Hooks: []hookEntry{{
					Type:    "command",
					Command: binaryPath + " post-hook",
					Timeout: 5,
				}},
			},
		},
		{
			event: "Stop",
			entry: hookMatcher{
				Hooks: []hookEntry{{
					Type:    "command",
					Command: binaryPath + " session-end",
					Timeout: 5,
				}},
			},
		},
	}

	for _, def := range hookDefs {
		existing, _ := hooks[def.event].([]any)

		entryJSON, _ := json.Marshal(def.entry)
		var entryMap map[string]any
		json.Unmarshal(entryJSON, &entryMap)

		filtered := removePurseFirstEntries(existing)
		hooks[def.event] = append(filtered, entryMap)
	}

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

func removePurseFirstEntries(entries []any) []any {
	var filtered []any
	for _, existing := range entries {
		existingMap, ok := existing.(map[string]any)
		if !ok {
			filtered = append(filtered, existing)
			continue
		}

		if isPurseFirstEntry(existingMap) {
			continue
		}

		filtered = append(filtered, existing)
	}
	return filtered
}

func isPurseFirstEntry(entry map[string]any) bool {
	hooks, _ := entry["hooks"].([]any)
	for _, h := range hooks {
		hMap, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hMap["command"].(string)
		if strings.Contains(cmd, "purse-first") {
			return true
		}
	}
	return false
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
