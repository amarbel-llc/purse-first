package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Socket string `toml:"socket"`
	LSPs   []LSP  `toml:"lsp"`
}

type LSP struct {
	Name         string              `toml:"name"`
	Flake        string              `toml:"flake"`
	Binary       string              `toml:"binary,omitempty"`
	Args         []string            `toml:"args"`
	Env          map[string]string   `toml:"env,omitempty"`
	InitOptions  map[string]any      `toml:"init_options,omitempty"`
	Settings     map[string]any      `toml:"settings,omitempty"`
	SettingsKey  string              `toml:"settings_key,omitempty"`
	Capabilities    *CapabilityOverride `toml:"capabilities,omitempty"`
	WaitForReady    *bool               `toml:"wait_for_ready,omitempty"`
	ReadyTimeout    string              `toml:"ready_timeout,omitempty"`
	ActivityTimeout string              `toml:"activity_timeout,omitempty"`
	EagerStart      *bool               `toml:"eager_start,omitempty"`
}

type CapabilityOverride struct {
	Disable []string `toml:"disable,omitempty"`
	Enable  []string `toml:"enable,omitempty"`
}

func ConfigDir() string {
	return configDir()
}

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lux")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "lux")
	}
	return filepath.Join(home, ".config", "lux")
}

func dataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "lux")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".local", "share", "lux")
	}
	return filepath.Join(home, ".local", "share", "lux")
}

func runtimeDir() string {
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return xdg
	}
	return os.TempDir()
}

func ConfigPath() string {
	return filepath.Join(configDir(), "lsps.toml")
}

func DataDir() string {
	return dataDir()
}

func CapabilitiesDir() string {
	return filepath.Join(dataDir(), "capabilities")
}

func (c *Config) SocketPath() string {
	if c.Socket != "" {
		return c.Socket
	}
	return filepath.Join(runtimeDir(), "lux.sock")
}

func Load() (*Config, error) {
	return LoadFrom(ConfigPath())
}

func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	names := make(map[string]bool)
	for i, lsp := range c.LSPs {
		if lsp.Name == "" {
			return fmt.Errorf("lsp[%d]: name is required", i)
		}
		if lsp.Flake == "" {
			return fmt.Errorf("lsp[%d] (%s): flake is required", i, lsp.Name)
		}
		if names[lsp.Name] {
			return fmt.Errorf("lsp[%d]: duplicate name %q", i, lsp.Name)
		}
		names[lsp.Name] = true

		// Validate environment variable names
		for k := range lsp.Env {
			if !isValidEnvVarName(k) {
				return fmt.Errorf("lsp[%d] (%s): invalid environment variable name %q", i, lsp.Name, k)
			}
		}

		// Validate init_options can be marshaled to JSON
		if len(lsp.InitOptions) > 0 {
			if _, err := json.Marshal(lsp.InitOptions); err != nil {
				return fmt.Errorf("lsp[%d] (%s): invalid init_options: %w", i, lsp.Name, err)
			}
		}

		// Validate settings can be marshaled to JSON
		if len(lsp.Settings) > 0 {
			if _, err := json.Marshal(lsp.Settings); err != nil {
				return fmt.Errorf("lsp[%d] (%s): invalid settings: %w", i, lsp.Name, err)
			}
		}

		// Validate capability names (warn only, don't error)
		if lsp.Capabilities != nil {
			for _, name := range append(lsp.Capabilities.Disable, lsp.Capabilities.Enable...) {
				if !isKnownCapability(name) {
					fmt.Fprintf(os.Stderr, "warning: unknown capability %q in lsp %s\n", name, lsp.Name)
				}
			}
		}
	}
	return nil
}

func (l *LSP) SettingsWireKey() string {
	if l.SettingsKey != "" {
		return l.SettingsKey
	}
	return l.Name
}

func (l *LSP) ShouldWaitForReady() bool {
	if l.WaitForReady == nil {
		return true
	}
	return *l.WaitForReady
}

func (l *LSP) ReadyTimeoutDuration() time.Duration {
	if l.ReadyTimeout == "" {
		return 10 * time.Minute
	}
	d, err := time.ParseDuration(l.ReadyTimeout)
	if err != nil {
		return 10 * time.Minute
	}
	return d
}

func (l *LSP) ShouldEagerStart() bool {
	if l.EagerStart == nil {
		return false
	}
	return *l.EagerStart
}

func (l *LSP) ActivityTimeoutDuration() time.Duration {
	if l.ActivityTimeout == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(l.ActivityTimeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

func (c *Config) FindLSP(name string) *LSP {
	for i := range c.LSPs {
		if c.LSPs[i].Name == name {
			return &c.LSPs[i]
		}
	}
	return nil
}

func Save(cfg *Config) error {
	return SaveTo(ConfigPath(), cfg)
}

func SaveTo(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	return nil
}

func AddLSP(lsp LSP) error {
	return AddLSPTo(ConfigPath(), lsp)
}

func AddLSPTo(path string, lsp LSP) error {
	cfg, err := LoadFrom(path)
	if err != nil {
		return err
	}

	for i, existing := range cfg.LSPs {
		if existing.Name == lsp.Name {
			cfg.LSPs[i] = lsp
			return SaveTo(path, cfg)
		}
	}

	cfg.LSPs = append(cfg.LSPs, lsp)
	return SaveTo(path, cfg)
}

func isValidEnvVarName(name string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	return matched
}

var knownCapabilities = map[string]bool{
	"hover":                       true,
	"hoverProvider":               true,
	"completion":                  true,
	"completionProvider":          true,
	"definition":                  true,
	"definitionProvider":          true,
	"typeDefinition":              true,
	"typeDefinitionProvider":      true,
	"implementation":              true,
	"implementationProvider":      true,
	"references":                  true,
	"referencesProvider":          true,
	"documentHighlight":           true,
	"documentHighlightProvider":   true,
	"documentSymbol":              true,
	"documentSymbolProvider":      true,
	"codeAction":                  true,
	"codeActionProvider":          true,
	"codeLens":                    true,
	"codeLensProvider":            true,
	"documentFormatting":          true,
	"documentFormattingProvider":  true,
	"documentRangeFormatting":     true,
	"documentRangeFormattingProvider": true,
	"rename":                      true,
	"renameProvider":              true,
	"foldingRange":                true,
	"foldingRangeProvider":        true,
	"selectionRange":              true,
	"selectionRangeProvider":      true,
	"semanticTokens":              true,
	"semanticTokensProvider":      true,
	"inlayHint":                   true,
	"inlayHintProvider":           true,
	"diagnostic":                  true,
	"diagnosticProvider":          true,
	"workspaceSymbol":             true,
	"workspaceSymbolProvider":     true,
}

func isKnownCapability(name string) bool {
	return knownCapabilities[name]
}
