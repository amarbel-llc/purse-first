package mapping

type ToolSuggestion struct {
	Name    string `json:"name"`
	UseWhen string `json:"use_when"`
}

type Mapping struct {
	Replaces        string           `json:"replaces"`
	Extensions      []string         `json:"extensions,omitempty"`
	CommandPrefixes []string         `json:"command_prefixes,omitempty"`
	Tools           []ToolSuggestion `json:"tools"`
	Reason          string           `json:"reason"`
}

type MappingFile struct {
	Server     string    `json:"server"`
	ToolPrefix string    `json:"tool_prefix,omitempty"`
	Mappings   []Mapping `json:"mappings"`
}
