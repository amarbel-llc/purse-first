package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// LoadWithProject loads global config and merges with project-level config
func LoadWithProject(projectRoot string) (*Config, error) {
	// Load global config
	globalCfg, err := Load()
	if err != nil {
		return nil, err
	}

	// Try to load project config
	projectCfg, err := loadProjectConfig(projectRoot)
	if err != nil {
		// No project config or error loading - use global only
		return globalCfg, nil
	}

	// Merge project over global
	merged := mergeConfigs(globalCfg, projectCfg)
	return merged, nil
}

func loadProjectConfig(projectRoot string) (*Config, error) {
	configPath := ProjectConfigPath(projectRoot)
	if configPath == "" {
		return nil, fmt.Errorf("no project config found")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading project config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing project config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating project config: %w", err)
	}

	return &cfg, nil
}

// mergeConfigs merges project config over global config
// Strategy: LSPs by name are deeply merged, new LSPs are added
func mergeConfigs(global, project *Config) *Config {
	merged := &Config{
		Socket: global.Socket,
		LSPs:   make([]LSP, 0, len(global.LSPs)+len(project.LSPs)),
	}

	// Use project socket if specified
	if project.Socket != "" {
		merged.Socket = project.Socket
	}

	// Build map of project LSPs by name
	projectMap := make(map[string]LSP)
	for _, lsp := range project.LSPs {
		projectMap[lsp.Name] = lsp
	}

	// Start with global LSPs, replacing with merged project versions where they exist
	for _, globalLSP := range global.LSPs {
		if projectLSP, exists := projectMap[globalLSP.Name]; exists {
			// Use deep merged version
			merged.LSPs = append(merged.LSPs, mergeLSP(globalLSP, projectLSP))
			delete(projectMap, globalLSP.Name) // Mark as processed
		} else {
			merged.LSPs = append(merged.LSPs, globalLSP)
		}
	}

	// Add remaining project LSPs that weren't in global
	for _, lsp := range projectMap {
		merged.LSPs = append(merged.LSPs, lsp)
	}

	return merged
}

// mergeLSP merges project LSP over global LSP with deep merge for maps
func mergeLSP(global, project LSP) LSP {
	// Start with project (most fields are simple replacements)
	result := project

	// Deep merge for Env (project env vars override global)
	if len(global.Env) > 0 {
		if result.Env == nil {
			result.Env = make(map[string]string)
		}
		for k, v := range global.Env {
			if _, exists := result.Env[k]; !exists {
				result.Env[k] = v
			}
		}
	}

	// Deep merge for InitOptions
	if len(global.InitOptions) > 0 {
		result.InitOptions = deepMergeMap(global.InitOptions, project.InitOptions)
	}

	// Deep merge for Settings
	if len(global.Settings) > 0 {
		result.Settings = deepMergeMap(global.Settings, project.Settings)
	}

	return result
}

// deepMergeMap performs deep merge of maps, with project values taking precedence
func deepMergeMap(global, project map[string]any) map[string]any {
	if len(project) == 0 {
		return global
	}

	result := make(map[string]any)

	// Start with global
	for k, v := range global {
		result[k] = v
	}

	// Override with project (deep merge for nested maps)
	for k, v := range project {
		result[k] = deepMergeValue(result[k], v)
	}

	return result
}

// deepMergeValue recursively merges values, with new taking precedence
func deepMergeValue(existing, new any) any {
	// If new is a map and existing is a map, merge recursively
	existingMap, existingIsMap := existing.(map[string]any)
	newMap, newIsMap := new.(map[string]any)

	if existingIsMap && newIsMap {
		merged := make(map[string]any)
		for k, v := range existingMap {
			merged[k] = v
		}
		for k, v := range newMap {
			merged[k] = deepMergeValue(merged[k], v)
		}
		return merged
	}

	// Otherwise, new value completely replaces existing
	return new
}
