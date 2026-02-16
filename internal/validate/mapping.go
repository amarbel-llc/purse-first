package validate

import (
	"encoding/json"
	"fmt"
	"strings"
)

var knownTools = map[string]bool{
	"Read":      true,
	"Edit":      true,
	"Write":     true,
	"Grep":      true,
	"Glob":      true,
	"Bash":      true,
	"WebFetch":  true,
	"WebSearch": true,
}

type mappingDoc struct {
	Server   string `json:"server"`
	Mappings []struct {
		Replaces        string `json:"replaces"`
		Extensions      []string `json:"extensions,omitempty"`
		CommandPrefixes []string `json:"command_prefixes,omitempty"`
		Tools           []struct {
			Name    string `json:"name"`
			UseWhen string `json:"use_when"`
		} `json:"tools"`
		Reason string `json:"reason"`
	} `json:"mappings"`
}

func validateMapping(data []byte, strict bool) *Result {
	r := &Result{}

	var doc mappingDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		r.addError("", "invalid JSON: "+err.Error())
		return r
	}

	if doc.Server == "" {
		r.addError("server", "server is required")
	} else if !namePattern.MatchString(doc.Server) {
		r.addError("server", "must match ^[a-z][a-z0-9-]*$")
	}

	if len(doc.Mappings) == 0 {
		r.addError("mappings", "at least one mapping is required")
	}

	for i, m := range doc.Mappings {
		prefix := fmt.Sprintf("mappings[%d]", i)

		if m.Replaces == "" {
			r.addError(prefix+".replaces", "replaces is required")
		} else if !knownTools[m.Replaces] {
			r.addWarning(prefix+".replaces", fmt.Sprintf("%q is not a known built-in tool", m.Replaces))
		}

		if len(m.Tools) == 0 {
			r.addError(prefix+".tools", "at least one tool is required")
		}

		for j, t := range m.Tools {
			tp := fmt.Sprintf("%s.tools[%d]", prefix, j)
			if t.Name == "" {
				r.addError(tp+".name", "name is required")
			}
			if t.UseWhen == "" {
				r.addError(tp+".use_when", "use_when is required")
			}
		}

		if m.Reason == "" {
			r.addError(prefix+".reason", "reason is required")
		}

		for _, ext := range m.Extensions {
			if !strings.HasPrefix(ext, ".") {
				r.addWarning(prefix+".extensions", fmt.Sprintf("extension %q should start with \".\"", ext))
			}
		}

		if len(m.Extensions) == 0 && len(m.CommandPrefixes) == 0 {
			if strict {
				r.addError(prefix, "must have extensions or command_prefixes for scoping")
			} else {
				r.addWarning(prefix, "no extensions or command_prefixes; mapping applies to all invocations")
			}
		}
	}

	return r
}
