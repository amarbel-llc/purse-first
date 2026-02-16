package validate

import (
	"encoding/json"
	"regexp"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var knownPluginFields = map[string]bool{
	"$schema":      true,
	"name":         true,
	"description":  true,
	"version":      true,
	"author":       true,
	"homepage":     true,
	"repository":   true,
	"license":      true,
	"keywords":     true,
	"mcpServers":   true,
	"lspServers":   true,
	"skills":       true,
	"hooks":        true,
	"commands":     true,
	"agents":       true,
	"outputStyles": true,
}

type pluginDoc struct {
	Name       string         `json:"name"`
	McpServers map[string]any `json:"mcpServers"`
	Author     *struct {
		Name string `json:"name"`
	} `json:"author"`
}

func validatePlugin(data []byte, strict bool) *Result {
	r := &Result{}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		r.addError("", "invalid JSON: "+err.Error())
		return r
	}

	for key := range raw {
		if !knownPluginFields[key] {
			if strict {
				r.addError(key, "unknown field")
			} else {
				r.addWarning(key, "unknown field")
			}
		}
	}

	var doc pluginDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		r.addError("", "invalid JSON: "+err.Error())
		return r
	}

	if doc.Name == "" {
		r.addError("name", "name is required")
	} else if !namePattern.MatchString(doc.Name) {
		r.addError("name", "must match ^[a-z][a-z0-9-]*$")
	}

	if doc.Author != nil && doc.Author.Name == "" {
		r.addError("author.name", "author name is required when author is present")
	}

	if len(doc.McpServers) == 0 {
		r.addWarning("mcpServers", "no MCP servers declared")
	} else {
		for name, srv := range doc.McpServers {
			srvMap, ok := srv.(map[string]any)
			if !ok {
				r.addError("mcpServers."+name, "expected object")
				continue
			}

			t, _ := srvMap["type"].(string)
			if t != "stdio" {
				r.addError("mcpServers."+name+".type", "must be \"stdio\"")
			}

			cmd, _ := srvMap["command"].(string)
			if cmd == "" {
				r.addError("mcpServers."+name+".command", "command is required")
			} else if strings.Contains(cmd, " ") {
				r.addWarning("mcpServers."+name+".command", "command contains spaces; arguments should go in args")
			}
		}
	}

	return r
}
