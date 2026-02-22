package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type FormatterMode string

const (
	FormatterModeStdin    FormatterMode = "stdin"
	FormatterModeFilepath FormatterMode = "filepath"
)

type FormatterConfig struct {
	Formatters []Formatter `toml:"formatter"`
}

type Formatter struct {
	Name       string            `toml:"name"`
	Flake      string            `toml:"flake"`
	Binary     string            `toml:"binary,omitempty"`
	Path       string            `toml:"path"`
	Args       []string          `toml:"args"`
	Env        map[string]string `toml:"env"`
	Mode       FormatterMode     `toml:"mode"`
	Disabled   bool              `toml:"disabled"`
}

func FormatterConfigPath() string {
	return filepath.Join(configDir(), "formatters.toml")
}

func LocalFormatterConfigPath() string {
	return filepath.Join(".lux", "formatters.toml")
}

func LoadFormatters() (*FormatterConfig, error) {
	return loadFormatterFile(FormatterConfigPath())
}

func LoadLocalFormatters() (*FormatterConfig, error) {
	return loadFormatterFile(LocalFormatterConfigPath())
}

func loadFormatterFile(path string) (*FormatterConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FormatterConfig{}, nil
		}
		return nil, fmt.Errorf("reading formatter config %s: %w", path, err)
	}

	var cfg FormatterConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing formatter config %s: %w", path, err)
	}

	return &cfg, nil
}

func LoadMergedFormatters() (*FormatterConfig, error) {
	global, err := LoadFormatters()
	if err != nil {
		return nil, fmt.Errorf("loading global formatters: %w", err)
	}

	local, err := LoadLocalFormatters()
	if err != nil {
		return nil, fmt.Errorf("loading local formatters: %w", err)
	}

	return MergeFormatters(global, local), nil
}

func MergeFormatters(global, local *FormatterConfig) *FormatterConfig {
	localByName := make(map[string]*Formatter, len(local.Formatters))
	for i := range local.Formatters {
		localByName[local.Formatters[i].Name] = &local.Formatters[i]
	}

	merged := &FormatterConfig{}

	for _, gf := range global.Formatters {
		if lf, ok := localByName[gf.Name]; ok {
			if !lf.Disabled {
				merged.Formatters = append(merged.Formatters, *lf)
			}
			delete(localByName, gf.Name)
		} else {
			merged.Formatters = append(merged.Formatters, gf)
		}
	}

	// Add local-only formatters that aren't disabled
	for _, lf := range local.Formatters {
		if _, wasGlobal := localByName[lf.Name]; wasGlobal && !lf.Disabled {
			merged.Formatters = append(merged.Formatters, lf)
		}
	}

	return merged
}

func (cfg *FormatterConfig) Validate() error {
	names := make(map[string]bool)
	for i, f := range cfg.Formatters {
		if f.Name == "" {
			return fmt.Errorf("formatter[%d]: name is required", i)
		}
		if names[f.Name] {
			return fmt.Errorf("formatter[%d]: duplicate name %q", i, f.Name)
		}
		names[f.Name] = true

		if f.Flake == "" && f.Path == "" {
			return fmt.Errorf("formatter[%d] (%s): flake or path is required", i, f.Name)
		}
		if f.Flake != "" && f.Path != "" {
			return fmt.Errorf("formatter[%d] (%s): flake and path are mutually exclusive", i, f.Name)
		}

		if f.Mode != "" && f.Mode != FormatterModeStdin && f.Mode != FormatterModeFilepath {
			return fmt.Errorf("formatter[%d] (%s): invalid mode %q (must be %q or %q)", i, f.Name, f.Mode, FormatterModeStdin, FormatterModeFilepath)
		}
	}
	return nil
}

func (f *Formatter) EffectiveMode() FormatterMode {
	if f.Mode == "" {
		return FormatterModeStdin
	}
	return f.Mode
}

func AddFormatterTo(path string, f Formatter) error {
	cfg, err := loadFormatterFile(path)
	if err != nil {
		return err
	}

	for i, existing := range cfg.Formatters {
		if existing.Name == f.Name {
			cfg.Formatters[i] = f
			return saveFormatterFile(path, cfg)
		}
	}

	cfg.Formatters = append(cfg.Formatters, f)
	return saveFormatterFile(path, cfg)
}

func saveFormatterFile(path string, cfg *FormatterConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating formatter config directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating formatter config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("encoding formatter config: %w", err)
	}

	return nil
}

func ExpandEnvVars(path string) string {
	return os.ExpandEnv(path)
}
