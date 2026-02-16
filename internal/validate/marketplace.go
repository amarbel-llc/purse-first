package validate

import (
	"encoding/json"
	"fmt"
)

var knownSources = map[string]bool{
	"github": true,
	"url":    true,
	"npm":    true,
	"pip":    true,
}

type marketplaceDoc struct {
	Name    string `json:"name"`
	Owner   struct {
		Name string `json:"name"`
	} `json:"owner"`
	Plugins []struct {
		Name   string `json:"name"`
		Source json.RawMessage `json:"source"`
	} `json:"plugins"`
}

type sourceObj struct {
	Source string `json:"source"`
	Repo   string `json:"repo"`
}

func validateMarketplace(data []byte, strict bool) *Result {
	r := &Result{}

	var doc marketplaceDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		r.addError("", "invalid JSON: "+err.Error())
		return r
	}

	if doc.Name == "" {
		r.addError("name", "name is required")
	} else if !namePattern.MatchString(doc.Name) {
		r.addError("name", "must match ^[a-z][a-z0-9-]*$")
	}

	if doc.Owner.Name == "" {
		r.addError("owner.name", "owner name is required")
	}

	if len(doc.Plugins) == 0 {
		r.addError("plugins", "at least one plugin is required")
	}

	seen := make(map[string]bool)

	for i, p := range doc.Plugins {
		prefix := fmt.Sprintf("plugins[%d]", i)

		if p.Name == "" {
			r.addError(prefix+".name", "name is required")
		} else {
			if !namePattern.MatchString(p.Name) {
				r.addError(prefix+".name", "must match ^[a-z][a-z0-9-]*$")
			}
			if seen[p.Name] {
				r.addError(prefix+".name", fmt.Sprintf("duplicate plugin name %q", p.Name))
			}
			seen[p.Name] = true
		}

		if len(p.Source) == 0 {
			r.addError(prefix+".source", "source is required")
			continue
		}

		// Source can be a string path or an object
		var strSource string
		if err := json.Unmarshal(p.Source, &strSource); err == nil {
			continue
		}

		var src sourceObj
		if err := json.Unmarshal(p.Source, &src); err != nil {
			r.addError(prefix+".source", "must be a string or object with \"source\" field")
			continue
		}

		if !knownSources[src.Source] {
			r.addError(prefix+".source.source", fmt.Sprintf("must be one of: github, url, npm, pip; got %q", src.Source))
		}

		if src.Source == "github" && src.Repo == "" {
			r.addError(prefix+".source.repo", "repo is required for github source")
		}
	}

	return r
}
