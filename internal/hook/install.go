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

type hookSpec struct {
	eventName string
	matcher   string
	command   string
	timeout   int
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

	specs := []hookSpec{
		{
			eventName: "PreToolUse",
			matcher:   "Read|Edit|Write|Grep|Glob|Bash",
			command:   binaryPath + " hook",
			timeout:   5,
		},
		{
			eventName: "PostToolUse",
			matcher:   "Read|Edit|Write",
			command:   binaryPath + " post-hook",
			timeout:   5,
		},
		{
			eventName: "Stop",
			command:   binaryPath + " session-end",
			timeout:   5,
		},
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	for _, spec := range specs {
		entry := hookMatcher{
			Matcher: spec.matcher,
			Hooks: []hookEntry{
				{
					Type:    "command",
					Command: spec.command,
					Timeout: spec.timeout,
				},
			},
		}

		upsertHook(hooks, spec.eventName, entry, binaryPath)
	}

	settings["hooks"] = hooks

	return writeSettings(settingsPath, settings)
}

func upsertHook(hooks map[string]any, eventName string, entry hookMatcher, binaryPath string) {
	entryJSON, _ := json.Marshal(entry)
	var entryMap map[string]any
	json.Unmarshal(entryJSON, &entryMap)

	existing, _ := hooks[eventName].([]any)

	found := false
	for i, e := range existing {
		eMap, ok := e.(map[string]any)
		if !ok {
			continue
		}

		eHooks, _ := eMap["hooks"].([]any)
		for _, h := range eHooks {
			hMap, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := hMap["command"].(string)
			if cmd != "" && strings.HasPrefix(cmd, binaryPath) {
				existing[i] = entryMap
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		existing = append(existing, entryMap)
	}

	hooks[eventName] = existing
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
