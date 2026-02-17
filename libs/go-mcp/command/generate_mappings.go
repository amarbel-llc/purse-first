package command

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type mappingToolSuggestion struct {
	Name    string `json:"name"`
	UseWhen string `json:"use_when"`
}

type mappingEntry struct {
	Replaces        string                  `json:"replaces"`
	CommandPrefixes []string                `json:"command_prefixes,omitempty"`
	Tools           []mappingToolSuggestion `json:"tools"`
	Reason          string                  `json:"reason"`
}

type mappingFile struct {
	Server   string         `json:"server"`
	Mappings []mappingEntry `json:"mappings"`
}

// GenerateMappings writes a mappings.json file to {dir}/{app.Name}/mappings.json.
// Only commands with MapsBash declarations are included. Each BashMapping on a
// command produces a separate mapping entry. If no commands have bash mappings,
// no file is written.
func (a *App) GenerateMappings(dir string) error {
	var entries []mappingEntry

	for _, cmd := range a.AllCommands() {
		if cmd.Hidden {
			continue
		}
		for _, bm := range cmd.MapsBash {
			entries = append(entries, mappingEntry{
				Replaces:        "Bash",
				CommandPrefixes: bm.Prefixes,
				Tools: []mappingToolSuggestion{
					{Name: cmd.Name, UseWhen: bm.UseWhen},
				},
				Reason: "Use the " + a.Name + " MCP tool instead of shelling out",
			})
		}
	}

	if len(entries) == 0 {
		return nil
	}

	mf := mappingFile{
		Server:   a.Name,
		Mappings: entries,
	}

	pluginDir := filepath.Join(dir, a.Name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(filepath.Join(pluginDir, "mappings.json"), data, 0o644)
}
