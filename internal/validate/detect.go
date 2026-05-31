package validate

import (
	"encoding/json"
	"path/filepath"
)

type DocType int

const (
	Unknown DocType = iota
	PluginDoc
	MappingDoc
	MCPDoc
)

func (d DocType) String() string {
	switch d {
	case PluginDoc:
		return "plugin"
	case MappingDoc:
		return "mapping"
	case MCPDoc:
		return "mcp"
	default:
		return "unknown"
	}
}

func DetectTypeFromFilename(name string) DocType {
	base := filepath.Base(name)
	switch base {
	case "plugin.json":
		return PluginDoc
	case "mappings.json":
		return MappingDoc
	default:
		return Unknown
	}
}

func DetectTypeFromContent(data []byte) DocType {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return Unknown
	}

	_, hasMappings := obj["mappings"]
	_, hasServer := obj["server"]
	if hasMappings && hasServer {
		return MappingDoc
	}

	if _, hasName := obj["name"]; hasName {
		return PluginDoc
	}

	return Unknown
}
