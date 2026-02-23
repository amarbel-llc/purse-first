package packagetoml

import "github.com/BurntSushi/toml"

// Author represents the package author.
type Author struct {
	Name string `toml:"name"`
}

// MCPServer describes an MCP server declared in a package.toml.
type MCPServer struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
}

// Hook describes a Claude Code hook trigger.
type Hook struct {
	Matcher string `toml:"matcher"`
	Command string `toml:"command"`
	Timeout int    `toml:"timeout"`
}

// Package is the top-level structure parsed from a package.toml file.
type Package struct {
	Name        string               `toml:"name"`
	Description string               `toml:"description"`
	Version     string               `toml:"version"`
	Author      Author               `toml:"author"`
	MCP         map[string]MCPServer `toml:"mcp"`
	Hooks       map[string][]Hook    `toml:"hooks"`
}

// Parse decodes a package.toml from raw bytes.
func Parse(data []byte) (*Package, error) {
	var pkg Package
	if err := toml.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}

// ParseFile decodes a package.toml from a file path.
func ParseFile(path string) (*Package, error) {
	var pkg Package
	if _, err := toml.DecodeFile(path, &pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}
