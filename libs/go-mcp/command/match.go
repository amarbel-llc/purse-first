package command

import (
	"path/filepath"
	"strings"
)

// FindToolMatch checks whether a tool invocation matches any of the given
// ToolMapping declarations. Returns a pointer to the first match, or nil.
func FindToolMatch(mappings []ToolMapping, toolName, filePath, command string) *ToolMapping {
	for i := range mappings {
		m := &mappings[i]

		if m.Replaces != toolName {
			continue
		}

		hasExtensions := len(m.Extensions) > 0
		hasPrefixes := len(m.CommandPrefixes) > 0

		// Catch-all: no extensions and no prefixes means match everything.
		if !hasExtensions && !hasPrefixes {
			return m
		}

		if hasExtensions && filePath != "" {
			ext := strings.ToLower(filepath.Ext(filePath))
			for _, e := range m.Extensions {
				if strings.ToLower(e) == ext {
					return m
				}
			}
		}

		if hasPrefixes && command != "" {
			for _, prefix := range m.CommandPrefixes {
				if strings.HasPrefix(command, prefix) {
					return m
				}
			}
		}
	}

	return nil
}
