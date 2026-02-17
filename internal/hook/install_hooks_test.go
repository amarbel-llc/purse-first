package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSetsBlockingTrueForPreToolUse(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	settingsDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Install("/usr/bin/purse-first", false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	preToolUse, _ := hooks["PreToolUse"].([]any)
	if len(preToolUse) == 0 {
		t.Fatal("expected PreToolUse hook entries, got none")
	}

	entry, _ := preToolUse[0].(map[string]any)
	hooksList, _ := entry["hooks"].([]any)
	if len(hooksList) == 0 {
		t.Fatal("expected hook entries, got none")
	}

	hookEntry, _ := hooksList[0].(map[string]any)

	blocking, ok := hookEntry["blocking"]
	if !ok {
		t.Fatal("expected blocking field to be present in PreToolUse hook")
	}

	if blocking != true {
		t.Errorf("expected PreToolUse blocking=true, got %v", blocking)
	}
}

func TestInstallSetsBlockingFalseForPostToolUse(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	settingsDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Install("/usr/bin/purse-first", false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	postToolUse, _ := hooks["PostToolUse"].([]any)
	if len(postToolUse) == 0 {
		t.Fatal("expected PostToolUse hook entries, got none")
	}

	entry, _ := postToolUse[0].(map[string]any)
	hooksList, _ := entry["hooks"].([]any)
	if len(hooksList) == 0 {
		t.Fatal("expected hook entries, got none")
	}

	hookEntry, _ := hooksList[0].(map[string]any)

	blocking, ok := hookEntry["blocking"]
	if !ok {
		t.Fatal("expected blocking field to be present in PostToolUse hook")
	}

	if blocking != false {
		t.Errorf("expected PostToolUse blocking=false, got %v", blocking)
	}
}
