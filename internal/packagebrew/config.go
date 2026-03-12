package packagebrew

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Name             string                   `json:"name"`
	Description      string                   `json:"description,omitempty"`
	Owner            Owner                    `json:"owner"`
	ReleaseRepo      string                   `json:"releaseRepo"`
	TapName          string                   `json:"tapName"`
	License          string                   `json:"license"`
	Private          bool                     `json:"private,omitempty"`
	DownloadStrategy string                   `json:"downloadStrategy,omitempty"`
	Packages         map[string]PackageConfig `json:"packages"`
}

type Owner struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type PackageConfig struct {
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Binary      bool              `json:"binary"`
	Homepage    string            `json:"homepage,omitempty"`
	Category    string            `json:"category,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Platforms   map[string]string `json:"platforms,omitempty"`
	Share       string            `json:"share"`
	BrewDeps    []string          `json:"brewDeps,omitempty"`
}

func ReadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading brew config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing brew config: %w", err)
	}

	return cfg, nil
}
