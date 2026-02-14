package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Mode        string `json:"mode"`
	FallbackTTL int    `json:"fallback_ttl"`
}

func DefaultConfig() Config {
	return Config{
		Mode:        "prefer",
		FallbackTTL: 60,
	}
}

func xdgConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func ConfigPath() string {
	return filepath.Join(xdgConfigHome(), "purse-first", "config.toml")
}
