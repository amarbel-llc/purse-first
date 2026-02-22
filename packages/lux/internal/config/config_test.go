package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

func TestLSP_BinaryField_TOML(t *testing.T) {
	tests := []struct {
		name     string
		toml     string
		expected LSP
	}{
		{
			name: "with binary field",
			toml: `
name = "test"
flake = "nixpkgs#gopls"
binary = "gopls"
`,
			expected: LSP{
				Name:   "test",
				Flake:  "nixpkgs#gopls",
				Binary: "gopls",
			},
		},
		{
			name: "without binary field",
			toml: `
name = "test"
flake = "nixpkgs#gopls"
`,
			expected: LSP{
				Name:   "test",
				Flake:  "nixpkgs#gopls",
				Binary: "",
			},
		},
		{
			name: "with binary as relative path",
			toml: `
name = "test"
flake = "github:owner/repo"
binary = "bin/custom-lsp"
`,
			expected: LSP{
				Name:   "test",
				Flake:  "github:owner/repo",
				Binary: "bin/custom-lsp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lsp LSP
			if err := toml.Unmarshal([]byte(tt.toml), &lsp); err != nil {
				t.Fatalf("failed to parse TOML: %v", err)
			}

			if lsp.Name != tt.expected.Name {
				t.Errorf("Name: expected %q, got %q", tt.expected.Name, lsp.Name)
			}
			if lsp.Flake != tt.expected.Flake {
				t.Errorf("Flake: expected %q, got %q", tt.expected.Flake, lsp.Flake)
			}
			if lsp.Binary != tt.expected.Binary {
				t.Errorf("Binary: expected %q, got %q", tt.expected.Binary, lsp.Binary)
			}
		})
	}
}

func TestConfig_BinaryField_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "lsps.toml")

	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	originalConfig := &Config{
		Socket: "/tmp/test.sock",
		LSPs: []LSP{
			{
				Name:   "gopls",
				Flake:  "nixpkgs#gopls",
				Binary: "gopls",
			},
			{
				Name:   "custom",
				Flake:  "github:owner/repo",
				Binary: "bin/custom-lsp",
			},
			{
				Name:  "default",
				Flake: "nixpkgs#rust-analyzer",
			},
		},
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	if err := Save(originalConfig); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loadedConfig, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(loadedConfig.LSPs) != len(originalConfig.LSPs) {
		t.Fatalf("expected %d LSPs, got %d", len(originalConfig.LSPs), len(loadedConfig.LSPs))
	}

	for i, expectedLSP := range originalConfig.LSPs {
		gotLSP := loadedConfig.LSPs[i]

		if gotLSP.Name != expectedLSP.Name {
			t.Errorf("LSP[%d] Name: expected %q, got %q", i, expectedLSP.Name, gotLSP.Name)
		}
		if gotLSP.Flake != expectedLSP.Flake {
			t.Errorf("LSP[%d] Flake: expected %q, got %q", i, expectedLSP.Flake, gotLSP.Flake)
		}
		if gotLSP.Binary != expectedLSP.Binary {
			t.Errorf("LSP[%d] Binary: expected %q, got %q", i, expectedLSP.Binary, gotLSP.Binary)
		}
	}
}

func TestConfig_BinaryOmitempty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "lux", "lsps.toml")

	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	config := &Config{
		LSPs: []LSP{
			{
				Name:  "test",
				Flake: "nixpkgs#gopls",
			},
		},
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	if err := Save(config); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	content := string(data)
	if contains(content, "binary") {
		t.Error("expected binary field to be omitted when empty, but it was present in TOML")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

func TestAddLSP_WithBinary(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	if err := os.MkdirAll(filepath.Join(tmpDir, "lux"), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	lsp := LSP{
		Name:   "test-lsp",
		Flake:  "nixpkgs#test",
		Binary: "custom-binary",
	}

	if err := AddLSP(lsp); err != nil {
		t.Fatalf("failed to add LSP: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.LSPs) != 1 {
		t.Fatalf("expected 1 LSP, got %d", len(cfg.LSPs))
	}

	if cfg.LSPs[0].Binary != "custom-binary" {
		t.Errorf("expected binary %q, got %q", "custom-binary", cfg.LSPs[0].Binary)
	}
}

func TestLSP_SettingsField_TOML(t *testing.T) {
	input := `
name = "gopls"
flake = "nixpkgs#gopls"

[settings]
  gofumpt = true
  staticcheck = true

  [settings.analyses]
    unusedparams = true
    shadow = true
    ST1000 = false
`
	var lsp LSP
	if err := toml.Unmarshal([]byte(input), &lsp); err != nil {
		t.Fatalf("failed to parse TOML: %v", err)
	}

	if lsp.Settings == nil {
		t.Fatal("expected Settings to be non-nil")
	}

	if gofumpt, ok := lsp.Settings["gofumpt"].(bool); !ok || !gofumpt {
		t.Errorf("expected gofumpt=true, got %v", lsp.Settings["gofumpt"])
	}

	analyses, ok := lsp.Settings["analyses"].(map[string]any)
	if !ok {
		t.Fatal("expected analyses to be a map")
	}

	if shadow, ok := analyses["shadow"].(bool); !ok || !shadow {
		t.Errorf("expected analyses.shadow=true, got %v", analyses["shadow"])
	}

	if st1000, ok := analyses["ST1000"].(bool); !ok || st1000 {
		t.Errorf("expected analyses.ST1000=false, got %v", analyses["ST1000"])
	}
}

func TestLSP_SettingsValidation(t *testing.T) {
	cfg := &Config{
		LSPs: []LSP{
			{
				Name:  "test",
				Flake: "nixpkgs#test",
				Settings: map[string]any{
					"valid_key": "valid_value",
					"nested": map[string]any{
						"deep": true,
					},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid settings to pass validation, got: %v", err)
	}
}

func TestLSP_SettingsWireKey(t *testing.T) {
	tests := []struct {
		name        string
		lspName     string
		settingsKey string
		expected    string
	}{
		{"defaults to name", "gopls", "", "gopls"},
		{"uses explicit key", "gopls", "custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lsp := &LSP{Name: tt.lspName, SettingsKey: tt.settingsKey}
			if got := lsp.SettingsWireKey(); got != tt.expected {
				t.Errorf("SettingsWireKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLSP_SettingsDeepMerge(t *testing.T) {
	global := LSP{
		Name:  "gopls",
		Flake: "nixpkgs#gopls",
		Settings: map[string]any{
			"gofumpt":     true,
			"staticcheck": true,
			"analyses": map[string]any{
				"unusedparams": true,
				"shadow":       true,
			},
		},
	}

	project := LSP{
		Name:  "gopls",
		Flake: "nixpkgs#gopls",
		Settings: map[string]any{
			"staticcheck": false,
			"analyses": map[string]any{
				"shadow": false,
				"ST1000": false,
			},
		},
	}

	result := mergeLSP(global, project)

	if result.Settings == nil {
		t.Fatal("expected merged Settings to be non-nil")
	}

	// gofumpt should be preserved from global
	if gofumpt, ok := result.Settings["gofumpt"].(bool); !ok || !gofumpt {
		t.Errorf("expected gofumpt=true (from global), got %v", result.Settings["gofumpt"])
	}

	// staticcheck should be overridden by project
	if sc, ok := result.Settings["staticcheck"].(bool); !ok || sc {
		t.Errorf("expected staticcheck=false (from project), got %v", result.Settings["staticcheck"])
	}

	// analyses should be deep merged
	analyses, ok := result.Settings["analyses"].(map[string]any)
	if !ok {
		t.Fatal("expected analyses to be a map")
	}

	// unusedparams from global should be preserved
	if up, ok := analyses["unusedparams"].(bool); !ok || !up {
		t.Errorf("expected analyses.unusedparams=true (from global), got %v", analyses["unusedparams"])
	}

	// shadow should be overridden by project
	if shadow, ok := analyses["shadow"].(bool); !ok || shadow {
		t.Errorf("expected analyses.shadow=false (from project), got %v", analyses["shadow"])
	}

	// ST1000 should be added from project
	if st, ok := analyses["ST1000"].(bool); !ok || st {
		t.Errorf("expected analyses.ST1000=false (from project), got %v", analyses["ST1000"])
	}
}

func TestAddLSP_UpdateWithBinary(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	if err := os.MkdirAll(filepath.Join(tmpDir, "lux"), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	firstLSP := LSP{
		Name:  "test-lsp",
		Flake: "nixpkgs#test",
	}

	if err := AddLSP(firstLSP); err != nil {
		t.Fatalf("failed to add first LSP: %v", err)
	}

	updatedLSP := LSP{
		Name:   "test-lsp",
		Flake:  "nixpkgs#test-v2",
		Binary: "custom-binary",
	}

	if err := AddLSP(updatedLSP); err != nil {
		t.Fatalf("failed to update LSP: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.LSPs) != 1 {
		t.Fatalf("expected 1 LSP, got %d", len(cfg.LSPs))
	}

	if cfg.LSPs[0].Binary != "custom-binary" {
		t.Errorf("expected binary %q, got %q", "custom-binary", cfg.LSPs[0].Binary)
	}
	if cfg.LSPs[0].Flake != "nixpkgs#test-v2" {
		t.Errorf("expected flake %q, got %q", "nixpkgs#test-v2", cfg.LSPs[0].Flake)
	}
}

func TestLSP_ReadinessFields_TOML(t *testing.T) {
	input := `
name = "gopls"
flake = "nixpkgs#gopls"
wait_for_ready = false
ready_timeout = "5m"
activity_timeout = "15s"
`
	var lsp LSP
	if err := toml.Unmarshal([]byte(input), &lsp); err != nil {
		t.Fatalf("failed to parse TOML: %v", err)
	}

	if lsp.WaitForReady == nil || *lsp.WaitForReady != false {
		t.Errorf("expected WaitForReady=false, got %v", lsp.WaitForReady)
	}
	if lsp.ReadyTimeout != "5m" {
		t.Errorf("expected ReadyTimeout=5m, got %q", lsp.ReadyTimeout)
	}
	if lsp.ActivityTimeout != "15s" {
		t.Errorf("expected ActivityTimeout=15s, got %q", lsp.ActivityTimeout)
	}
}

func TestLSP_ReadinessFields_Defaults(t *testing.T) {
	input := `
name = "gopls"
flake = "nixpkgs#gopls"
`
	var lsp LSP
	if err := toml.Unmarshal([]byte(input), &lsp); err != nil {
		t.Fatalf("failed to parse TOML: %v", err)
	}

	if lsp.WaitForReady != nil {
		t.Errorf("expected WaitForReady=nil (default), got %v", *lsp.WaitForReady)
	}

	readyTimeout := lsp.ReadyTimeoutDuration()
	if readyTimeout != 10*time.Minute {
		t.Errorf("expected default ReadyTimeout=10m, got %v", readyTimeout)
	}

	activityTimeout := lsp.ActivityTimeoutDuration()
	if activityTimeout != 30*time.Second {
		t.Errorf("expected default ActivityTimeout=30s, got %v", activityTimeout)
	}
}

func TestLSP_ReadinessFields_InvalidDuration(t *testing.T) {
	input := `
name = "gopls"
flake = "nixpkgs#gopls"
ready_timeout = "not-a-duration"
`
	var lsp LSP
	if err := toml.Unmarshal([]byte(input), &lsp); err != nil {
		t.Fatalf("failed to parse TOML: %v", err)
	}

	readyTimeout := lsp.ReadyTimeoutDuration()
	if readyTimeout != 10*time.Minute {
		t.Errorf("expected fallback ReadyTimeout=10m, got %v", readyTimeout)
	}
}

func TestLSP_ShouldWaitForReady(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name     string
		lsp      LSP
		expected bool
	}{
		{"nil defaults to true", LSP{WaitForReady: nil}, true},
		{"explicit true", LSP{WaitForReady: &trueVal}, true},
		{"explicit false", LSP{WaitForReady: &falseVal}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.lsp.ShouldWaitForReady(); got != tt.expected {
				t.Errorf("ShouldWaitForReady() = %v, want %v", got, tt.expected)
			}
		})
	}
}
